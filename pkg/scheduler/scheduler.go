package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	appsapi "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	known "github.com/lmxia/gaia/pkg/common"
	"github.com/lmxia/gaia/pkg/controllers/apps/description"
	gaiaClientSet "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	gaiainformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sync"

	"github.com/lmxia/gaia/pkg/utils"
	cacheddiscovery "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	rest "k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"

	"github.com/lmxia/gaia/pkg/common"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type Scheduler struct {
	// add some config here, but for now i don't know what that is.
	localGaiaClient       *gaiaClientSet.Clientset
	localSuperConfig      *rest.Config
	dynamicClient         dynamic.Interface
	schedulerCache        Cache
	localDescController   *description.Controller
	localInformerFactory  gaiainformers.SharedInformerFactory // namespaced
	parentDescController  *description.Controller
	parentInformerFactory gaiainformers.SharedInformerFactory // namespaced
	discoveryRESTMapper   *restmapper.DeferredDiscoveryRESTMapper
	localGaiaAllFactory   gaiainformers.SharedInformerFactory // all ns
	parentGaiaClient      *gaiaClientSet.Clientset
	// clientset for child cluster
	childKubeClientSet kubernetes.Interface
	// dedicated kubeconfig for accessing parent cluster, which is auto populated by the parent cluster when cluster registration request gets approved
	parentDedicatedKubeConfig *rest.Config
	dedicatedNamespace        string `json:"dedicatednamespace,omitempty" protobuf:"bytes,1,opt,name=dedicatedNamespace"`
}

func New(localGaiaClient *gaiaClientSet.Clientset, localGaiaAllFactory gaiainformers.SharedInformerFactory, localSuperKubeConfig *rest.Config, childKubeClientSet kubernetes.Interface) (*Scheduler, error) {
	klog.Infof("eNew host ==== %v\n", localSuperKubeConfig.Host)
	dynamicClient, err := dynamic.NewForConfig(localSuperKubeConfig)
	if err != nil {
		return nil, err
	}
	childKubeClient, err := kubernetes.NewForConfig(localSuperKubeConfig)
	if err != nil {
		return nil, err
	}

	sched := &Scheduler{
		localGaiaClient:     localGaiaClient,
		schedulerCache:      newSchedulerCache(),
		localSuperConfig:    localSuperKubeConfig,
		dynamicClient:       dynamicClient,
		localGaiaAllFactory: localGaiaAllFactory,
		discoveryRESTMapper: restmapper.NewDeferredDiscoveryRESTMapper(cacheddiscovery.NewMemCacheClient(childKubeClient.Discovery())),
		childKubeClientSet:  childKubeClientSet,
	}
	return sched.SetLocalDescController(localGaiaClient, known.GaiaReservedNamespace)
}

func (sched *Scheduler) NewDescController(gaiaClient *gaiaClientSet.Clientset, gaiaAllFactory gaiainformers.SharedInformerFactory, namespace string) (gaiainformers.SharedInformerFactory, *description.Controller, error) {
	gaiaInformerFactory := gaiainformers.NewSharedInformerFactoryWithOptions(gaiaClient, known.DefaultResync, gaiainformers.WithNamespace(namespace))

	var descController *description.Controller
	var err error

	descController, err = description.NewController(gaiaClient, gaiaInformerFactory.Apps().V1alpha1().Descriptions(), gaiaAllFactory.Platform().V1alpha1().ManagedClusters(),
		cache.ResourceEventHandlerFuncs{
			AddFunc:    sched.addClusterToCache,
			UpdateFunc: sched.updateClusterInCache,
			DeleteFunc: sched.deleteClusterFromCache,
		}, sched.handleDescription)

	if err != nil {
		return nil, nil, err
	}
	return gaiaInformerFactory, descController, err
}

func (sched *Scheduler) SetLocalDescController(gaiaClient *gaiaClientSet.Clientset, namespace string) (*Scheduler, error) {
	localInformerFactory, localController, err := sched.NewDescController(gaiaClient, sched.localGaiaAllFactory, namespace)
	if err != nil {
		return nil, err
	}

	sched.localDescController = localController
	sched.localInformerFactory = localInformerFactory
	return sched, nil
}

func (sched *Scheduler) SetParentDescController(parentGaiaClient *gaiaClientSet.Clientset, parentNamespace string) (*Scheduler, error) {
	parentInformerFactory, parentController, err := sched.NewDescController(parentGaiaClient, sched.localGaiaAllFactory, parentNamespace)
	if err != nil {
		return nil, err
	}
	sched.parentGaiaClient = parentGaiaClient
	sched.parentDescController = parentController
	sched.parentInformerFactory = parentInformerFactory
	return sched, nil
}

func (sched *Scheduler) handleDescription(desc *appsapi.Description) error {
	klog.V(5).Infof("handle Description %s", klog.KObj(desc))
	clusters := sched.schedulerCache.GetClusters()
	labelsDesc := desc.GetLabels()
	clusterLevel, ok := labelsDesc["clusterlevel"]
	// desc.meta.labels.clusterlevel empty, cluster default
	if !ok {
		if len(clusters) == 0 {
			if desc.DeletionTimestamp != nil {
				return utils.OffloadDescription(context.TODO(), sched.parentGaiaClient, sched.dynamicClient,
					sched.discoveryRESTMapper, desc)
			}
			if error := utils.ApplyDescription(context.TODO(), sched.parentGaiaClient, sched.dynamicClient, sched.discoveryRESTMapper, desc); error != nil {
				return fmt.Errorf("there is no clusters so we dont need to schedule across sub-clusters")
			}
			return nil
		} else {
			if desc.DeletionTimestamp != nil {
				return sched.OffloadAccrossClusters(context.TODO(), desc)
			}
			// need schedule across clusters
			if error := sched.ApplyAccrosClusters(context.TODO(), desc); error != nil {
				return fmt.Errorf("schedule across sub-clusters failed")
			}
		}
	} else if clusterLevel == "cluster" {
		// no joined clusters, deploy to local
		if len(clusters) == 0 {
			if desc.DeletionTimestamp != nil {
				return utils.OffloadDescription(context.TODO(), sched.parentGaiaClient, sched.dynamicClient,
					sched.discoveryRESTMapper, desc)
			}
			if error := utils.ApplyDescription(context.TODO(), sched.parentGaiaClient, sched.dynamicClient, sched.discoveryRESTMapper, desc); error != nil {
				return fmt.Errorf("clusterlevel=cluster: ApplyDescription ERROR")
			}
		} else {
			if desc.DeletionTimestamp != nil {
				return sched.OffloadAccrossClusters(context.TODO(), desc)
			}
			// need schedule across clusters
			if error := sched.ApplyAccrosClusters(context.TODO(), desc); error != nil {
				return fmt.Errorf("clusterlevel=cluster: schedule across sub-clusters failed")
			}
		}
	} else if clusterLevel == "field" {
		if sched.parentGaiaClient != nil {
			if desc.DeletionTimestamp != nil {
				return utils.OffloadDescription(context.TODO(), sched.parentGaiaClient, sched.dynamicClient,
					sched.discoveryRESTMapper, desc)
			}
			if error := utils.ApplyDescription(context.TODO(), sched.parentGaiaClient, sched.dynamicClient, sched.discoveryRESTMapper, desc); error != nil {
				return fmt.Errorf("clusterlevel=field: ApplyDescription ERROR")
			}
			return nil
		} else {
			if desc.DeletionTimestamp != nil {
				return sched.OffloadAccrossClusters(context.TODO(), desc)
			}
			// need schedule across clusters
			if error := sched.ApplyAccrosClusters(context.TODO(), desc); error != nil {
				return fmt.Errorf("clusterlevel=field: schedule across sub-clusters failed")
			}
		}
	} else if clusterLevel == "global" {
		if sched.parentGaiaClient == nil {
			if desc.DeletionTimestamp != nil {
				return utils.OffloadDescription(context.TODO(), sched.localGaiaClient, sched.dynamicClient,
					sched.discoveryRESTMapper, desc)
			}
			if error := utils.ApplyDescription(context.TODO(), sched.localGaiaClient, sched.dynamicClient, sched.discoveryRESTMapper, desc); error != nil {
				return fmt.Errorf("clusterlevel=global: ApplyDescription ERROR")
			}
		}
	} else {
		return fmt.Errorf("desc.meta.labels.clusterlevel ERROR")
	}

	return nil
}

func (sched *Scheduler) RunLocalScheduler(workers int, stopCh <-chan struct{}) {
	klog.Info("starting local desc scheduler...")
	defer klog.Info("shutting local scheduler")
	klog.Info("starting sharedInformerFactory ...")
	sched.localInformerFactory.Start(stopCh)
	klog.Info("starting local desc scheduler ...")
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

func (sched *Scheduler) ApplyAccrosClusters(ctx context.Context, desc *appsapi.Description) error {
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
					depRaw, _ := json.Marshal(dep)

					curDesc, err := sched.localGaiaClient.AppsV1alpha1().Descriptions(managedCluster.Namespace).Get(ctx, desc.Name, metav1.GetOptions{})
					if err == nil {
						if replicas == 0 {
							// delete
							er := sched.localGaiaClient.AppsV1alpha1().Descriptions(managedCluster.Namespace).Delete(ctx, desc.Name, metav1.DeleteOptions{})
							if er != nil {
								errCh <- err
							}
						} else {
							// update
							curDesc.Spec.Raw[i] = depRaw
							_, er := sched.localGaiaClient.AppsV1alpha1().Descriptions(managedCluster.Namespace).Update(ctx, curDesc, metav1.UpdateOptions{})
							if er != nil {
								errCh <- err
							}
						}

					} else {
						if apierrors.IsNotFound(err) {
							if replicas == 0 {
								// no need to create desc in this cluster.
								continue
							}
							labels := desc.GetLabels()
							if labels == nil {
								labels = map[string]string{}
							}
							labels[known.AppsNameLabel] = desc.Name

							// every ns desc must have a finalizer, so we can make sure sub resources can be recycled.
							newDesc := &appsapi.Description{
								ObjectMeta: metav1.ObjectMeta{
									Namespace: managedCluster.Namespace,
									Name:      desc.GetName(),
									Labels:    labels,
									Finalizers: []string{
										known.AppFinalizer,
									},
								},
							}
							newDesc.Spec.Raw = make([][]byte, len(desc.Spec.Raw))
							copy(newDesc.Spec.Raw, desc.Spec.Raw)
							newDesc.Spec.Raw[i] = depRaw
							_, err := sched.localGaiaClient.AppsV1alpha1().Descriptions(managedCluster.Namespace).Create(ctx, newDesc, metav1.CreateOptions{})
							if err != nil {
								errCh <- err
							}
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

	// description come from local ns
	var err error
	if desc.GetNamespace() == known.GaiaReservedNamespace {
		_, err = sched.localGaiaClient.AppsV1alpha1().Descriptions(desc.Namespace).UpdateStatus(context.TODO(), desc, metav1.UpdateOptions{})
	} else {
		_, err = sched.parentGaiaClient.AppsV1alpha1().Descriptions(desc.Namespace).UpdateStatus(context.TODO(), desc, metav1.UpdateOptions{})
	}

	if len(allErrs) > 0 {
		return utilerrors.NewAggregate(allErrs)
	}
	return err

}

func (sched *Scheduler) OffloadAccrossClusters(ctx context.Context, description *appsapi.Description) error {
	var allErrs []error
	descrs, err := sched.localGaiaClient.AppsV1alpha1().Descriptions(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{
			known.AppsNameLabel: description.Name,
		}).String(),
	})
	errCh := make(chan error, len(descrs.Items))
	wg := sync.WaitGroup{}
	for _, desc := range descrs.Items {
		wg.Add(1)
		descDel := desc
		go func(desc *appsapi.Description) {
			defer wg.Done()
			err := sched.localGaiaClient.AppsV1alpha1().Descriptions(desc.Namespace).Delete(ctx, desc.Name, metav1.DeleteOptions{})
			if err != nil {
				errCh <- err
			}
		}(&descDel)

	}
	wg.Wait()
	// collect errors
	close(errCh)
	for err := range errCh {
		allErrs = append(allErrs, err)
	}

	err = utilerrors.NewAggregate(allErrs)
	if err == nil {
		descCopy := description.DeepCopy()
		descCopy.Finalizers = utils.RemoveString(descCopy.Finalizers, known.AppFinalizer)
		if description.GetNamespace() == known.GaiaReservedNamespace {
			_, err = sched.localGaiaClient.AppsV1alpha1().Descriptions(descCopy.Namespace).Update(context.TODO(), descCopy, metav1.UpdateOptions{})
		} else {
			_, err = sched.parentGaiaClient.AppsV1alpha1().Descriptions(descCopy.Namespace).Update(context.TODO(), descCopy, metav1.UpdateOptions{})
		}

		if err != nil {
			klog.WarningDepth(4, fmt.Sprintf("failed to remove finalizer %s from Description %s: %v", known.AppFinalizer, klog.KObj(descCopy), err))

		}
		return err
	} else {
		msg := fmt.Sprintf("failed to deleting Description %s: %v", klog.KObj(description), err)
		klog.ErrorDepth(5, msg)
	}
	return nil
}

func (sched *Scheduler) SetparentDedicatedKubeConfig(ctx context.Context) {
	// complete your controller loop here
	klog.Info("start set parent DedicatedKubeConfig current cluster as a child cluster...")
	// wait until stopCh is closed or request is approved
	waitingCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	wait.JitterUntilWithContext(waitingCtx, func(ctx context.Context) {
		target, err := sched.localGaiaClient.PlatformV1alpha1().Targets().Get(ctx, common.ParentClusterTargetName, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("set parentDedicatedKubeConfig failed to get targets: %v wait for next loop", err)
			return
		}
		secret, err := sched.childKubeClientSet.CoreV1().Secrets(common.GaiaSystemNamespace).Get(ctx, common.ParentClusterSecretName, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			klog.Errorf("set parentDedicatedKubeConfig failed to get secretFromParentCluster: %v", err)
			return
		}
		if err == nil {
			klog.Infof("found existing secretFromParentCluster '%s/%s' that can be used to access parent cluster", common.GaiaSystemNamespace, common.ParentClusterSecretName)

			if string(secret.Data[common.ClusterAPIServerURLKey]) != target.Spec.ParentURL {
				klog.Warningf("the parent url got changed from %q to %q", secret.Data[known.ClusterAPIServerURLKey], target.Spec.ParentURL)
				klog.Warningf("will try to re-get current cluster secret")
			} else {
				parentDedicatedKubeConfig, err := utils.GenerateKubeConfigFromToken(target.Spec.ParentURL, string(secret.Data[corev1.ServiceAccountTokenKey]), secret.Data[corev1.ServiceAccountRootCAKey], 2)
				if err == nil {
					sched.parentDedicatedKubeConfig = parentDedicatedKubeConfig
					sched.dedicatedNamespace = string(secret.Data[corev1.ServiceAccountNamespaceKey])
				}
			}
		}

		klog.V(4).Infof("set parentDedicatedKubeConfig still waiting for getting secret...", target.Name)
		cancel()
	}, known.DefaultRetryPeriod, 0.3, true)

}

func (sched *Scheduler) GetparentDedicatedKubeConfig() *rest.Config {
	// complete your controller loop here
	klog.Info(" get parent DedicatedKubeConfig current cluster as a child cluster...")
	fmt.Printf("sched.parentDedicatedKubeConfig host == %s \n", sched.parentDedicatedKubeConfig.Host)
	return sched.parentDedicatedKubeConfig
}
func (sched *Scheduler) GetDedicatedNamespace() string {
	// complete your controller loop here
	fmt.Printf("sched.GetDedicatedNamespace == %s \n", sched.dedicatedNamespace)
	return sched.dedicatedNamespace
}
