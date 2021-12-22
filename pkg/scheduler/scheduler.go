package scheduler

import (
	appsapi "gaia.io/gaia/pkg/apis/apps/v1alpha1"
	known "gaia.io/gaia/pkg/common"
	"gaia.io/gaia/pkg/controllers/apps/description"
	gaiaClientSet "gaia.io/gaia/pkg/generated/clientset/versioned"
	gaiainformers "gaia.io/gaia/pkg/generated/informers/externalversions"
	"k8s.io/klog/v2"
)

type Scheduler struct {
	//add some config here, but for now i don't know what that is.
	localGaiaClient *gaiaClientSet.Clientset
	localdescController   *description.Controller
	localInformerFactory  gaiainformers.SharedInformerFactory
	parentdescController  *description.Controller
	parentInformerFactory gaiainformers.SharedInformerFactory
}

func New(localGaiaClient *gaiaClientSet.Clientset) (*Scheduler, error) {
	sched := &Scheduler{
		localGaiaClient: localGaiaClient,
	}
	return sched.SetLocalDescController(localGaiaClient)
}

func (sched *Scheduler) NewDescController(gaiaClient *gaiaClientSet.Clientset, namespace string) (gaiainformers.SharedInformerFactory, *description.Controller, error) {
	selfGaiaInformerFactory := gaiainformers.NewSharedInformerFactoryWithOptions(gaiaClient,
		known.DefaultResync, gaiainformers.WithNamespace(namespace))

	descController, err := description.NewController(gaiaClient,
		selfGaiaInformerFactory.Apps().V1alpha1().Descriptions(), sched.handleDescription)
	if err != nil {
		return nil, nil, err
	}
	return selfGaiaInformerFactory, descController, err
}

func (sched *Scheduler) SetLocalDescController(gaiaClient *gaiaClientSet.Clientset) (*Scheduler, error) {
	localInformerFactory, localController, err := sched.NewDescController(gaiaClient, known.GaiaReservedNamespace)
	if err != nil {
		return nil, err
	}

	sched.localdescController = localController
	sched.localInformerFactory = localInformerFactory
	return sched, nil
}

func (sched *Scheduler) SetParentDescController(gaiaClient *gaiaClientSet.Clientset, namespace string) (*Scheduler, error) {
	parentInformerFactory, parentController, err := sched.NewDescController(gaiaClient, namespace)
	if err != nil {
		return nil, err
	}

	sched.parentdescController = parentController
	sched.parentInformerFactory = parentInformerFactory
	return sched, nil
}

func (sched *Scheduler) handleDescription(desc *appsapi.Description) error {
	klog.V(5).Infof("handle Description %s", klog.KObj(desc))
	return nil
}

func (sched *Scheduler) RunLocalScheduler(workers int, stopCh <-chan struct{}) {
	klog.Info("starting local desc scheduler...")
	defer klog.Info("shutting local scheduler")
	sched.localInformerFactory.Start(stopCh)

	go sched.localdescController.Run(workers, stopCh)
	<-stopCh
}

func (sched *Scheduler) RunParentScheduler(workers int, stopCh <-chan struct{}) {
	klog.Info("starting parent desc scheduler...")
	defer klog.Info("shutting parent scheduler")
	sched.parentInformerFactory.Start(stopCh)

	go sched.parentdescController.Run(workers, stopCh)
	<-stopCh
}
