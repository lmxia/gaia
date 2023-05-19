/*
Copyright 2021 The Gaia Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resourcebinding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	appV1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
	"github.com/lmxia/gaia/pkg/controllers/resourcebindingmerger"
	gaiaClientSet "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	gaiaInformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	appsLister "github.com/lmxia/gaia/pkg/generated/listers/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/utils"
	"github.com/lmxia/gaia/pkg/utils/cartesian"
	"io"
	coreV1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilRuntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"strconv"
	"sync"
)

type UID string

// RBMerger defines configuration for ResourceBindings Merger
type RBMerger struct {
	rbToLocalController  *resourcebindingmerger.Controller
	rbTOParentController *resourcebindingmerger.Controller

	localKubeClient          *kubernetes.Clientset
	localGaiaClient          *gaiaClientSet.Clientset
	localGaiaInformerFactory gaiaInformers.SharedInformerFactory
	rbLister                 appsLister.ResourceBindingLister
	selfClusterName          string
	parentNameSpace          string
	parentGaiaClient         *gaiaClientSet.Clientset

	mu                      sync.Mutex
	clustersRBsOfOneFieldRB map[string]*ClustersRBs // map[descUID-parentRBName]*ClustersRBs
	fieldsRBOfOneParentRB   map[string]*FieldsRBs   // map[descUID-parentRBName]*FieldsRBs
	parentRBsOfDescUID      map[UID][]string        // map[descUID][descUID-parentRBName...]
	descUID                 map[string]bool
	postURL                 string
}

// ClustersRBs contains all cluster RB from field's mCls in a parentRB
type ClustersRBs struct {
	countRB       int
	rbNames       []string
	rbsOfParentRB []*appV1alpha1.ResourceBindingApps
}

// FieldsRBs contains all RB from mCls in a parentRB
type FieldsRBs struct {
	countCls        int
	NamesOfFieldRBs []string
	rbsOfFields     []*ClustersRBs
}

// NewRBMerger returns a new RBMerger for ResourceBinding.
func NewRBMerger(kubeClient *kubernetes.Clientset, gaiaClient *gaiaClientSet.Clientset,
	gaiaInformerFactory gaiaInformers.SharedInformerFactory) (*RBMerger, error) {

	postUrl := os.Getenv(common.ResourceBindMergePostURL)
	rbMerger := &RBMerger{
		localKubeClient:          kubeClient,
		localGaiaClient:          gaiaClient,
		localGaiaInformerFactory: gaiaInformerFactory,
		rbLister:                 gaiaInformerFactory.Apps().V1alpha1().ResourceBindings().Lister(),
		clustersRBsOfOneFieldRB:  make(map[string]*ClustersRBs),
		fieldsRBOfOneParentRB:    make(map[string]*FieldsRBs),
		parentRBsOfDescUID:       make(map[UID][]string),
		descUID:                  make(map[string]bool, 0),
		postURL:                  postUrl,
	}

	rbLocalController, err := resourcebindingmerger.NewController(gaiaClient,
		gaiaInformerFactory.Apps().V1alpha1().ResourceBindings(),
		rbMerger.handleToLocalResourceBinding)
	if err != nil {
		return nil, err
	}
	rbMerger.rbToLocalController = rbLocalController

	return rbMerger, nil
}

func (rbMerger *RBMerger) RunToLocalResourceBindingMerger(workers int, stopCh <-chan struct{}) {
	klog.Info("Starting local ResourceBinding Merger ...")
	defer klog.Info("Shutting local ResourceBinding Merger ...")
	// todo: goroutines
	rbMerger.rbToLocalController.Run(workers, stopCh)
	return
}

func (rbMerger *RBMerger) RunToParentResourceBindingMerger(workers int, stopCh <-chan struct{}) {
	klog.Info("Starting parent ResourceBinding Merger ...")
	defer klog.Info("Shutting parent ResourceBinding Merger ...")
	// todo: goroutines
	rbMerger.rbTOParentController.Run(workers, stopCh)
	return
}

func (rbMerger *RBMerger) SetParentRBController() (*RBMerger, error) {
	parentGaiaClient, _, _ := utils.SetParentClient(rbMerger.localKubeClient, rbMerger.localGaiaClient)
	selfClusterName, parentNameSpace, errClusterName := utils.GetLocalClusterName(rbMerger.localKubeClient)
	if errClusterName != nil {
		klog.Errorf("local handleResourceBinding failed to get clusterName From secret: %v", errClusterName)
		return nil, errClusterName
	}
	rbMerger.parentGaiaClient = parentGaiaClient
	rbMerger.selfClusterName = selfClusterName
	rbMerger.parentNameSpace = parentNameSpace

	rbTOParentController, err := resourcebindingmerger.NewController(rbMerger.localGaiaClient,
		rbMerger.localGaiaInformerFactory.Apps().V1alpha1().ResourceBindings(),
		rbMerger.handleToParentResourceBinding)
	if err != nil {
		return nil, err
	}
	rbMerger.rbTOParentController = rbTOParentController
	return rbMerger, nil
}

func (rbMerger *RBMerger) handleToParentResourceBinding(rb *appV1alpha1.ResourceBinding) error {
	klog.V(5).Infof("handle local resourceBinding %s", klog.KObj(rb))

	clusters, err := rbMerger.localGaiaClient.PlatformV1alpha1().ManagedClusters(coreV1.NamespaceAll).List(context.TODO(), metaV1.ListOptions{})
	if err != nil {
		klog.Warningf("failed to list managed clusters: %v", err)
	}
	// TODO: 重构掉
	if rb.Namespace != common.GaiaRSToBeMergedReservedNamespace {
		klog.V(4).Infof("The ResourceBinding not in %s namespace.", common.GaiaRSToBeMergedReservedNamespace)
		return nil
	}
	if rb.Spec.StatusScheduler != "" {
		klog.V(4).Infof("ResourceBinding %s has already been processed with Result %q. Skip it.", klog.KObj(rb), rb.Status.Status)
		return nil
	}

	// level cluster
	if len(clusters.Items) == 0 {
		err = rbMerger.reCreateRBtoParent(rb)
		if apiErrors.IsAlreadyExists(err) || err == nil {
			klog.Infof("created ResourceBinding %q to parent cluster successfully", klog.KObj(rb))
			err = rbMerger.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRSToBeMergedReservedNamespace).Delete(context.TODO(), rb.Name, metaV1.DeleteOptions{})
			if err != nil {
				klog.Errorf("failed to delete local ResourceBinding %q. error: %v", klog.KObj(rb).String(), err)
				return err
			}
		} else {
			klog.Errorf("failed to create ResourceBinding %q to parent cluster. Error: %v", klog.KObj(rb).String(), err)
			return err
		}
		return nil
	} else if rbMerger.parentGaiaClient != nil { // level field
		rbMerger.mu.Lock()
		defer rbMerger.mu.Unlock()
		rbLabels := rb.GetLabels()
		uid := rbLabels[common.OriginDescriptionUIDLabel]
		indexParentRB := uid + "-" + rb.Spec.ParentRB

		if !utils.ContainsString(rbMerger.parentRBsOfDescUID[UID(uid)], indexParentRB) {
			rbMerger.parentRBsOfDescUID[UID(uid)] = append(rbMerger.parentRBsOfDescUID[UID(uid)], indexParentRB)
		}
		if rbMerger.clustersRBsOfOneFieldRB[indexParentRB] == nil {
			rbMerger.clustersRBsOfOneFieldRB[indexParentRB] = &ClustersRBs{}
		}
		for _, value := range rb.Spec.RbApps {
			if value.Children != nil {
				rbMerger.clustersRBsOfOneFieldRB[indexParentRB].countRB = rb.Spec.TotalPeer
				rbMerger.clustersRBsOfOneFieldRB[indexParentRB].rbNames = append(rbMerger.clustersRBsOfOneFieldRB[indexParentRB].rbNames, rb.Name)
				rbMerger.clustersRBsOfOneFieldRB[indexParentRB].rbsOfParentRB = append(rbMerger.clustersRBsOfOneFieldRB[indexParentRB].rbsOfParentRB, value)
			}
		}

		v, exist := rbMerger.descUID[uid]
		if rbMerger.canCreateCollectedRBs(rb, rbLabels, indexParentRB) && v != true {
			if rbMerger.createCollectedRBs(rb, rbLabels) {
				rbMerger.deleteFieldDescUID(UID(uid))
				rbMerger.descUID[uid] = true

				err = rbMerger.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRSToBeMergedReservedNamespace).
					DeleteCollection(context.TODO(), metaV1.DeleteOptions{}, metaV1.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{
						common.OriginDescriptionNameLabel:      rbLabels[common.OriginDescriptionNameLabel],
						common.OriginDescriptionNamespaceLabel: rbLabels[common.OriginDescriptionNamespaceLabel],
						common.OriginDescriptionUIDLabel:       rbLabels[common.OriginDescriptionUIDLabel],
					}).String()})
				if err != nil {
					klog.Infof("failed to delete rbs in %s namespace", common.GaiaRBMergedReservedNamespace, err)
					return err
				}
			}
		} else {
			if !exist {
				rbMerger.descUID[uid] = false
			} else if v == true {
				_ = rbMerger.deleteRB(rb)
				rbMerger.deleteFieldDescUID(UID(uid))
			}
		}
	} else { // global
		return nil
	}
	return nil
}

func (rbMerger *RBMerger) handleToLocalResourceBinding(rb *appV1alpha1.ResourceBinding) error {
	klog.V(5).Infof("handle local resourceBinding %s", klog.KObj(rb))

	if rb.Namespace != common.GaiaRSToBeMergedReservedNamespace && rb.Namespace != common.GaiaRBMergedReservedNamespace {
		return nil
	}
	// ResourceBing in gaia-merged namespace
	// TODO: 重构迁移到明翔代码
	if rb.Namespace == common.GaiaRBMergedReservedNamespace && rb.Spec.StatusScheduler == appV1alpha1.ResourceBindingSelected {
		err := rbMerger.UpdateSelectedResourceBinding(rb)
		if err != nil {
			return err
		}
		return nil
	}
	rbLabels := rb.GetLabels()
	// ResourceBinding in gaia-to-be-merged namespace
	if rb.Namespace == common.GaiaRSToBeMergedReservedNamespace && rb.Spec.StatusScheduler == appV1alpha1.ResourceBindingMerging {
		clusters, err := rbMerger.localGaiaClient.PlatformV1alpha1().ManagedClusters(coreV1.NamespaceAll).List(context.TODO(), metaV1.ListOptions{})
		if err != nil {
			klog.Warningf("handleToLocalResourceBinding: failed to list managed clusters: %v", err)
		}
		descUID := rbLabels[common.OriginDescriptionUIDLabel]
		chanResult := make(chan []*appV1alpha1.ResourceBindingApps)
		rbMerger.mu.Lock()
		defer rbMerger.mu.Unlock()

		// clustersRBsOfOneFieldRB: map[descUID-RBName]*ClustersRBs
		indexFieldRB := descUID + "-" + rb.Name
		if rbMerger.clustersRBsOfOneFieldRB[indexFieldRB] == nil {
			rbMerger.clustersRBsOfOneFieldRB[indexFieldRB] = &ClustersRBs{}
		} else {
			return fmt.Errorf("handleToLocalResourceBinding: ERROR: already handle ResourceBinding %q", klog.KObj(rb).String())
		}
		for _, rbApp := range rb.Spec.RbApps {
			rbMerger.clustersRBsOfOneFieldRB[indexFieldRB].rbsOfParentRB = append(rbMerger.clustersRBsOfOneFieldRB[indexFieldRB].rbsOfParentRB, rbApp.Children[0])
		}

		indexParentRB := descUID + "-" + rb.Spec.ParentRB
		if rbMerger.fieldsRBOfOneParentRB[indexParentRB] == nil {
			rbMerger.fieldsRBOfOneParentRB[indexParentRB] = &FieldsRBs{}
			rbMerger.fieldsRBOfOneParentRB[indexParentRB].countCls = len(clusters.Items)
		}
		rbMerger.fieldsRBOfOneParentRB[indexParentRB].NamesOfFieldRBs = append(rbMerger.fieldsRBOfOneParentRB[indexParentRB].NamesOfFieldRBs, indexFieldRB)
		rbMerger.fieldsRBOfOneParentRB[indexParentRB].rbsOfFields = append(rbMerger.fieldsRBOfOneParentRB[indexParentRB].rbsOfFields, rbMerger.clustersRBsOfOneFieldRB[indexFieldRB])
		if !utils.ContainsString(rbMerger.parentRBsOfDescUID[UID(descUID)], rb.Spec.ParentRB) {
			rbMerger.parentRBsOfDescUID[UID(descUID)] = append(rbMerger.parentRBsOfDescUID[UID(descUID)], indexParentRB)
		}

		if rbMerger.mergeResourceBinding(rb.Spec.ParentRB, indexParentRB, rbMerger.fieldsRBOfOneParentRB, chanResult, rb) {
			err := rbMerger.deleteRBsCollected(rbMerger.fieldsRBOfOneParentRB[indexParentRB].NamesOfFieldRBs, descUID)
			if err != nil {
				klog.Infof("Successful merged RBs of parent resource binding %q, but failed to delete RBs locally.", rb.Spec.ParentRB)
				return err
			}
			if rbMerger.canDeleteDescUID(descUID, rb.Spec.TotalPeer) {
				rbMerger.deleteGlobalDescUID(descUID)
				go func(labels map[string]string) {
					rbMerger.postMergedRBs(labels)
				}(rbLabels)
			}
		}
	}
	return nil
}

func (rbMerger *RBMerger) mergeResourceBinding(parentRBName, indexParentRB string, fieldsRBsOfParentRB map[string]*FieldsRBs, chanResult chan []*appV1alpha1.ResourceBindingApps, rb *appV1alpha1.ResourceBinding) bool {
	var allChildren [][]*appV1alpha1.ResourceBindingApps
	if fieldsRbs, ok := fieldsRBsOfParentRB[indexParentRB]; ok {
		if fieldsRbs.countCls == len(fieldsRbs.rbsOfFields) {
			for _, filedRBs := range fieldsRbs.rbsOfFields {
				allChildren = append(allChildren, filedRBs.rbsOfParentRB)
			}
			chanResult = cartesian.Iter(allChildren...)

			// deploy the Merged ResourceBinding
			rbMerger.getMergedResourceBindings(chanResult, parentRBName, rb)
			return true
		}
	}
	return false
}

func (rbMerger *RBMerger) getMergedResourceBindings(chanResult chan []*appV1alpha1.ResourceBindingApps, parentRBName string, rb *appV1alpha1.ResourceBinding) {
	// deploy the Merged ResourceBinding
	descName := rb.GetLabels()[common.OriginDescriptionNameLabel]
	desc, err := rbMerger.localGaiaClient.AppsV1alpha1().Descriptions(common.GaiaReservedNamespace).Get(context.TODO(), descName, metaV1.GetOptions{})
	if err != nil {
		klog.Errorf("failed to get Description %s of ResourceBing %s in Merging ResourceBindings.", descName, klog.KObj(rb))
		return
	}
	index := 0
	for rbN := range chanResult {
		// create new result ResourceBinding
		newResultRB := &appV1alpha1.ResourceBinding{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%d", parentRBName, index),
				Namespace: common.GaiaRBMergedReservedNamespace,
				Labels: map[string]string{
					common.StatusScheduler:                 string(appV1alpha1.ResourceBindingmerged),
					common.GaiaDescriptionLabel:            desc.Name,
					common.OriginDescriptionNameLabel:      desc.Name,
					common.OriginDescriptionNamespaceLabel: desc.Namespace,
					common.OriginDescriptionUIDLabel:       string(desc.UID),
				},
			},
			Spec: appV1alpha1.ResourceBindingSpec{
				AppID:           descName,
				TotalPeer:       rb.Spec.TotalPeer,
				ParentRB:        rb.Spec.ParentRB,
				RbApps:          rbN,
				NetworkPath:     rb.Spec.NetworkPath,
				StatusScheduler: appV1alpha1.ResourceBindingmerged,
			},
		}
		newResultRB.Kind = "ResourceBinding"
		newResultRB.APIVersion = "apps.gaia.io/v1alpha1"

		_, err := rbMerger.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRBMergedReservedNamespace).Create(context.TODO(), newResultRB, metaV1.CreateOptions{})
		if err != nil {
			klog.InfoS("ResourceBinding of %q merge success, but not created success %q.", parentRBName, common.GaiaRSToBeMergedReservedNamespace, err)
		} else {
			klog.Infof("ResourceBinding %q successfully merged and %q created.", parentRBName, newResultRB.Name)
		}

		index += 1
		// limit amount
		if index > 1 {
			break
		}
	}
}

func (rbMerger *RBMerger) canCreateCollectedRBs(rb *appV1alpha1.ResourceBinding, rbLabels map[string]string, indexParentRB string) bool {
	uid := rbLabels[common.OriginDescriptionUIDLabel]
	totalPeer, err := strconv.Atoi(rbLabels[common.TotalPeerOfParentRB])
	if err != nil {
		klog.V(5).Infof("Failed to get totalPeer from label.")
		totalPeer = 0
	}
	if rb.Spec.TotalPeer != 0 && totalPeer == len(rbMerger.parentRBsOfDescUID[UID(uid)]) && len(rbMerger.clustersRBsOfOneFieldRB[indexParentRB].rbsOfParentRB) == rb.Spec.TotalPeer {
		for _, InxParentRB := range rbMerger.parentRBsOfDescUID[UID(uid)] {
			if len(rbMerger.clustersRBsOfOneFieldRB[InxParentRB].rbNames) != rbMerger.clustersRBsOfOneFieldRB[InxParentRB].countRB {
				return false
			} else {
				continue
			}
		}
		return true
	}
	return false
}

func (rbMerger *RBMerger) createCollectedRBs(rb *appV1alpha1.ResourceBinding, rbLabels map[string]string) bool {

	descName := rbLabels[common.OriginDescriptionNameLabel]
	uid := rbLabels[common.OriginDescriptionUIDLabel]
	// field: all rb collected by InxParentRB
	for _, InxParentRB := range rbMerger.parentRBsOfDescUID[UID(uid)] {
		var rbApps []*appV1alpha1.ResourceBindingApps
		for _, rbAppChild := range rbMerger.clustersRBsOfOneFieldRB[InxParentRB].rbsOfParentRB {
			rbApp := &appV1alpha1.ResourceBindingApps{
				// ClusterName: rbMerger.clustersRBsOfOneFieldRB[InxParentRB].rbNames[index],
				Children: []*appV1alpha1.ResourceBindingApps{rbAppChild},
			}
			rbApps = append(rbApps, rbApp)
		}
		totalPeer, err := strconv.Atoi(rbLabels[common.TotalPeerOfParentRB])
		if err != nil {
			klog.V(5).Infof("Failed to get totalPeer from label.")
			totalPeer = 0
		}
		delete(rbLabels, common.TotalPeerOfParentRB)
		parenRB := InxParentRB[len(uid)+1:]
		// create new result ResourceBinding in parent cluster
		newResultRB := &appV1alpha1.ResourceBinding{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s", parenRB, rbMerger.selfClusterName),
				Namespace: common.GaiaRSToBeMergedReservedNamespace,
				Labels:    rbLabels,
			},
			Spec: appV1alpha1.ResourceBindingSpec{
				AppID:           descName,
				TotalPeer:       totalPeer,
				ParentRB:        parenRB,
				RbApps:          rbApps,
				NetworkPath:     rb.Spec.NetworkPath,
				StatusScheduler: appV1alpha1.ResourceBindingMerging,
			},
		}
		newResultRB.Kind = "ResourceBinding"
		newResultRB.APIVersion = "apps.gaia.io/v1alpha1"
		_, err = rbMerger.parentGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRSToBeMergedReservedNamespace).Create(context.TODO(), newResultRB, metaV1.CreateOptions{})
		if err != nil && !apiErrors.IsAlreadyExists(err) {
			klog.Infof("Failed to create ResourceBinding %q/%q to parent cluster. ERROR: %v", newResultRB.Namespace, newResultRB.Name, err)
			return false
		} else {
			klog.Infof("successfully created ResourceBinding %q/%q to parent cluster", newResultRB.Namespace, newResultRB.Name)
		}
	}
	return true
}

func (rbMerger *RBMerger) deleteRBsCollected(rbNames []string, uid string) error {
	for _, InxRB := range rbNames {
		rbName := InxRB[len(uid)+1:]
		err := rbMerger.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRSToBeMergedReservedNamespace).Delete(context.TODO(), rbName, metaV1.DeleteOptions{})
		if err != nil {
			klog.Infof("failed to delete rb %q/%q namespace, ERROR: %v", common.GaiaRSToBeMergedReservedNamespace, rbName, err)
			return err
		}
	}
	return nil
}

func (rbMerger *RBMerger) reCreateRBtoParent(rb *appV1alpha1.ResourceBinding) error {
	newRB := &appV1alpha1.ResourceBinding{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      rb.Name,
			Namespace: common.GaiaRSToBeMergedReservedNamespace,
			Labels:    rb.Labels,
		},
		Spec: appV1alpha1.ResourceBindingSpec{
			AppID:           rb.GetLabels()[common.OriginDescriptionNameLabel],
			TotalPeer:       rb.Spec.TotalPeer,
			ParentRB:        rb.Spec.ParentRB,
			RbApps:          rb.Spec.RbApps,
			NetworkPath:     rb.Spec.NetworkPath,
			StatusScheduler: rb.Spec.StatusScheduler,
		},
	}
	rb.Kind = "ResourceBinding"
	rb.APIVersion = "apps.gaia.io/v1alpha1"

	_, err := rbMerger.parentGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRSToBeMergedReservedNamespace).Create(context.TODO(), newRB, metaV1.CreateOptions{})

	return err
}

func (rbMerger *RBMerger) postMergedRBs(rbLabels map[string]string) {
	// TODO: && 不能连接 ==> return
	if rbMerger.postURL == "" {
		klog.Infof("postMergedRBs: postURL is nil.")
		return
	}

	var rbList []appV1alpha1.ResourceBinding
	var err error
	descName := rbLabels[common.OriginDescriptionNameLabel]
	descNS := rbLabels[common.OriginDescriptionNamespaceLabel]
	desc, err := rbMerger.localGaiaClient.AppsV1alpha1().Descriptions(common.GaiaReservedNamespace).Get(context.TODO(), descName, metaV1.GetOptions{})
	if err != nil {
		utilRuntime.HandleError(fmt.Errorf("postMergedRBs: failed to get description %q/%q, ERROR: %v", descNS, descName, err))
		return
	}
	rbs, err := rbMerger.rbLister.ResourceBindings(common.GaiaRBMergedReservedNamespace).List(labels.SelectorFromSet(labels.Set{
		common.OriginDescriptionNamespaceLabel: rbLabels[common.OriginDescriptionNamespaceLabel],
		common.OriginDescriptionUIDLabel:       rbLabels[common.OriginDescriptionUIDLabel],
		common.StatusScheduler:                 string(appV1alpha1.ResourceBindingmerged),
	}))
	if err != nil {
		utilRuntime.HandleError(fmt.Errorf("postMergedRBs: failed to get ResourceBinding of %q/%q, ERROR: %v", descNS, descName, err))
		return
	}
	for _, rb := range rbs {
		rbList = append(rbList, *rb)
	}
	resultSchemaSet := &resourcebindingmerger.SchemaSet{
		AppID:  descName,
		RBList: rbList,
		Desc:   *desc,
	}
	postBody, err := json.Marshal(resultSchemaSet)
	if err != nil {
		utilRuntime.HandleError(fmt.Errorf("postMergedRBs: failed to marshal resultSchemaSet to json, Description: %q/%q ERROR: %v", descNS, descName, err))
		return
	}
	klog.Infof("PostHyperOM: Begin to post description %q to HyperOM.", descName)
	fmt.Println(string(postBody))

	res, err := http.Post(rbMerger.postURL, "application/json", bytes.NewReader(postBody))
	if err != nil {
		utilRuntime.HandleError(fmt.Errorf("postHyperOM: post to HyperOM error, Description: %q/%q ERROR: %v", descNS, descName, err))
		return
	}
	content, errRd := io.ReadAll(res.Body)
	defer func() { _ = res.Body.Close() }()
	if errRd != nil {
		utilRuntime.HandleError(fmt.Errorf("ERROR: PostHyperOM: read response error, Description: %q/%q ERROR: %v", descNS, descName, errRd))
		return
	}
	klog.Infof("PostHyperOM: post the ResourceBindings of desc %q to HyperOM, Response: %s", descName, content)
}

func (rbMerger *RBMerger) deleteRB(rb *appV1alpha1.ResourceBinding) error {
	err := rbMerger.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRSToBeMergedReservedNamespace).Delete(context.TODO(), rb.Name, metaV1.DeleteOptions{})
	if err != nil {
		klog.Infof("Already upload, Resource Binding %q failed to delete. error: ", rb.Name, err)
	}
	klog.Infof("Already upload, Resource Binding %q be deleted successfully.", rb.Name)
	return err
}

func (rbMerger *RBMerger) canDeleteDescUID(uid string, totalPeer int) bool {
	if len(rbMerger.parentRBsOfDescUID[UID(uid)]) == totalPeer {
		for _, indexParentRB := range rbMerger.parentRBsOfDescUID[UID(uid)] {
			if len(rbMerger.fieldsRBOfOneParentRB[indexParentRB].rbsOfFields) != rbMerger.fieldsRBOfOneParentRB[indexParentRB].countCls {
				return false
			}
		}
		return true
	}
	return false
}

func (rbMerger *RBMerger) deleteGlobalDescUID(uid string) {
	for _, indexParentRB := range rbMerger.parentRBsOfDescUID[UID(uid)] {
		for _, rbName := range rbMerger.fieldsRBOfOneParentRB[indexParentRB].NamesOfFieldRBs {
			delete(rbMerger.clustersRBsOfOneFieldRB, rbName)
		}
		delete(rbMerger.fieldsRBOfOneParentRB, indexParentRB)
	}
	delete(rbMerger.parentRBsOfDescUID, UID(uid))
}

func (rbMerger *RBMerger) deleteFieldDescUID(uid UID) {
	for _, InxParentRB := range rbMerger.parentRBsOfDescUID[uid] {
		delete(rbMerger.clustersRBsOfOneFieldRB, InxParentRB)
	}
	delete(rbMerger.parentRBsOfDescUID, uid)
}

func (rbMerger *RBMerger) UpdateSelectedResourceBinding(rb *appV1alpha1.ResourceBinding) error {
	if !utils.ContainsString(rb.Finalizers, common.AppFinalizer) && rb.DeletionTimestamp == nil {
		rb.Finalizers = append(rb.Finalizers, common.AppFinalizer)
		rb.GetLabels()[common.StatusScheduler] = string(appV1alpha1.ResourceBindingSelected)
		if _, err := rbMerger.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRBMergedReservedNamespace).Update(context.TODO(), rb, metaV1.UpdateOptions{}); err != nil {
			msg := fmt.Sprintf("failed to inject  finalizer %s and select ResouceBinding %s: %v", common.AppFinalizer, klog.KObj(rb), err)
			klog.WarningDepth(4, msg)
			return err
		}
		rbLabels := rb.GetLabels()
		descName := rbLabels[common.OriginDescriptionNamespaceLabel]
		klog.V(4).Infof("Delete unselected ResourceBindings in namespace %q from Description %q.", common.GaiaRBMergedReservedNamespace, descName)
		err := rbMerger.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRBMergedReservedNamespace).
			DeleteCollection(context.TODO(), metaV1.DeleteOptions{}, metaV1.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{
				common.OriginDescriptionNameLabel:      descName,
				common.OriginDescriptionNamespaceLabel: rbLabels[common.OriginDescriptionNamespaceLabel],
				common.OriginDescriptionUIDLabel:       rbLabels[common.OriginDescriptionUIDLabel],
				common.StatusScheduler:                 string(appV1alpha1.ResourceBindingmerged),
			}).String()})
		if err != nil {
			klog.Infof("failed to delete rbs in %q namespace, ERROR: %v", common.GaiaRBMergedReservedNamespace, err)
			return err
		}

	}
	return nil
}
