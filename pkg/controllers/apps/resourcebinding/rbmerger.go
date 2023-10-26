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
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"

	appV1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
	"github.com/lmxia/gaia/pkg/controllers/resourcebindingmerger"
	gaiaClientSet "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	gaiaInformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	"github.com/lmxia/gaia/pkg/utils"
	"github.com/lmxia/gaia/pkg/utils/cartesian"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilRuntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type UID string

// RBMerger defines configuration for ResourceBindings Merger
type RBMerger struct {
	rbToLocalController  *resourcebindingmerger.Controller
	rbTOParentController *resourcebindingmerger.Controller

	localKubeClient                 *kubernetes.Clientset
	localGaiaClient                 *gaiaClientSet.Clientset
	localToMergeGaiaInformerFactory gaiaInformers.SharedInformerFactory
	toMergeRBSynced                 cache.InformerSynced

	selfClusterName  string
	parentNamespace  string
	parentGaiaClient *gaiaClientSet.Clientset

	mu                      sync.Mutex
	clustersRBsOfOneFieldRB map[string]*ClustersRBs // map[descUID-parentRBName]*ClustersRBs
	fieldsRBOfOneParentRB   map[string]*FieldsRBs   // map[descUID-parentRBName]*FieldsRBs
	parentRBsOfDescUID      map[UID][]string        // map[descUID][descUID-parentRBName...]
	descUID                 map[string]bool
	postURL                 string
}

// ClustersRBs contains all cluster RB from field's mCls in a parentRB
type ClustersRBs struct {
	sync.Mutex
	countRB       int
	rbNames       []string
	rbsOfParentRB []*appV1alpha1.ResourceBindingApps
}

// FieldsRBs contains all RB from mCls in a parentRB
type FieldsRBs struct {
	sync.Mutex
	countCls        int
	NamesOfFieldRBs []string
	rbsOfFields     []*ClustersRBs
}

// NewRBMerger returns a new RBMerger for ResourceBinding.
func NewRBMerger(kubeClient *kubernetes.Clientset, gaiaClient *gaiaClientSet.Clientset) (*RBMerger, error) {
	postUrl := os.Getenv(common.ResourceBindingMergerPostURL)
	localToGaiaInformerFactory := gaiaInformers.NewSharedInformerFactoryWithOptions(gaiaClient, common.DefaultResync, gaiaInformers.WithNamespace(common.GaiaRSToBeMergedReservedNamespace))
	rbMerger := &RBMerger{
		localKubeClient:                 kubeClient,
		localGaiaClient:                 gaiaClient,
		localToMergeGaiaInformerFactory: localToGaiaInformerFactory,
		toMergeRBSynced:                 localToGaiaInformerFactory.Apps().V1alpha1().ResourceBindings().Informer().HasSynced,
		clustersRBsOfOneFieldRB:         make(map[string]*ClustersRBs),
		fieldsRBOfOneParentRB:           make(map[string]*FieldsRBs),
		parentRBsOfDescUID:              make(map[UID][]string),
		descUID:                         make(map[string]bool),
		postURL:                         postUrl,
	}

	rbLocalController, err := resourcebindingmerger.NewController(gaiaClient,
		localToGaiaInformerFactory.Apps().V1alpha1().ResourceBindings(),
		rbMerger.handleToLocalResourceBinding)
	if err != nil {
		return nil, err
	}
	rbMerger.rbToLocalController = rbLocalController

	return rbMerger, nil
}

func (m *RBMerger) RunToLocalResourceBindingMerger(workers int, stopCh <-chan struct{}) {
	klog.Info("Starting local ResourceBinding Merger ...")
	defer klog.Info("Shutting local ResourceBinding Merger ...")

	m.localToMergeGaiaInformerFactory.Start(stopCh)
	if !cache.WaitForNamedCacheSync("to-local-resourcebinding-merger", stopCh, m.toMergeRBSynced) {
		return
	}
	m.rbToLocalController.Run(workers, stopCh)
	return
}

func (m *RBMerger) RunToParentResourceBindingMerger(workers int, stopCh <-chan struct{}) {
	klog.Info("Starting parent ResourceBinding Merger ...")
	defer klog.Info("Shutting parent ResourceBinding Merger ...")

	m.localToMergeGaiaInformerFactory.Start(stopCh)
	if !cache.WaitForNamedCacheSync("to-parent-resourcebinding-merger", stopCh, m.toMergeRBSynced) {
		return
	}
	m.rbTOParentController.Run(workers, stopCh)
	return
}

func (m *RBMerger) SetParentRBController() (*RBMerger, error) {
	parentGaiaClient, _, _ := utils.SetParentClient(m.localKubeClient, m.localGaiaClient)
	selfClusterName, parentNamespace, errClusterName := utils.GetLocalClusterName(m.localKubeClient)
	if errClusterName != nil {
		klog.Errorf("local handleResourceBinding failed to get clusterName From secret: %v", errClusterName)
		return nil, errClusterName
	}
	m.parentGaiaClient = parentGaiaClient
	m.selfClusterName = selfClusterName
	m.parentNamespace = parentNamespace

	rbTOParentController, err := resourcebindingmerger.NewController(m.localGaiaClient, m.localToMergeGaiaInformerFactory.Apps().V1alpha1().ResourceBindings(), m.handleToParentResourceBinding)
	if err != nil {
		klog.Errorf("failed to create rbMerger to parent, err==%v", err)
		return nil, err
	}
	m.rbTOParentController = rbTOParentController
	return m, nil
}

func (m *RBMerger) handleToParentResourceBinding(rb *appV1alpha1.ResourceBinding) error {
	klog.V(5).Infof("handle local resourceBinding %s", klog.KObj(rb))
	clusters, err := m.localGaiaClient.PlatformV1alpha1().ManagedClusters(coreV1.NamespaceAll).List(context.TODO(), metaV1.ListOptions{})
	if err != nil {
		klog.Warningf("failed to list managed clusters: %v", err)
	}
	if rb.Spec.StatusScheduler != "" {
		klog.V(4).Infof("ResourceBinding %s has already been processed with Result %q. Skip it.", klog.KObj(rb), rb.Status.Status)
		return nil
	}

	// level cluster
	if len(clusters.Items) == 0 {
		m.reCreateRBtoParent(context.TODO(), rb)
		err = m.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRSToBeMergedReservedNamespace).Delete(context.TODO(), rb.Name, metaV1.DeleteOptions{})
		if err != nil {
			klog.Errorf("failed to delete local ResourceBinding %q. error: %v", klog.KObj(rb).String(), err)
			return err
		}
		return nil
	} else if m.parentGaiaClient != nil { // level field
		m.mu.Lock()
		defer m.mu.Unlock()
		rbLabels := rb.GetLabels()
		uid := rbLabels[common.OriginDescriptionUIDLabel]
		indexParentRB := uid + "-" + rb.Spec.ParentRB

		if !utils.ContainsString(m.parentRBsOfDescUID[UID(uid)], indexParentRB) {
			m.parentRBsOfDescUID[UID(uid)] = append(m.parentRBsOfDescUID[UID(uid)], indexParentRB)
		}
		if m.clustersRBsOfOneFieldRB[indexParentRB] == nil {
			m.clustersRBsOfOneFieldRB[indexParentRB] = &ClustersRBs{}
		}
		for _, value := range rb.Spec.RbApps {
			if value.Children != nil {
				m.clustersRBsOfOneFieldRB[indexParentRB].Lock()
				m.clustersRBsOfOneFieldRB[indexParentRB].countRB = rb.Spec.TotalPeer
				m.clustersRBsOfOneFieldRB[indexParentRB].rbNames = append(m.clustersRBsOfOneFieldRB[indexParentRB].rbNames, rb.Name)
				m.clustersRBsOfOneFieldRB[indexParentRB].rbsOfParentRB = append(m.clustersRBsOfOneFieldRB[indexParentRB].rbsOfParentRB, value)
				m.clustersRBsOfOneFieldRB[indexParentRB].Unlock()
			}
		}

		v, exist := m.descUID[uid]
		if m.canCreateCollectedRBs(rb, rbLabels, indexParentRB) && v != true {
			if m.createCollectedRBs(context.TODO(), rb, rbLabels) {
				m.deleteFieldDescUID(UID(uid))
				m.descUID[uid] = true

				err = m.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRSToBeMergedReservedNamespace).
					DeleteCollection(context.TODO(), metaV1.DeleteOptions{}, metaV1.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{
						common.OriginDescriptionNameLabel:      rbLabels[common.OriginDescriptionNameLabel],
						common.OriginDescriptionNamespaceLabel: rbLabels[common.OriginDescriptionNamespaceLabel],
						common.OriginDescriptionUIDLabel:       rbLabels[common.OriginDescriptionUIDLabel],
					}).String()})
				if err != nil {
					klog.Infof("failed to delete rbs in %s namespace", common.GaiaRSToBeMergedReservedNamespace, err)
					return err
				}
			}
		} else {
			if !exist {
				m.descUID[uid] = false
			} else if v == true {
				_ = m.deleteRB(rb)
				m.deleteFieldDescUID(UID(uid))
			}
		}
	} else { // global
		return nil
	}
	return nil
}

// handleToLocalResourceBinding handles gaia-to-be-merged namespace rbs
func (m *RBMerger) handleToLocalResourceBinding(rb *appV1alpha1.ResourceBinding) error {
	klog.V(5).Infof("handleToLocalResourceBinding: handle local resourceBinding %s", klog.KObj(rb))
	if rb.Spec.StatusScheduler != appV1alpha1.ResourceBindingMerging {
		return nil
	}

	rbLabels := rb.GetLabels()
	clusters, err := m.localGaiaClient.PlatformV1alpha1().ManagedClusters(coreV1.NamespaceAll).List(context.TODO(), metaV1.ListOptions{})
	if err != nil {
		klog.Warningf("handleToLocalResourceBinding: failed to list managed clusters: %v", err)
	}
	descUID := rbLabels[common.OriginDescriptionUIDLabel]
	descName := rbLabels[common.OriginDescriptionNameLabel]
	chanResult := make(chan []*appV1alpha1.ResourceBindingApps)
	m.mu.Lock()
	defer m.mu.Unlock()

	indexFieldRB := descUID + "-" + rb.Name
	indexParentRB := descUID + "-" + rb.Spec.ParentRB
	// clustersRBsOfOneFieldRB: map[descUID-RBName]*ClustersRBs
	if m.clustersRBsOfOneFieldRB[indexFieldRB] == nil {
		m.clustersRBsOfOneFieldRB[indexFieldRB] = &ClustersRBs{}

		// add debug log for map content
		klog.V(5).Infof(fmt.Sprintf("m.parentRBsOfDescUID[%s]:\n   %+v\n", descUID, m.parentRBsOfDescUID[UID(descUID)]))
		for _, indexParentRB := range m.parentRBsOfDescUID[UID(descUID)] {
			klog.V(5).Infof(fmt.Sprintf("m.fieldsRBOfOneParentRB[%s]:\n   %+v\n", indexParentRB, m.fieldsRBOfOneParentRB[indexParentRB]))
		}
		// m.clustersRBsOfOneFieldRB[indexFieldRB].Lock()
		for _, rbApp := range rb.Spec.RbApps {
			m.clustersRBsOfOneFieldRB[indexFieldRB].rbsOfParentRB = append(m.clustersRBsOfOneFieldRB[indexFieldRB].rbsOfParentRB, rbApp.Children[0])
		}
		// m.clustersRBsOfOneFieldRB[indexFieldRB].Unlock()

		// process fieldsRBOfOneParentRB: map[descUID-parentRBName]*FieldsRBs
		if m.fieldsRBOfOneParentRB[indexParentRB] == nil {
			m.fieldsRBOfOneParentRB[indexParentRB] = &FieldsRBs{}
			m.fieldsRBOfOneParentRB[indexParentRB].countCls = len(clusters.Items)
		}
		m.fieldsRBOfOneParentRB[indexParentRB].Lock()
		m.fieldsRBOfOneParentRB[indexParentRB].NamesOfFieldRBs = append(m.fieldsRBOfOneParentRB[indexParentRB].NamesOfFieldRBs, indexFieldRB)
		m.fieldsRBOfOneParentRB[indexParentRB].rbsOfFields = append(m.fieldsRBOfOneParentRB[indexParentRB].rbsOfFields, m.clustersRBsOfOneFieldRB[indexFieldRB])
		m.fieldsRBOfOneParentRB[indexParentRB].Unlock()
		// add debug log for map content
		klog.V(5).Infof(fmt.Sprintf("after added: m.parentRBsOfDescUID[%s]:\n   %+v\n", descUID, m.parentRBsOfDescUID[UID(descUID)]))
		for _, indexParentRB := range m.parentRBsOfDescUID[UID(descUID)] {
			klog.V(5).Infof(fmt.Sprintf("after added: m.fieldsRBOfOneParentRB[%s]:\n   %+v\n", indexParentRB, m.fieldsRBOfOneParentRB[indexParentRB]))
		}

		// process parentRBsOfDescUID: map[descUID][descUID-parentRBName...]
		if !utils.ContainsString(m.parentRBsOfDescUID[UID(descUID)], indexParentRB) {
			m.parentRBsOfDescUID[UID(descUID)] = append(m.parentRBsOfDescUID[UID(descUID)], indexParentRB)
		}
		m.mergeResourceBinding(rb.Spec.ParentRB, indexParentRB, m.fieldsRBOfOneParentRB, chanResult, rb)
	} else {
		klog.InfoS("handleToLocalResourceBinding: already handled", "ResourceBinding", klog.KObj(rb).String())
	}

	if m.canDeleteDescUID(descUID, rb.Spec.TotalPeer) {
		klog.V(5).Infof("handleToLocalResourceBinding: begin to post description(%q) and resourceBindings to HyperOM, target Server: %q", descName, m.postURL)
		postErr := m.postMergedRBs(descName)
		if postErr != nil {
			return postErr
		}

		m.deleteGlobalDescUID(descUID)
		err = m.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRSToBeMergedReservedNamespace).
			DeleteCollection(context.TODO(), metaV1.DeleteOptions{}, metaV1.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{
				common.OriginDescriptionNameLabel:      descName,
				common.OriginDescriptionNamespaceLabel: common.GaiaReservedNamespace,
				common.OriginDescriptionUIDLabel:       descUID,
			}).String()})
		if err != nil {
			klog.InfoS("failed to delete gaia-to-be-merged rbs", "Description", klog.KRef(common.GaiaReservedNamespace, descName), err)
			return err
		}
	}
	return nil
}

func (m *RBMerger) mergeResourceBinding(parentRBName, indexParentRB string, fieldsRBsOfParentRB map[string]*FieldsRBs, chanResult chan []*appV1alpha1.ResourceBindingApps, rb *appV1alpha1.ResourceBinding) {
	var allChildren [][]*appV1alpha1.ResourceBindingApps
	if fieldsRbs, ok := fieldsRBsOfParentRB[indexParentRB]; ok {
		if fieldsRbs.countCls == len(fieldsRbs.rbsOfFields) {
			for _, filedRBs := range fieldsRbs.rbsOfFields {
				allChildren = append(allChildren, filedRBs.rbsOfParentRB)
			}
			chanResult = cartesian.Iter(allChildren...)

			// deploy the Merged ResourceBinding
			m.getMergedResourceBindings(chanResult, parentRBName, rb)
			// return true
		}
	}
	// return false
}

func (m *RBMerger) getMergedResourceBindings(chanResult chan []*appV1alpha1.ResourceBindingApps, parentRBName string, rb *appV1alpha1.ResourceBinding) {
	// deploy the Merged ResourceBinding
	descName := rb.GetLabels()[common.OriginDescriptionNameLabel]
	desc, err := m.localGaiaClient.AppsV1alpha1().Descriptions(common.GaiaReservedNamespace).Get(context.TODO(), descName, metaV1.GetOptions{})
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
				AppID: descName,
				// TotalPeer:       rb.Spec.TotalPeer,
				ParentRB:        rb.Spec.ParentRB,
				RbApps:          rbN,
				NetworkPath:     rb.Spec.NetworkPath,
				StatusScheduler: appV1alpha1.ResourceBindingmerged,
			},
		}
		newResultRB.Kind = "ResourceBinding"
		newResultRB.APIVersion = "apps.gaia.io/v1alpha1"

		_, err := m.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRBMergedReservedNamespace).Create(context.TODO(), newResultRB, metaV1.CreateOptions{})
		if err != nil {
			klog.InfoS("ResourceBinding merged success, but failed to create", "ResourceBinding", parentRBName, err)
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

func (m *RBMerger) canCreateCollectedRBs(rb *appV1alpha1.ResourceBinding, rbLabels map[string]string, indexParentRB string) bool {
	uid := rbLabels[common.OriginDescriptionUIDLabel]
	totalPeer, err := strconv.Atoi(rbLabels[common.TotalPeerOfParentRB])
	if err != nil {
		klog.V(5).Infof("Failed to get totalPeer from label.")
		totalPeer = 0
	}
	if rb.Spec.TotalPeer != 0 && totalPeer == len(m.parentRBsOfDescUID[UID(uid)]) && len(m.clustersRBsOfOneFieldRB[indexParentRB].rbsOfParentRB) == rb.Spec.TotalPeer {
		for _, InxParentRB := range m.parentRBsOfDescUID[UID(uid)] {
			if len(m.clustersRBsOfOneFieldRB[InxParentRB].rbNames) != m.clustersRBsOfOneFieldRB[InxParentRB].countRB {
				return false
			} else {
				continue
			}
		}
		return true
	}
	return false
}

func (m *RBMerger) createCollectedRBs(ctx context.Context, rb *appV1alpha1.ResourceBinding, rbLabels map[string]string) bool {
	wg := sync.WaitGroup{}
	descName := rbLabels[common.OriginDescriptionNameLabel]
	uid := rbLabels[common.OriginDescriptionUIDLabel]
	totalPeer, err := strconv.Atoi(rbLabels[common.TotalPeerOfParentRB])
	if err != nil {
		klog.V(5).Infof("Failed to get totalPeer from label.")
		totalPeer = 0
	}
	delete(rbLabels, common.TotalPeerOfParentRB)
	// field: all rb collected by InxParentRB
	for _, InxParentRB := range m.parentRBsOfDescUID[UID(uid)] {
		var rbApps []*appV1alpha1.ResourceBindingApps
		for _, rbAppChild := range m.clustersRBsOfOneFieldRB[InxParentRB].rbsOfParentRB {
			rbApp := &appV1alpha1.ResourceBindingApps{
				// ClusterName: m.clustersRBsOfOneFieldRB[InxParentRB].rbNames[index],
				Children: []*appV1alpha1.ResourceBindingApps{rbAppChild},
			}
			rbApps = append(rbApps, rbApp)
		}
		parenRB := InxParentRB[len(uid)+1:]
		// create new result ResourceBinding in parent cluster
		newResultRB := &appV1alpha1.ResourceBinding{
			ObjectMeta: metaV1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s", parenRB, m.selfClusterName),
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
		wg.Add(1)
		go func(rb *appV1alpha1.ResourceBinding) {
			defer wg.Done()
			utils.CreateRBtoParentWithRetry(ctx, m.parentGaiaClient, common.GaiaRSToBeMergedReservedNamespace, newResultRB)
		}(newResultRB)
	}
	wg.Wait()
	return true
}

func (m *RBMerger) deleteRBsCollected(rbNames []string, uid string) error {
	for _, InxRB := range rbNames {
		rbName := InxRB[len(uid)+1:]
		err := m.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRSToBeMergedReservedNamespace).Delete(context.TODO(), rbName, metaV1.DeleteOptions{})
		if err != nil {
			klog.Error(fmt.Errorf("failed to delete rb %q, ERROR: %v", klog.KRef(common.GaiaRSToBeMergedReservedNamespace, rbName), err))
			return err
		}
	}
	return nil
}

func (m *RBMerger) reCreateRBtoParent(ctx context.Context, rb *appV1alpha1.ResourceBinding) {
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

	utils.CreateRBtoParentWithRetry(ctx, m.parentGaiaClient, common.GaiaRSToBeMergedReservedNamespace, newRB)
}

func (m *RBMerger) postMergedRBs(descName string) error {
	klog.Infof("PostHyperOM: handling with posting description %q to HyperOM, target Server: %q", descName, m.postURL)

	if m.postURL == "" {
		klog.Errorf("postMergedRBs: postURL is nil.")
		return nil
	}
	var rbList []appV1alpha1.ResourceBinding
	var err error
	desc, err := m.localGaiaClient.AppsV1alpha1().Descriptions(common.GaiaReservedNamespace).Get(context.TODO(), descName, metaV1.GetOptions{})
	if err != nil {
		return fmt.Errorf("postMergedRBs: failed to get description %q, ERROR: %v", klog.KRef(common.GaiaReservedNamespace, descName), err)
	}
	rbs, err := m.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRBMergedReservedNamespace).List(context.TODO(), metaV1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{
			common.OriginDescriptionNameLabel:      descName,
			common.OriginDescriptionNamespaceLabel: common.GaiaReservedNamespace,
			common.OriginDescriptionUIDLabel:       string(desc.GetUID()),
			common.StatusScheduler:                 string(appV1alpha1.ResourceBindingmerged),
		}).String(),
	})
	if err != nil {
		return fmt.Errorf("postMergedRBs: failed to get ResourceBindings of %q, ERROR: %v", klog.KRef(common.GaiaReservedNamespace, descName), err)
	}
	for _, rb := range rbs.Items {
		rbList = append(rbList, rb)
	}
	resultSchemaSet := &resourcebindingmerger.SchemaSet{
		AppID:  descName,
		RBList: rbList,
		Desc:   *desc,
	}
	postBody, err := json.Marshal(resultSchemaSet)
	if err != nil {
		return fmt.Errorf("postMergedRBs: failed to marshal resultSchemaSet, Description: %q ERROR: %v", klog.KRef(common.GaiaReservedNamespace, descName), err)
	}
	fmt.Printf("postMergedRBs: postBody:\n%s \n\n", string(postBody))

	request, err := http.NewRequest("POST", m.postURL, bytes.NewBuffer(postBody))
	if err != nil {
		return fmt.Errorf("postMergedRBs: post new request, error=%v \n", err)
	}
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("cache-control", "no-cache")
	resp, err := http.DefaultClient.Do(request)
	if resp != nil {
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				utilRuntime.HandleError(fmt.Errorf("postMergedRBs: failed to close response body, Description: %q ERROR: %v", klog.KRef(common.GaiaReservedNamespace, descName), err))
			}
		}(resp.Body)
	}
	if err != nil {
		return fmt.Errorf("postHyperOM: post to HyperOM error, Description: %q ERROR: %v", klog.KRef(common.GaiaReservedNamespace, descName), err)
	}
	content, errRd := io.ReadAll(resp.Body)
	if errRd != nil {
		return fmt.Errorf("ERROR: PostHyperOM: read response error, Description: %q ERROR: %v", klog.KRef(common.GaiaReservedNamespace, descName), errRd)
	}

	klog.Infof("PostHyperOM: post the ResourceBindings of desc %q to HyperOM, Response: %s", descName, content)

	return nil
}

func (m *RBMerger) deleteRB(rb *appV1alpha1.ResourceBinding) error {
	err := m.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRSToBeMergedReservedNamespace).Delete(context.TODO(), rb.Name, metaV1.DeleteOptions{})
	if err != nil {
		klog.Infof("Already upload, Resource Binding %q failed to delete. error: ", rb.Name, err)
	}
	klog.Infof("Already upload, Resource Binding %q be deleted successfully.", rb.Name)
	return err
}

func (m *RBMerger) canDeleteDescUID(uid string, totalPeer int) bool {
	if len(m.parentRBsOfDescUID[UID(uid)]) == totalPeer {
		klog.V(5).Infof(fmt.Sprintf("canDeleteDescUID: m.parentRBsOfDescUID[%s]:\n   %+v\n", uid, m.parentRBsOfDescUID[UID(uid)]))
		for _, indexParentRB := range m.parentRBsOfDescUID[UID(uid)] {
			klog.V(5).Infof(fmt.Sprintf("canDeleteDescUID: m.fieldsRBOfOneParentRB[%s]:\n   %+v\n", indexParentRB, m.fieldsRBOfOneParentRB[indexParentRB]))
		}

		for _, indexParentRB := range m.parentRBsOfDescUID[UID(uid)] {
			if len(m.fieldsRBOfOneParentRB[indexParentRB].rbsOfFields) != m.fieldsRBOfOneParentRB[indexParentRB].countCls {
				return false
			}
		}
		return true
	}
	return false
}

func (m *RBMerger) deleteGlobalDescUID(uid string) {
	for _, indexParentRB := range m.parentRBsOfDescUID[UID(uid)] {
		for _, rbName := range m.fieldsRBOfOneParentRB[indexParentRB].NamesOfFieldRBs {
			delete(m.clustersRBsOfOneFieldRB, rbName)
		}
		delete(m.fieldsRBOfOneParentRB, indexParentRB)
	}
	delete(m.parentRBsOfDescUID, UID(uid))
	klog.V(5).Infof(fmt.Sprintf("deleteGlobalDescUID: m.parentRBsOfDescUID[%s]:\n   %+v\n", uid, m.parentRBsOfDescUID[UID(uid)]))
}

func (m *RBMerger) deleteFieldDescUID(uid UID) {
	for _, InxParentRB := range m.parentRBsOfDescUID[uid] {
		delete(m.clustersRBsOfOneFieldRB, InxParentRB)
	}
	delete(m.parentRBsOfDescUID, uid)
}
