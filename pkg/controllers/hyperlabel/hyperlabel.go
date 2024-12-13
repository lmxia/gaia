package hyperlabel

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	kubeInformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/dixudx/yacht"
	"github.com/lmxia/gaia/pkg/apis/service/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
	gaiaclientset "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	gaiaInformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	gaiav1alpha1 "github.com/lmxia/gaia/pkg/generated/listers/service/v1alpha1"
	"github.com/lmxia/gaia/pkg/utils"
)

type HyperLabelController struct {
	master kubernetes.Interface

	yachtController    *yacht.Controller
	hyperlabelLister   gaiav1alpha1.HyperLabelLister
	serviceListner     corelisters.ServiceLister
	localGaiaClientSet *gaiaclientset.Clientset
	sync.Mutex
}

func (r *HyperLabelController) Handle(obj interface{}) (requeueAfter *time.Duration, err error) {
	ctx := context.Background()
	d := 2 * time.Second
	key := obj.(string)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.Errorf("invalid service key: %s", key)
		return nil, nil
	}

	hyperlabel, err := r.hyperlabelLister.HyperLabels(namespace).Get(name)
	if err != nil {
		// don't retry, if we can't find it in client cluster.
		if apierrors.IsNotFound(err) {
			klog.Errorf("Skipping handler it for %s/%s missing in Kubernetes", namespace, name)
			return nil, nil
		}
		return &d, err
	}
	if hyperlabel.Status.PublicIPInfo == nil {
		hyperlabel.Status.PublicIPInfo = make(map[string]map[string]string)
	}

	for index := range hyperlabel.Spec {
		hyperlabelItem := &hyperlabel.Spec[index]
		err = r.deriveServiceFromHyperlabelItem(hyperlabelItem, hyperlabel)
		if err != nil {
			return &d, err
		}
	}

	_, err = r.localGaiaClientSet.ServiceV1alpha1().HyperLabels(hyperlabel.Namespace).UpdateStatus(ctx, hyperlabel,
		metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Update hyperlabel for %s/%s faile for %s", namespace, name, err)
		return nil, err
	}
	return nil, nil
}

func NewHyperLabelController(master kubernetes.Interface, localGaiaClientSet *gaiaclientset.Clientset,
	kubeInformerFactory kubeInformers.SharedInformerFactory,
	gaiarFactory gaiaInformers.SharedInformerFactory,
) *HyperLabelController {
	hyperlabelInformer := gaiarFactory.Service().V1alpha1().HyperLabels()
	hyperlabelLister := hyperlabelInformer.Lister()

	serviceInformer := kubeInformerFactory.Core().V1().Services()

	hyperLabelController := &HyperLabelController{
		master:             master,
		hyperlabelLister:   hyperlabelLister,
		serviceListner:     serviceInformer.Lister(),
		localGaiaClientSet: localGaiaClientSet,
	}
	// add event handler for ServiceImport
	yachtController := yacht.NewController("clientservice").
		WithCacheSynced(hyperlabelInformer.Informer().HasSynced, serviceInformer.Informer().HasSynced).
		WithHandlerFunc(hyperLabelController.Handle)

	_, err := hyperlabelInformer.Informer().AddEventHandler(yachtController.DefaultResourceEventHandlerFuncs())
	if err != nil {
		return nil
	}
	_, err = serviceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			svc := obj.(*v1.Service)
			// 过滤svc
			return utils.LetItGoOn(svc)
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				svc := obj.(*v1.Service)
				if se, err2 := getHyperlabelFromService(svc, hyperlabelLister); err2 == nil {
					yachtController.Enqueue(se)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				svc := newObj.(*v1.Service)
				if se, err2 := getHyperlabelFromService(svc, hyperlabelLister); err2 == nil {
					yachtController.Enqueue(se)
				}
			},
			DeleteFunc: func(obj interface{}) {
				svc := obj.(*v1.Service)
				if se, err2 := getHyperlabelFromService(svc, hyperlabelLister); err2 == nil {
					yachtController.Enqueue(se)
				}
			},
		},
	})
	if err != nil {
		return nil
	}

	hyperLabelController.yachtController = yachtController
	return hyperLabelController
}

func getHyperlabelFromService(svc *v1.Service, listner gaiav1alpha1.HyperLabelLister) (*v1alpha1.HyperLabel, error) {
	if name, exit := svc.Annotations[common.HyperLabelName]; exit {
		if hyperlabel, err := listner.HyperLabels(svc.Namespace).Get(name); err == nil {
			return hyperlabel, nil
		}
	}
	return nil, errors.New("fail to get hyperlabel name from this service")
}

func (r *HyperLabelController) Run(workers int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	ctx := context.Background()
	// set workers for service reflect controllers.
	r.yachtController.WithWorkers(workers)
	klog.V(4).Infof("Starting client service reflect controller...")
	go func() {
		r.yachtController.Run(ctx)
	}()
	<-stopCh
}

func (r *HyperLabelController) deriveServiceFromHyperlabelItem(hyperLabelItem *v1alpha1.HyperLabelItem,
	hyperLabel *v1alpha1.HyperLabel,
) error {
	allNeedService := make([]*v1.Service, 0)

	if hyperLabel.Status.PublicIPInfo[hyperLabelItem.ComponentName] == nil {
		hyperLabel.Status.PublicIPInfo[hyperLabelItem.ComponentName] = make(map[string]string, 0)
	}
	// ceni模式下，ceni ip的数量必须和vn数量一致
	if hyperLabelItem.ExposeType == common.ExposeTypeCENI {
		if len(hyperLabelItem.CeniIPList) != len(hyperLabelItem.VNList) {
			// 校验失败
			return fmt.Errorf("hyperlabelitem %s is not illegal", hyperLabelItem.ComponentName)
		}
	}
	// 一个hyperLabel Item 意味着，一个component的暴露方式
	for vnIndex, vnName := range hyperLabelItem.VNList {
		newDerivedService := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: hyperLabel.Namespace,
				Name:      GenerateDerivedServiceName(hyperLabelItem.ComponentName, vnName),
				Labels: map[string]string{
					common.ServiceManagedByLabel: vnName,
					common.HyperLabelName:        hyperLabelItem.ComponentName,
				},
			},
			Spec: v1.ServiceSpec{
				// 必须是loadbalancer
				Type:  v1.ServiceTypeLoadBalancer,
				Ports: hyperLabelItem.Ports,
			},
		}
		if hyperLabelItem.ExposeType == common.ExposeTypeCENI {
			newDerivedService.Annotations[common.VirtualClusterIPKey] = hyperLabelItem.CeniIPList[vnIndex]
		}
		allNeedService = append(allNeedService, newDerivedService)
	}

	serviceList, err := r.serviceListner.Services(hyperLabel.Namespace).List(
		labels.SelectorFromSet(labels.Set{
			common.ServiceManagedByLabel: hyperLabelItem.ComponentName,
		}))
	if err != nil {
		klog.Errorf("List derived service for hyperlabel(%s/%s) failed, Error: %v",
			hyperLabel.Namespace, hyperLabelItem.ComponentName, err)
		return err
	}

	srcServiceMap := make(map[string]bool)
	for _, item := range allNeedService {
		srcServiceMap[item.Name] = true
	}

	dstServiceMap := make(map[string]*v1.Service)
	for _, item := range serviceList {
		dstServiceMap[item.Name] = item
	}

	// remove none exist services
	err = utils.RemoveNonexistentService(r.master, srcServiceMap, serviceList)
	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	var allErrs []error
	errCh := make(chan error, len(allNeedService))
	// create or update existing services.
	for _, item := range allNeedService {
		wg.Add(1)
		srcService := item
		go func(svcName string, svc *v1.Service) {
			defer wg.Done()
			dstService := dstServiceMap[svcName]
			// 如果 spec 完全没变，去获得dstService然后更新hyperlabel的status
			if dstService != nil && reflect.DeepEqual(svc.Spec, dstService.Spec) {
				vnName, ips := utils.GetLoadbalancerIP(dstService)
				if len(ips) > 0 {
					// 更新一下hyperlabel的status，然后处理下一个
					// 这里并发修改，需要加锁
					r.Lock()
					hyperLabel.Status.PublicIPInfo[hyperLabelItem.ComponentName][vnName] = ips[0]
					r.Unlock()
				}
				return
			} else {
				if err = utils.ApplyServiceWithRetry(r.master, svc); err != nil {
					errCh <- err
					klog.Infof("svc %s sync err: %s", svc.Name, err)
				}
			}
		}(srcService.Name, srcService)
	}
	wg.Wait()
	// collect errors
	close(errCh)
	for err := range errCh {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) > 0 {
		reason := utilerrors.NewAggregate(allErrs).Error()
		msg := fmt.Sprintf("failed to sync service of hyperlabel %s/%s: %s",
			hyperLabel.Namespace, hyperLabelItem.ComponentName, reason)
		klog.ErrorDepth(5, msg)
		return err
	}
	klog.Infof("hyperlabelitem %s/%s has been synced successfully",
		hyperLabel.Namespace, hyperLabelItem.ComponentName)
	return nil
}

// GenerateDerivedServiceName vn name 过长，我们做一个128位hash，取前10位，作为生产的name
func GenerateDerivedServiceName(componentName, vnName string) string {
	hash := sha256.New()
	hash.Write([]byte(vnName))
	hashName := strings.ToLower(base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString(hash.Sum(nil)))[:10]
	return fmt.Sprintf("%s-%s", componentName, hashName)
}
