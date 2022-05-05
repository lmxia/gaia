// This file was copied from k8s.io/kubernetes/pkg/scheduler/framework/runtime/framework.go and modified

package runtime

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	gaiaClientSet "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	informers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	schedulerapis "github.com/lmxia/gaia/pkg/scheduler/apis"
	framework "github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
	"github.com/lmxia/gaia/pkg/scheduler/metrics"
	"github.com/lmxia/gaia/pkg/scheduler/parallelize"
	"k8s.io/apimachinery/pkg/util/sets"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
)

const (
	// Filter is the name of the filter extension point.
	Filter = "Filter"
	// Specifies the maximum timeout a permit plugin can return.
	maxTimeout              = 15 * time.Minute
	preFilter               = "PreFilter"
	postFilter              = "PostFilter"
	preScore                = "PreScore"
	score                   = "Score"
	scoreExtensionNormalize = "ScoreExtensionNormalize"
)

// frameworkImpl is the component responsible for initializing and running scheduler
// plugins.
type frameworkImpl struct {
	scorePluginWeight map[string]int
	preFilterPlugins  []framework.PreFilterPlugin
	filterPlugins     []framework.FilterPlugin
	postFilterPlugins []framework.PostFilterPlugin
	preScorePlugins   []framework.PreScorePlugin
	scorePlugins      []framework.ScorePlugin

	clientSet       gaiaClientSet.Interface
	kubeConfig      *restclient.Config
	eventRecorder   record.EventRecorder
	informerFactory informers.SharedInformerFactory

	metricsRecorder *metricsRecorder
	profileName     string

	parallelizer parallelize.Parallelizer

	// Indicates that RunFilterPlugins should accumulate all failed statuses and not return
	// after the first failure.
	runAllFilters bool
}

// extensionPoint encapsulates desired and applied set of plugins at a specific extension
// point. This is used to simplify iterating over all extension points supported by the
// frameworkImpl.
type extensionPoint struct {
	// the set of plugins to be configured at this extension point.
	plugins *schedulerapis.PluginSet
	// a pointer to the slice storing plugins implementations that will run at this
	// extension point.
	slicePtr interface{}
}

func (f *frameworkImpl) getExtensionPoints(plugins *schedulerapis.Plugins) []extensionPoint {
	return []extensionPoint{
		{&plugins.PreFilter, &f.preFilterPlugins},
		{&plugins.Filter, &f.filterPlugins},
		{&plugins.PostFilter, &f.postFilterPlugins},
		{&plugins.PreScore, &f.preScorePlugins},
		{&plugins.Score, &f.scorePlugins},
	}
}

type frameworkOptions struct {
	clientSet       gaiaClientSet.Interface
	kubeConfig      *restclient.Config
	eventRecorder   record.EventRecorder
	informerFactory informers.SharedInformerFactory
	runAllFilters   bool
	parallelizer    parallelize.Parallelizer
	metricsRecorder *metricsRecorder
}

// Option for the frameworkImpl.
type Option func(*frameworkOptions)

// WithClientSet sets clientSet for the scheduling frameworkImpl.
func WithClientSet(clientSet gaiaClientSet.Interface) Option {
	return func(o *frameworkOptions) {
		o.clientSet = clientSet
	}
}

// WithKubeConfig sets kubeConfig for the scheduling frameworkImpl.
func WithKubeConfig(kubeConfig *restclient.Config) Option {
	return func(o *frameworkOptions) {
		o.kubeConfig = kubeConfig
	}
}

// WithEventRecorder sets clientSet for the scheduling frameworkImpl.
func WithEventRecorder(recorder record.EventRecorder) Option {
	return func(o *frameworkOptions) {
		o.eventRecorder = recorder
	}
}

// WithInformerFactory sets informer factory for the scheduling frameworkImpl.
func WithInformerFactory(informerFactory informers.SharedInformerFactory) Option {
	return func(o *frameworkOptions) {
		o.informerFactory = informerFactory
	}
}

// WithRunAllFilters sets the runAllFilters flag, which means RunFilterPlugins accumulates
// all failure Statuses.
func WithRunAllFilters(runAllFilters bool) Option {
	return func(o *frameworkOptions) {
		o.runAllFilters = runAllFilters
	}
}

// WithParallelism sets parallelism for the scheduling frameworkImpl.
func WithParallelism(parallelism int) Option {
	return func(o *frameworkOptions) {
		o.parallelizer = parallelize.NewParallelizer(parallelism)
	}
}

func defaultFrameworkOptions() frameworkOptions {
	return frameworkOptions{
		metricsRecorder: newMetricsRecorder(1000, time.Second),
		parallelizer:    parallelize.NewParallelizer(parallelize.DefaultParallelism),
	}
}

var _ framework.Framework = &frameworkImpl{}

// NewFramework initializes plugins given the configuration and the registry.
func NewFramework(r Registry, plugins *schedulerapis.Plugins, opts ...Option) (framework.Framework, error) {
	options := defaultFrameworkOptions()
	for _, opt := range opts {
		opt(&options)
	}

	f := &frameworkImpl{
		scorePluginWeight: make(map[string]int),
		clientSet:         options.clientSet,
		kubeConfig:        options.kubeConfig,
		eventRecorder:     options.eventRecorder,
		informerFactory:   options.informerFactory,
		metricsRecorder:   options.metricsRecorder,
		runAllFilters:     options.runAllFilters,
		parallelizer:      options.parallelizer,
		profileName:       "default",
	}

	if r == nil {
		return f, nil
	}

	// initialize plugins per individual extension points
	pluginsMap := make(map[string]framework.Plugin)
	for name, factory := range r {
		p, err := factory(nil, f)
		if err != nil {
			return nil, fmt.Errorf("initializing plugin %q: %w", name, err)
		}
		pluginsMap[name] = p
	}

	for _, e := range f.getExtensionPoints(plugins) {
		if err := updatePluginList(e.slicePtr, *e.plugins, pluginsMap); err != nil {
			return nil, err
		}
	}

	for _, scorePlugin := range plugins.Score.Enabled {
		// a weight of zero is not permitted, plugins can be disabled explicitly
		// when configured.
		if scorePlugin.Weight == 0 {
			return nil, fmt.Errorf("score plugin %q is not configured with weight", scorePlugin.Name)
		}
		f.scorePluginWeight[scorePlugin.Name] = int(scorePlugin.Weight)
	}

	return f, nil
}

// copied from k8s.io/kubernetes/pkg/scheduler/framework/runtime/framework.go
func updatePluginList(pluginList interface{}, pluginSet schedulerapis.PluginSet, pluginsMap map[string]framework.Plugin) error {
	plugins := reflect.ValueOf(pluginList).Elem()
	pluginType := plugins.Type().Elem()
	set := sets.NewString()
	for _, ep := range pluginSet.Enabled {
		pg, ok := pluginsMap[ep.Name]
		if !ok {
			return fmt.Errorf("%s %q does not exist", pluginType.Name(), ep.Name)
		}

		if !reflect.TypeOf(pg).Implements(pluginType) {
			return fmt.Errorf("plugin %q does not extend %s plugin", ep.Name, pluginType.Name())
		}

		if set.Has(ep.Name) {
			return fmt.Errorf("plugin %q already registered as %q", ep.Name, pluginType.Name())
		}

		set.Insert(ep.Name)

		newPlugins := reflect.Append(plugins, reflect.ValueOf(pg))
		plugins.Set(newPlugins)
	}
	return nil
}

// RunPreFilterPlugins runs the set of configured PreFilter plugins. It returns
// *Status and its code is set to non-success if any of the plugins returns
// anything but Success. If a non-success status is returned, then the scheduling
// cycle is aborted.
func (f *frameworkImpl) RunPreFilterPlugins(ctx context.Context, sub *v1alpha1.Component) (status *framework.Status) {
	startTime := time.Now()
	defer func() {
		metrics.FrameworkExtensionPointDuration.WithLabelValues(preFilter, status.Code().String(), f.profileName).Observe(metrics.SinceInSeconds(startTime))
	}()
	for _, pl := range f.preFilterPlugins {
		status = f.runPreFilterPlugin(ctx, pl, sub)
		if !status.IsSuccess() {
			status.SetFailedPlugin(pl.Name())
			if status.IsUnschedulable() {
				return status
			}
			return framework.AsStatus(fmt.Errorf("running PreFilter plugin %q: %w", pl.Name(), status.AsError())).WithFailedPlugin(pl.Name())
		}
	}

	return nil
}

func (f *frameworkImpl) runPreFilterPlugin(ctx context.Context, pl framework.PreFilterPlugin, sub *v1alpha1.Component) *framework.Status {
	startTime := time.Now()
	status := pl.PreFilter(ctx, sub)
	f.metricsRecorder.observePluginDurationAsync(preFilter, pl.Name(), status, metrics.SinceInSeconds(startTime))
	return status
}

// RunFilterPlugins runs the set of configured Filter plugins for subscription on
// the given cluster. If any of these plugins doesn't return "Success", the
// given cluster is not suitable for running subscription.
// Meanwhile, the failure message and status are set for the given cluster.
func (f *frameworkImpl) RunFilterPlugins(ctx context.Context, com *v1alpha1.Component, cluster *clusterapi.ManagedCluster) framework.PluginToStatus {
	statuses := make(framework.PluginToStatus)
	for _, pl := range f.filterPlugins {
		pluginStatus := f.runFilterPlugin(ctx, pl, com, cluster)
		if !pluginStatus.IsSuccess() {
			if !pluginStatus.IsUnschedulable() {
				// Filter plugins are not supposed to return any status other than
				// Success or Unschedulable.
				errStatus := framework.AsStatus(fmt.Errorf("running %q filter plugin: %w", pl.Name(), pluginStatus.AsError())).WithFailedPlugin(pl.Name())
				return map[string]*framework.Status{pl.Name(): errStatus}
			}
			pluginStatus.SetFailedPlugin(pl.Name())
			statuses[pl.Name()] = pluginStatus
			if !f.runAllFilters {
				// Exit early if we don't need to run all filters.
				return statuses
			}
		}
	}

	return statuses
}

func (f *frameworkImpl) runFilterPlugin(ctx context.Context, pl framework.FilterPlugin, com *v1alpha1.Component, cluster *clusterapi.ManagedCluster) *framework.Status {
	startTime := time.Now()
	status := pl.Filter(ctx, com, cluster)
	f.metricsRecorder.observePluginDurationAsync(Filter, pl.Name(), status, metrics.SinceInSeconds(startTime))
	return status
}

// RunPostFilterPlugins runs the set of configured PostFilter plugins until the first
// Success or Error is met, otherwise continues to execute all plugins.
func (f *frameworkImpl) RunPostFilterPlugins(ctx context.Context, sub *v1alpha1.Component, filteredClusterStatusMap framework.ClusterToStatusMap) (_ *framework.PostFilterResult, status *framework.Status) {
	startTime := time.Now()
	defer func() {
		metrics.FrameworkExtensionPointDuration.WithLabelValues(postFilter, status.Code().String(), f.profileName).Observe(metrics.SinceInSeconds(startTime))
	}()

	statuses := make(framework.PluginToStatus)
	for _, pl := range f.postFilterPlugins {
		r, s := f.runPostFilterPlugin(ctx, pl, sub, filteredClusterStatusMap)
		if s.IsSuccess() {
			return r, s
		} else if !s.IsUnschedulable() {
			// Any status other than Success or Unschedulable is Error.
			return nil, framework.AsStatus(s.AsError())
		}
		statuses[pl.Name()] = s
	}

	return nil, statuses.Merge()
}

func (f *frameworkImpl) runPostFilterPlugin(ctx context.Context, pl framework.PostFilterPlugin, sub *v1alpha1.Component, filteredClusterStatusMap framework.ClusterToStatusMap) (*framework.PostFilterResult, *framework.Status) {
	startTime := time.Now()
	r, s := pl.PostFilter(ctx, sub, filteredClusterStatusMap)
	f.metricsRecorder.observePluginDurationAsync(postFilter, pl.Name(), s, metrics.SinceInSeconds(startTime))
	return r, s
}

// RunPreScorePlugins runs the set of configured pre-score plugins. If any
// of these plugins returns any status other than "Success", the given subscription is rejected.
func (f *frameworkImpl) RunPreScorePlugins(ctx context.Context, sub *v1alpha1.Component, clusters []*clusterapi.ManagedCluster) (status *framework.Status) {
	startTime := time.Now()
	defer func() {
		metrics.FrameworkExtensionPointDuration.WithLabelValues(preScore, status.Code().String(), f.profileName).Observe(metrics.SinceInSeconds(startTime))
	}()
	for _, pl := range f.preScorePlugins {
		status = f.runPreScorePlugin(ctx, pl, sub, clusters)
		if !status.IsSuccess() {
			return framework.AsStatus(fmt.Errorf("running PreScore plugin %q: %w", pl.Name(), status.AsError()))
		}
	}

	return nil
}

func (f *frameworkImpl) runPreScorePlugin(ctx context.Context, pl framework.PreScorePlugin, sub *v1alpha1.Component, clusters []*clusterapi.ManagedCluster) *framework.Status {
	startTime := time.Now()
	status := pl.PreScore(ctx, sub, clusters)
	f.metricsRecorder.observePluginDurationAsync(preScore, pl.Name(), status, metrics.SinceInSeconds(startTime))
	return status
}

// RunScorePlugins runs the set of configured scoring plugins. It returns a list that
// stores for each scoring plugin name the corresponding  ResourceBindingScoreList(s).
// It also returns *Status, which is set to non-success if any of the plugins returns
// a non-success status.
func (f *frameworkImpl) RunScorePlugins(ctx context.Context, rbs []*v1alpha1.ResourceBinding, clusters []*clusterapi.ManagedCluster) (ps framework.PluginToRBScores, status *framework.Status) {
	startTime := time.Now()
	defer func() {
		metrics.FrameworkExtensionPointDuration.WithLabelValues(score, status.Code().String(), f.profileName).Observe(metrics.SinceInSeconds(startTime))
	}()
	pluginToRBScores := make(framework.PluginToRBScores, len(f.scorePlugins))
	for _, pl := range f.scorePlugins {
		pluginToRBScores[pl.Name()] = make(framework.ResourceBindingScoreList, len(rbs))
	}
	ctx, cancel := context.WithCancel(ctx)
	errCh := parallelize.NewErrorChannel()

	// Run Score method for each cluster in parallel.
	f.Parallelizer().Until(ctx, len(rbs), func(index int) {
		for _, pl := range f.scorePlugins {
			s, status := f.runScorePlugin(ctx, pl, rbs[index], clusters)
			if !status.IsSuccess() {
				err := fmt.Errorf("plugin %q failed with: %w", pl.Name(), status.AsError())
				errCh.SendErrorWithCancel(err, cancel)
				return
			}
			pluginToRBScores[pl.Name()][index] = framework.ResourceBindingScore{
				Index: index,
				Score: s,
			}
		}
	})
	if err := errCh.ReceiveError(); err != nil {
		return nil, framework.AsStatus(fmt.Errorf("running Score plugins: %w", err))
	}

	// Run NormalizeScore method for each ScorePlugin in parallel.
	f.Parallelizer().Until(ctx, len(f.scorePlugins), func(index int) {
		pl := f.scorePlugins[index]
		ClusterScoreList := pluginToRBScores[pl.Name()]
		if pl.ScoreExtensions() == nil {
			return
		}
		status := f.runScoreExtension(ctx, pl, ClusterScoreList)
		if !status.IsSuccess() {
			err := fmt.Errorf("plugin %q failed with: %w", pl.Name(), status.AsError())
			errCh.SendErrorWithCancel(err, cancel)
			return
		}
	})
	if err := errCh.ReceiveError(); err != nil {
		return nil, framework.AsStatus(fmt.Errorf("running Normalize on Score plugins: %w", err))
	}

	// Apply score defaultWeights for each ScorePlugin in parallel.
	f.Parallelizer().Until(ctx, len(f.scorePlugins), func(index int) {
		pl := f.scorePlugins[index]
		// Score plugins' weight has been checked when they are initialized.
		weight := f.scorePluginWeight[pl.Name()]
		rbScoreList := pluginToRBScores[pl.Name()]

		for i, rbScore := range rbScoreList {
			// return error if score plugin returns invalid score.
			if rbScore.Score > framework.MaxClusterScore || rbScore.Score < framework.MinClusterScore {
				err := fmt.Errorf("plugin %q returns an invalid score %v, it should in the range of [%v, %v] after normalizing", pl.Name(), rbScore.Score, framework.MinClusterScore, framework.MaxClusterScore)
				errCh.SendErrorWithCancel(err, cancel)
				return
			}
			rbScoreList[i].Score = rbScore.Score * int64(weight)
		}
	})
	if err := errCh.ReceiveError(); err != nil {
		return nil, framework.AsStatus(fmt.Errorf("applying score defaultWeights on Score plugins: %w", err))
	}

	return pluginToRBScores, nil
}

func (f *frameworkImpl) runScorePlugin(ctx context.Context, pl framework.ScorePlugin, rb *v1alpha1.ResourceBinding, clusters []*clusterapi.ManagedCluster) (int64, *framework.Status) {
	startTime := time.Now()
	s, status := pl.Score(ctx, rb, clusters)
	f.metricsRecorder.observePluginDurationAsync(score, pl.Name(), status, metrics.SinceInSeconds(startTime))
	return s, status
}

func (f *frameworkImpl) runScoreExtension(ctx context.Context, pl framework.ScorePlugin, RBScoreList framework.ResourceBindingScoreList) *framework.Status {
	startTime := time.Now()
	status := pl.ScoreExtensions().NormalizeScore(ctx, RBScoreList)
	f.metricsRecorder.observePluginDurationAsync(scoreExtensionNormalize, pl.Name(), status, metrics.SinceInSeconds(startTime))
	return status
}

// HasFilterPlugins returns true if at least one filter plugin is defined.
func (f *frameworkImpl) HasFilterPlugins() bool {
	return len(f.filterPlugins) > 0
}

// HasPostFilterPlugins returns true if at least one postFilter plugin is defined.
func (f *frameworkImpl) HasPostFilterPlugins() bool {
	return len(f.postFilterPlugins) > 0
}

// HasScorePlugins returns true if at least one score plugin is defined.
func (f *frameworkImpl) HasScorePlugins() bool {
	return len(f.scorePlugins) > 0
}

// ListPlugins returns a map of extension point name to plugin names configured at each extension
// point. Returns nil if no plugins where configured.
func (f *frameworkImpl) ListPlugins() *schedulerapis.Plugins {
	m := schedulerapis.Plugins{}

	for _, e := range f.getExtensionPoints(&m) {
		plugins := reflect.ValueOf(e.slicePtr).Elem()
		extName := plugins.Type().Elem().Name()
		var cfgs []schedulerapis.Plugin
		for i := 0; i < plugins.Len(); i++ {
			name := plugins.Index(i).Interface().(framework.Plugin).Name()
			p := schedulerapis.Plugin{Name: name}
			if extName == "ScorePlugin" {
				// Weights apply only to score plugins.
				p.Weight = int32(f.scorePluginWeight[name])
			}
			cfgs = append(cfgs, p)
		}
		if len(cfgs) > 0 {
			e.plugins.Enabled = cfgs
		}
	}
	return &m
}

// ClientSet returns a clientset.
func (f *frameworkImpl) ClientSet() gaiaClientSet.Interface {
	return f.clientSet
}

// KubeConfig returns a kubeconfig.
func (f *frameworkImpl) KubeConfig() *restclient.Config {
	return f.kubeConfig
}

// EventRecorder returns an event recorder.
func (f *frameworkImpl) EventRecorder() record.EventRecorder {
	return f.eventRecorder
}

// SharedInformerFactory returns a shared informer factory.
func (f *frameworkImpl) SharedInformerFactory() informers.SharedInformerFactory {
	return f.informerFactory
}

// ProfileName returns the profile name associated to this framework.
func (f *frameworkImpl) ProfileName() string {
	return f.profileName
}

// Parallelizer returns a parallelizer holding parallelism for scheduler.
func (f *frameworkImpl) Parallelizer() parallelize.Parallelizer {
	return f.parallelizer
}
