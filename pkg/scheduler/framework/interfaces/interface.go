package interfaces

import (
	"context"
	"math"

	coreV1 "k8s.io/api/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"

	appsapi "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	gaiaClientSet "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	informers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	schedulerapis "github.com/lmxia/gaia/pkg/scheduler/apis"
	"github.com/lmxia/gaia/pkg/scheduler/parallelize"
)

// ResourceBindingScoreList declares a list of clusters and their scores.
type ResourceBindingScoreList []ResourceBindingScore

// ResourceBindingScore is a struct with cluster id and score.
type ResourceBindingScore struct {
	Index int // the index of rbs.
	Score int64
}

func (rbScores ResourceBindingScoreList) Len() int {
	return len(rbScores)
}

func (rbScores ResourceBindingScoreList) Less(i, j int) bool {
	return rbScores[i].Score < rbScores[j].Score
}

func (rbScores ResourceBindingScoreList) Swap(i, j int) {
	rbScores[i], rbScores[j] = rbScores[j], rbScores[i]
}

// PluginToRBScores declares a map from plugin name to its ResourceBindingScoreList.
type PluginToRBScores map[string]ResourceBindingScoreList

// ClusterToStatusMap declares map from cluster namespaced key to its status.
type ClusterToStatusMap map[string]*Status

// statusPrecedence defines a map from status to its precedence, larger value means higher precedent.
var statusPrecedence = map[Code]int{
	Error:                        ErrorPrecedence,
	UnschedulableAndUnresolvable: UnschedulableUnresolvablePrecedence,
	Unschedulable:                1,
	// Any other statuses we know today, `Skip` or `Wait`, will take precedence over `Success`.
	Success: -1,
}

const (
	// MaxClusterScore is the maximum score a Score plugin is expected to return.
	MaxClusterScore int64 = 100

	// MinClusterScore is the minimum score a Score plugin is expected to return.
	MinClusterScore int64 = 0

	// MaxTotalScore is the maximum total score.
	MaxTotalScore int64 = math.MaxInt64

	ErrorPrecedence                     = 3
	UnschedulableUnresolvablePrecedence = 2
)

// PluginToStatus maps plugin name to status. Currently, used to identify which Filter plugin
// returned which status.
type PluginToStatus map[string]*Status

// Merge merges the statuses in the map into one. The resulting status code have the following
// precedence: Error, UnschedulableAndUnresolvable, Unschedulable.
func (p PluginToStatus) Merge() *Status {
	if len(p) == 0 {
		return nil
	}

	finalStatus := NewStatus(Success)
	for _, s := range p {
		if s.Code() == Error {
			finalStatus.err = s.AsError()
		}
		if statusPrecedence[s.Code()] > statusPrecedence[finalStatus.code] {
			finalStatus.code = s.Code()
			// Same as code, we keep the most relevant failedPlugin in the returned Status.
			finalStatus.failedPlugin = s.FailedPlugin()
		}

		for _, r := range s.reasons {
			finalStatus.AppendReason(r)
		}
	}

	return finalStatus
}

// WaitingSubscription represents a subscription currently waiting in the permit phase.
type WaitingSubscription interface {
	// GetSubscription returns a reference to the waiting subscription.
	GetDescription() *appsapi.Description

	// GetPendingPlugins returns a list of pending Permit plugin's name.
	GetPendingPlugins() []string

	// Allow declares the waiting subscription is allowed to be scheduled by the plugin named as "pluginName".
	// If this is the last remaining plugin to allow, then a success signal is delivered
	// to unblock the subscription.
	Allow(pluginName string)

	// Reject declares the waiting subscription Unschedulable.
	Reject(pluginName, msg string)
}

// Plugin is the parent type for all the scheduling framework plugins.
type Plugin interface {
	Name() string
}

// PreFilterPlugin is an interface that must be implemented by "PreFilter" plugins.
// These plugins are called at the beginning of the scheduling cycle.
type PreFilterPlugin interface {
	Plugin

	// PreFilter is called at the beginning of the scheduling cycle. All PreFilter
	// plugins must return success or the subscription will be rejected.
	PreFilter(ctx context.Context, sub *appsapi.Component) *Status
}

// FilterPlugin is an interface for Filter plugins. These plugins are called at the
// filter extension point for filtering out hosts that cannot run a subscription.
// This concept used to be called 'predicate' in the original scheduler.
// These plugins should return Success, Unschedulable or Error in Status.code.
// However, the scheduler accepts other valid codes as well.
// Anything other than "Success" will lead to exclusion of the given host from
// running the subscription.
type FilterPlugin interface {
	Plugin

	// Filter is called by the scheduling framework.
	// All FilterPlugins should return "Success" to declare that
	// the given managed cluster fits the subscription. If Filter doesn't return "Success",
	// it will return Unschedulable, UnschedulableAndUnresolvable or Error.
	// For the cluster being evaluated, Filter plugins should look at the passed
	// cluster's information (e.g., subscriptions considered to be running on the cluster).
	Filter(ctx context.Context, com *appsapi.Component, cluster *clusterapi.ManagedCluster) *Status
	FilterVN(ctx context.Context, com *appsapi.Component, node *coreV1.Node) *Status
}

// PostFilterPlugin is an interface for "PostFilter" plugins. These plugins are called
// after a subscription cannot be scheduled.
type PostFilterPlugin interface {
	Plugin

	// PostFilter is called by the scheduling framework.
	// A PostFilter plugin should return one of the following statuses:
	// - Unschedulable: the plugin gets executed successfully but the subscription cannot be made schedulable.
	// - Success: the plugin gets executed successfully and the subscription can be made schedulable.
	// - Error: the plugin aborts due to some internal error.
	//
	// Informational plugins should be configured ahead of other ones, and always return Unschedulable status.
	// Optionally, a non-nil PostFilterResult may be returned along with a Success status. For example,
	// a preemption plugin may choose to return nominatedClusterName, so that framework can reuse that to update the
	// preemptor subscription's .spec.status.nominatedClusterName field.
	PostFilter(ctx context.Context, sub *appsapi.Component,
		filteredClusterStatusMap ClusterToStatusMap) (*PostFilterResult, *Status)
}

// PreScorePlugin is an interface for "PreScore" plugin. PreScore is an
// informational extension point. Plugins will be called with a list of clusters
// that passed the filtering phase. A plugin may use this data to update internal
// state or to generate logs/metrics.
type PreScorePlugin interface {
	Plugin

	// PreScore is called by the scheduling framework after a list of managed clusters
	// passed the filtering phase. All prescore plugins must return success or
	// the subscription will be rejected
	PreScore(ctx context.Context, sub *appsapi.Component, clusters []*clusterapi.ManagedCluster) *Status
	PreScoreVN(ctx context.Context, sub *appsapi.Component, nodes []*coreV1.Node) *Status
}

// ScoreExtensions is an interface for Score extended functionality.
type ScoreExtensions interface {
	// NormalizeScore is called for all cluster scores produced by the same plugin's "Score"
	// method. A successful run of NormalizeScore will update the scores list and return
	// a success status.
	NormalizeScore(ctx context.Context, scores ResourceBindingScoreList) *Status
}

// ScorePlugin is an interface that must be implemented by "Score" plugins to rank
// clusters that passed the filtering phase.
type ScorePlugin interface {
	Plugin

	// Score is called on each filtered cluster. It must return success and an integer
	// indicating the rank of the cluster. All scoring plugins must return success or
	// the subscription will be rejected.
	Score(ctx context.Context, description *appsapi.Description, sub *appsapi.ResourceBinding,
		clusters []*clusterapi.ManagedCluster) (int64, *Status)
	ScoreVN(ctx context.Context, description *appsapi.Description, sub *appsapi.ResourceBinding,
		nodes []*coreV1.Node) (int64, *Status)
	// ScoreExtensions returns a ScoreExtensions interface if it implements one, or nil if not.
	ScoreExtensions() ScoreExtensions
}

// Framework manages the set of plugins in use by the scheduling framework.
// Configured plugins are called at specified points in a scheduling context.
type Framework interface {
	Handle

	// RunPreFilterPlugins runs the set of configured PreFilter plugins. It returns
	// *Status and its code is set to non-success if any of the plugins returns
	// anything but Success. If a non-success status is returned, then the scheduling
	// cycle is aborted.
	RunPreFilterPlugins(ctx context.Context, sub *appsapi.Component) *Status

	// RunPostFilterPlugins runs the set of configured PostFilter plugins.
	// PostFilter plugins can either be informational, in which case should be configured
	// to execute first and return Unschedulable status, or ones that try to change the
	// cluster state to make the subscription potentially schedulable in a future scheduling cycle.
	RunPostFilterPlugins(ctx context.Context, sub *appsapi.Component,
		filteredClusterStatusMap ClusterToStatusMap) (*PostFilterResult, *Status)

	// HasFilterPlugins returns true if at least one Filter plugin is defined.
	HasFilterPlugins() bool

	// HasPostFilterPlugins returns true if at least one PostFilter plugin is defined.
	HasPostFilterPlugins() bool

	// HasScorePlugins returns true if at least one Score plugin is defined.
	HasScorePlugins() bool

	// ListPlugins returns a map of extension point name to list of configured Plugins.
	ListPlugins() *schedulerapis.Plugins

	// ProfileName returns the profile name associated to this framework.
	ProfileName() string
}

// Handle provides data and some tools that plugins can use. It is
// passed to the plugin factories at the time of plugin initialization. Plugins
// must store and use this handle to call framework functions.
type Handle interface {
	// PluginsRunner abstracts operations to run some plugins.
	PluginsRunner

	// ClientSet returns a clientSet.
	ClientSet() gaiaClientSet.Interface

	// KubeConfig returns the raw kubeconfig.
	KubeConfig() *restclient.Config

	// EventRecorder returns an event recorder.
	EventRecorder() record.EventRecorder

	SharedInformerFactory() informers.SharedInformerFactory

	// Parallelizer returns a parallelizer holding parallelism for scheduler.
	Parallelizer() parallelize.Parallelizer
}

// PostFilterResult wraps needed info for scheduler framework to act upon PostFilter phase.
type PostFilterResult struct {
	NominatedNamespacedClusters []string
}

// PluginsRunner abstracts operations to run some plugins.
// This is used by preemption PostFilter plugins when evaluating the feasibility of
// scheduling the subscription on clusters when certain running subscriptions get evicted.
type PluginsRunner interface {
	// RunPreScorePlugins runs the set of configured PreScore plugins. If any
	// of these plugins returns any status other than "Success", the given subscription is rejected.
	RunPreScorePlugins(context.Context, *appsapi.Component, []*clusterapi.ManagedCluster) *Status

	// RunScorePlugins runs the set of configured Score plugins. It returns a map that
	// stores for each Score plugin name the corresponding ResourceBindingScoreList(s).
	// It also returns *Status, which is set to non-success if any of the plugins returns
	// a non-success status.
	RunScorePlugins(context.Context, *appsapi.Description, []*appsapi.ResourceBinding,
		[]*clusterapi.ManagedCluster) (PluginToRBScores, *Status)

	// RunFilterPlugins runs the set of configured Filter plugins for subscription on
	// the given cluster.
	RunFilterPlugins(context.Context, *appsapi.Component, *clusterapi.ManagedCluster) PluginToStatus

	// RunPreScorePluginsVN runs the set of configured PreScore plugins. If any
	// of these plugins returns any status other than "Success", the given subscription is rejected.
	RunPreScorePluginsVN(context.Context, *appsapi.Component, []*coreV1.Node) *Status

	// RunScorePluginsVN runs the set of configured Score plugins. It returns a map that
	// stores for each Score plugin name the corresponding ResourceBindingScoreList(s).
	// It also returns *Status, which is set to non-success if any of the plugins returns
	// a non-success status.
	RunScorePluginsVN(context.Context, *appsapi.Description, []*appsapi.ResourceBinding,
		[]*coreV1.Node) (PluginToRBScores, *Status)

	// RunFilterPluginsVN runs the set of configured Filter plugins for subscription on
	// the given cluster.
	RunFilterPluginsVN(context.Context, *appsapi.Component, *coreV1.Node) PluginToStatus
}
