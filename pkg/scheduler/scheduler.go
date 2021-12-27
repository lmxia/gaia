package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	appsapi "gaia.io/gaia/pkg/apis/apps/v1alpha1"
	"gaia.io/gaia/pkg/apis/platform/v1alpha1"
	known "gaia.io/gaia/pkg/common"
	"gaia.io/gaia/pkg/controllers/apps/description"
	gaiaClientSet "gaia.io/gaia/pkg/generated/clientset/versioned"
	gaiainformers "gaia.io/gaia/pkg/generated/informers/externalversions"
	v1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sync"

	"gaia.io/gaia/pkg/utils"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	rest "k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	cacheddiscovery "k8s.io/client-go/discovery/cached/memory"

	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type Scheduler struct {
	//add some config here, but for now i don't know what that is.
	localGaiaClient       *gaiaClientSet.Clientset
	localSuperConfig      *rest.Config
	dynamicClient         dynamic.Interface
	schedulerCache        Cache
	localDescController   *description.Controller
	localInformerFactory  gaiainformers.SharedInformerFactory
	parentDescController  *description.Controller
	parentInformerFactory gaiainformers.SharedInformerFactory
	discoveryRESTMapper   *restmapper.DeferredDiscoveryRESTMapper
}

func New(localGaiaClient *gaiaClientSet.Clientset, localGaiaAllFactory gaiainformers.SharedInformerFactory, config *rest.Config) (*Scheduler, error) {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	childKubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	sched := &Scheduler{
		localGaiaClient:  localGaiaClient,
		schedulerCache:   newSchedulerCache(),
		localSuperConfig: config,
		dynamicClient: dynamicClient,
		discoveryRESTMapper: restmapper.NewDeferredDiscoveryRESTMapper(cacheddiscovery.NewMemCacheClient(childKubeClient.Discovery())),
	}
	return sched.SetLocalDescController(localGaiaClient, localGaiaAllFactory, known.GaiaReservedNamespace)
}

func (sched *Scheduler) NewDescController(gaiaClient *gaiaClientSet.Clientset, gaiaAllFactory gaiainformers.SharedInformerFactory, namespace string) (gaiainformers.SharedInformerFactory, *description.Controller, error) {
	selfGaiaInformerFactory := gaiainformers.NewSharedInformerFactoryWithOptions(gaiaClient,
		known.DefaultResync, gaiainformers.WithNamespace(namespace))

	var descController *description.Controller
	var err error

	if gaiaAllFactory != nil {
		descController, err = description.NewController(gaiaClient, selfGaiaInformerFactory.Apps().V1alpha1().Descriptions(),
			gaiaAllFactory.Platform().V1alpha1().ManagedClusters(), cache.ResourceEventHandlerFuncs{
				AddFunc:    sched.addClusterToCache,
				UpdateFunc: sched.updateClusterInCache,
				DeleteFunc: sched.deleteClusterFromCache,
			}, sched.handleDescription)
	} else {
		descController, err = description.NewController(gaiaClient, selfGaiaInformerFactory.Apps().V1alpha1().Descriptions(),
			nil, nil, sched.handleDescription)
	}
	if err != nil {
		return nil, nil, err
	}
	return selfGaiaInformerFactory, descController, err
}

func (sched *Scheduler) SetLocalDescController(gaiaClient *gaiaClientSet.Clientset, gaiaAllFactory gaiainformers.SharedInformerFactory, namespace string) (*Scheduler, error) {
	localInformerFactory, localController, err := sched.NewDescController(gaiaClient, gaiaAllFactory, namespace)
	if err != nil {
		return nil, err
	}

	sched.localDescController = localController
	sched.localInformerFactory = localInformerFactory
	return sched, nil
}

func (sched *Scheduler) SetParentDescController(gaiaClient *gaiaClientSet.Clientset, namespace string) (*Scheduler, error) {
	parentInformerFactory, parentController, err := sched.NewDescController(gaiaClient, nil, namespace)
	if err != nil {
		return nil, err
	}

	sched.parentDescController = parentController
	sched.parentInformerFactory = parentInformerFactory
	return sched, nil
}

func (sched *Scheduler) handleDescription(desc *appsapi.Description) error {
	klog.V(5).Infof("handle Description %s", klog.KObj(desc))
	clusters := sched.schedulerCache.GetClusters()
	// no joined clusters, deploy to local
	if len(clusters) == 0 {
		if error := utils.ApplyDescription(context.TODO(), sched.localGaiaClient, sched.dynamicClient, sched.discoveryRESTMapper,desc); error!=nil {
			return fmt.Errorf("there is no clusters so we dont need to schedule across sub-clusters")
		}
		return nil
	} else {
		// need schedule across clusters
		if error := sched.ApplyAcrosClusters(context.TODO(), desc); error!=nil {
			return fmt.Errorf("schedule across sub-clusters failed")
		}
	}
	return nil
}

func (sched *Scheduler) RunLocalScheduler(workers int, stopCh <-chan struct{}) {
	klog.Info("starting local desc scheduler...")
	defer klog.Info("shutting local scheduler")
	sched.localInformerFactory.Start(stopCh)
	go sched.localDescController.Run(workers, stopCh)
	<-stopCh
}

func (sched *Scheduler) RunParentScheduler(workers int, stopCh <-chan struct{}) {
	klog.Info("starting parent desc scheduler...")
	defer klog.Info("shutting parent scheduler")
	sched.parentInformerFactory.Start(stopCh)
	go sched.parentDescController.Run(workers, stopCh)
	<-stopCh
}

func (sched *Scheduler) addClusterToCache(obj interface{}) {
	cluster, ok := obj.(*v1alpha1.ManagedCluster)
	if !ok {
		klog.ErrorS(nil, "Cannot convert to *v1alpha1.ManagedCluster", "obj", obj)
		return
	}

	sched.schedulerCache.AddCluster(cluster)
}

func (sched *Scheduler) updateClusterInCache(oldObj, newObj interface{}) {
	oldCluster, ok := oldObj.(*v1alpha1.ManagedCluster)
	if !ok {
		klog.ErrorS(nil, "Cannot convert oldObj to *v1alpha1.ManagedCluster", "oldObj", oldObj)
		return
	}
	newCluster, ok := newObj.(*v1alpha1.ManagedCluster)
	if !ok {
		klog.ErrorS(nil, "Cannot convert newObj to *v1alpha1.ManagedCluster", "newObj", newObj)
		return
	}

	sched.schedulerCache.UpdateCluster(oldCluster, newCluster)
}

func (sched *Scheduler) deleteClusterFromCache(obj interface{}) {
	var cluster *v1alpha1.ManagedCluster
	switch t := obj.(type) {
	case *v1alpha1.ManagedCluster:
		cluster = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		cluster, ok = t.Obj.(*v1alpha1.ManagedCluster)
		if !ok {
			klog.ErrorS(nil, "Cannot convert to *v1alpha1.ManagedCluster", "obj", t.Obj)
			return
		}
	default:
		klog.ErrorS(nil, "Cannot convert to *v1alpha1.ManagedCluster", "obj", t)
		return
	}
	if err := sched.schedulerCache.RemoveCluster(cluster); err != nil {
		klog.ErrorS(err, "Scheduler cache RemoveCluster failed")
	}
}


func (sched *Scheduler) ApplyAcrosClusters(ctx context.Context, desc *appsapi.Description) error {
	var allErrs []error
	wg := sync.WaitGroup{}
	objectsToBeDeployed := desc.Spec.Raw
	errCh := make(chan error, len(objectsToBeDeployed))
	// 1. 判断是否是deployment
	deployGVK := schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}
	for i, object := range objectsToBeDeployed {
		resource := &unstructured.Unstructured{}
		err := resource.UnmarshalJSON(object)
		if err != nil {
			allErrs = append(allErrs, err)
			msg := fmt.Sprintf("failed to unmarshal resource: %v", err)
			klog.ErrorDepth(5, msg)
			continue
		}

		if resource.GroupVersionKind().String() == deployGVK.String() {
			wg.Add(1)
			go func(resource *unstructured.Unstructured, raw []byte) {
				defer wg.Done()
				dep := &v1.Deployment{}
				// TODO check may it's not a deployment
				json.Unmarshal(raw, dep)
				scheduleResult, _, _ := sched.ScheduleDeploymentOverClusters(dep, sched.schedulerCache.GetClusters())
				for clusterName, replicas := range scheduleResult {
					managedCluster := sched.schedulerCache.GetCluster(clusterName)
					deprep := int32(replicas)
					dep.Spec.Replicas = &deprep
					dep.Namespace = managedCluster.Namespace
					depRaw, _:= json.Marshal(dep)
					newDesc := desc.DeepCopy()
					newDesc.Spec.Raw[i] = depRaw
					_, err := sched.localGaiaClient.AppsV1alpha1().Descriptions(managedCluster.Namespace).Get(ctx, desc.Name, metav1.GetOptions{})
					if err == nil {
						// update
						sched.localGaiaClient.AppsV1alpha1().Descriptions(managedCluster.Namespace).Update(ctx,newDesc, metav1.UpdateOptions{})
					} else {
						if apierrors.IsNotFound(err) {
							sched.localGaiaClient.AppsV1alpha1().Descriptions(managedCluster.Namespace).Create(ctx, newDesc, metav1.CreateOptions{})
						}
					}
				}

			}(resource, object)
			break
			// 只处理deployment.
		}
	}
	wg.Wait()

	// collect errors
	close(errCh)
	for err := range errCh {
		allErrs = append(allErrs, err)
	}

	var statusPhase appsapi.DescriptionPhase
	var reason string
	if len(allErrs) > 0 {
		statusPhase = appsapi.DescriptionPhaseFailure
		reason = utilerrors.NewAggregate(allErrs).Error()

		msg := fmt.Sprintf("failed to deploying Description %s: %s", klog.KObj(desc), reason)
		klog.ErrorDepth(5, msg)
	} else {
		statusPhase = appsapi.DescriptionPhaseSuccess
		reason = ""

		msg := fmt.Sprintf("Description %s is deployed successfully", klog.KObj(desc))
		klog.V(5).Info(msg)
	}

	// update status
	desc.Status.Phase = statusPhase
	desc.Status.Reason = reason
	_, err := sched.localGaiaClient.AppsV1alpha1().Descriptions(desc.Namespace).UpdateStatus(context.TODO(), desc, metav1.UpdateOptions{})

	if len(allErrs) > 0 {
		return utilerrors.NewAggregate(allErrs)
	}
	return err

}
