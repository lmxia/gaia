/*
Copyright 2021 The Clusternet Authors.

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

package rbmerger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	appv1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
	"github.com/lmxia/gaia/pkg/controllers/resourcebindingmerger"
	gaiaClientSet "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	gaiainformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	appsLister "github.com/lmxia/gaia/pkg/generated/listers/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/utils"
	"github.com/lmxia/gaia/pkg/utils/cartesian"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"strconv"
	"sync"
)

// RBMerger defines configuration for ResourceBindings approver
type RBMerger struct {
	// rbmController *resourcebindingmerger.Controller
	rbToLocalController  *resourcebindingmerger.Controller
	rbTOParentController *resourcebindingmerger.Controller

	localKubeClient          *kubernetes.Clientset
	localGaiaClient          *gaiaClientSet.Clientset
	loaclGaiaInformerFactory gaiainformers.SharedInformerFactory
	rbLister                 appsLister.ResourceBindingLister
	selfClusterName          string
	parentNameSpace          string
	parentGaiaClient         *gaiaClientSet.Clientset

	mu                  sync.Mutex
	rbsOfParentRB       map[string]*RBsOfParentRB
	fieldsRBsOfParentRB map[string]*FieldsRBs
	parentsRBsOfAPPid   map[string][]string
	descResourceID      map[string]bool
	postURL             string
}

// RBsOfParentRB contains all RB from mCls in a parentRB
type RBsOfParentRB struct {
	count         int
	rbNames       []string
	rbsOfParentRB []*appv1alpha1.ResourceBindingApps
}

// FieldsRBs contains all RB from mCls in a parentRB
type FieldsRBs struct {
	countCls        int
	NamesOfFiledRBs []string
	rbsOfFields     []*RBsOfParentRB
}

// NewRBMerger returns a new RBMerger for ResourceBinding.
func NewRBMerger(kubeclient *kubernetes.Clientset, gaiaclient *gaiaClientSet.Clientset,
		gaiaInformerFactory gaiainformers.SharedInformerFactory) (*RBMerger, error) {

	postUrl := os.Getenv(common.ResourceBindMergePostURL)
	rbMerger := &RBMerger{
		localKubeClient:          kubeclient,
		localGaiaClient:          gaiaclient,
		loaclGaiaInformerFactory: gaiaInformerFactory,
		rbLister:                 gaiaInformerFactory.Apps().V1alpha1().ResourceBindings().Lister(),
		rbsOfParentRB:            make(map[string]*RBsOfParentRB),
		fieldsRBsOfParentRB:      make(map[string]*FieldsRBs),
		parentsRBsOfAPPid:        make(map[string][]string),
		descResourceID:           make(map[string]bool, 0),
		postURL:                  postUrl,
	}

	rbLocalController, err := resourcebindingmerger.NewController(gaiaclient,
		gaiaInformerFactory.Apps().V1alpha1().ResourceBindings(),
		rbMerger.handleToLocalResourceBinding)
	if err != nil {
		return nil, err
	}
	rbMerger.rbToLocalController = rbLocalController

	return rbMerger, nil
}

func (rbMerger *RBMerger) RunToLocalResourceBindingMerger(threadiness int, stopCh <-chan struct{}) {
	klog.Info("Starting local ResourceBinding Merger ...")
	defer klog.Info("Shutting local ResourceBinding Merger ...")
	// todo: gorountine
	rbMerger.rbToLocalController.Run(threadiness, stopCh)
	return
}

func (rbMerger *RBMerger) RunToParentResourceBindingMerger(threadiness int, stopCh <-chan struct{}) {
	klog.Info("Starting parent ResourceBinding Merger ...")
	defer klog.Info("Shutting parent ResourceBinding Merger ...")
	// todo: gorountine
	rbMerger.rbTOParentController.Run(threadiness, stopCh)
	return
}

func (rbMerger *RBMerger) SetParentRBController() (*RBMerger, error) {
	parentGaiaClient, _, _ := utils.SetParentClient(rbMerger.localKubeClient, rbMerger.localGaiaClient)
	selfClusterName, parentNameSpace, errClusterName := utils.GetLocalClusterName(rbMerger.localKubeClient)
	if errClusterName != nil {
		klog.Errorf("local handleResourceBinding failed to get clustername From secret: %v", errClusterName)
		return nil, errClusterName
	}
	rbMerger.parentGaiaClient = parentGaiaClient
	rbMerger.selfClusterName = selfClusterName
	rbMerger.parentNameSpace = parentNameSpace

	rbTOParentController, err := resourcebindingmerger.NewController(rbMerger.localGaiaClient,
		rbMerger.loaclGaiaInformerFactory.Apps().V1alpha1().ResourceBindings(),
		rbMerger.handleToParentResourceBinding)
	if err != nil {
		return nil, err
	}
	rbMerger.rbTOParentController = rbTOParentController
	return rbMerger, nil
}

func (rbMerger *RBMerger) handleToParentResourceBinding(rb *appv1alpha1.ResourceBinding) error {
	klog.V(5).Infof("handle local resourceBinding %s", klog.KObj(rb))

	clusters, err := rbMerger.localGaiaClient.PlatformV1alpha1().ManagedClusters(corev1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Warningf("failed to list managed clusters: %v", err)
	}

	if rb.Namespace != common.GaiaRSToBeMergedReservedNamespace {
		klog.V(4).Infof("The ResourceBinding not in %q namespace.", common.GaiaRSToBeMergedReservedNamespace)
		return nil
	}
	if rb.Spec.StatusScheduler != "" {
		klog.V(4).Infof("ResourceBinding %q has already been processed with Result %q. Skip it.", klog.KObj(rb), rb.Status.Status)
		return nil
	}

	if len(clusters.Items) == 0 {

		err = rbMerger.reCreateRBtoParent(rb)
		if apierrors.IsAlreadyExists(err) || err == nil {
			klog.Infof("successfully create ResourceBinding %q to ParentCluster", rb.Name)
			err = rbMerger.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRSToBeMergedReservedNamespace).Delete(context.TODO(), rb.Name, metav1.DeleteOptions{})
			if err != nil {
				klog.V(4).Infof("Resource Binding %q failed to delete. error: ", rb.Name, err)
				return err
			}
		} else {
			klog.V(4).Infof("Failed to create ResourceBinding %q to parent.", rb.Name, err)
			return err
		}

		return nil

	} else if rbMerger.parentGaiaClient != nil {

		rbMerger.mu.Lock()
		descName := rb.GetLabels()[common.GaiaDescriptionLabel]
		if !utils.ContainsString(rbMerger.parentsRBsOfAPPid[descName], rb.Spec.ParentRB) {
			rbMerger.parentsRBsOfAPPid[descName] = append(rbMerger.parentsRBsOfAPPid[descName], rb.Spec.ParentRB)
		}
		if rbMerger.rbsOfParentRB[rb.Spec.ParentRB] == nil {
			rbMerger.rbsOfParentRB[rb.Spec.ParentRB] = &RBsOfParentRB{}
		}
		for _, value := range rb.Spec.RbApps {
			if value.Children != nil {
				rbMerger.rbsOfParentRB[rb.Spec.ParentRB].count = rb.Spec.TotalPeer
				rbMerger.rbsOfParentRB[rb.Spec.ParentRB].rbNames = append(rbMerger.rbsOfParentRB[rb.Spec.ParentRB].rbNames, rb.Name)
				rbMerger.rbsOfParentRB[rb.Spec.ParentRB].rbsOfParentRB = append(rbMerger.rbsOfParentRB[rb.Spec.ParentRB].rbsOfParentRB, value)
			}
		}

		desc, _ := rbMerger.parentGaiaClient.AppsV1alpha1().Descriptions(rbMerger.parentNameSpace).Get(context.TODO(), descName, metav1.GetOptions{})
		v, exist := rbMerger.descResourceID[string(desc.GetUID())]
		if rbMerger.canCreateCollectedRBs(rb) && v != true {
			if rbMerger.createCollectedRBs(rb) {
				rbMerger.deleteFieldAppIDKey(descName)
				rbMerger.descResourceID[string(desc.GetUID())] = true

				err = rbMerger.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRSToBeMergedReservedNamespace).
						DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{
							common.GaiaDescriptionLabel: descName,
						}).String()})
				if err != nil {
					klog.Infof("failed to delete rbs in %s namespace", common.GaiaRBMergedReservedNamespace, err)
					return err
				}
			}
		} else {
			if !exist {
				rbMerger.descResourceID[string(desc.GetUID())] = false
			} else if v == true {
				_ = rbMerger.deleteRB(rb)
			}
		}
	}
	rbMerger.mu.Unlock()
	return nil
}

func (rbMerger *RBMerger) handleToLocalResourceBinding(rb *appv1alpha1.ResourceBinding) error {
	klog.V(5).Infof("handle local resourceBinding %s", klog.KObj(rb))

	if rb.Namespace != common.GaiaRSToBeMergedReservedNamespace && rb.Namespace != common.GaiaRBMergedReservedNamespace {
		return nil
	}
	// delete unselected RBs
	if rb.Namespace == common.GaiaRBMergedReservedNamespace && rb.Spec.StatusScheduler == appv1alpha1.ResourceBindingSelected {
		err := rbMerger.deleteRBsUnselected(rb)
		if err != nil {
			return err
		}
		return nil
	}

	if rb.Namespace == common.GaiaRSToBeMergedReservedNamespace && rb.Spec.StatusScheduler == appv1alpha1.ResourceBindingMerging {

		clusters, err := rbMerger.localGaiaClient.PlatformV1alpha1().ManagedClusters(corev1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			klog.Warningf("failed to list managed clusters: %v", err)
		}
		descName := rb.GetLabels()[common.GaiaDescriptionLabel]

		// var fieldRBs *FieldRBs
		chanResult := make(chan []*appv1alpha1.ResourceBindingApps)
		rbMerger.mu.Lock()
		// rbsOfParentRB[field-xx]RBs
		if rbMerger.rbsOfParentRB[rb.Name] == nil {
			rbMerger.rbsOfParentRB[rb.Name] = &RBsOfParentRB{}
		}

		// rbMerger.rbsOfParentRB[rb.Name].count = len(rb.Spec.RbApps)
		for _, rbApp := range rb.Spec.RbApps {
			rbMerger.rbsOfParentRB[rb.Name].rbsOfParentRB = append(rbMerger.rbsOfParentRB[rb.Name].rbsOfParentRB, rbApp.Children[0])
		}

		if rbMerger.fieldsRBsOfParentRB[rb.Spec.ParentRB] == nil {
			rbMerger.fieldsRBsOfParentRB[rb.Spec.ParentRB] = &FieldsRBs{}
		}
		rbMerger.fieldsRBsOfParentRB[rb.Spec.ParentRB].countCls = len(clusters.Items)
		rbMerger.fieldsRBsOfParentRB[rb.Spec.ParentRB].NamesOfFiledRBs = append(rbMerger.fieldsRBsOfParentRB[rb.Spec.ParentRB].NamesOfFiledRBs, rb.Name)
		rbMerger.fieldsRBsOfParentRB[rb.Spec.ParentRB].rbsOfFields = append(rbMerger.fieldsRBsOfParentRB[rb.Spec.ParentRB].rbsOfFields, rbMerger.rbsOfParentRB[rb.Name])
		if !utils.ContainsString(rbMerger.parentsRBsOfAPPid[descName], rb.Spec.ParentRB) {
			rbMerger.parentsRBsOfAPPid[descName] = append(rbMerger.parentsRBsOfAPPid[descName], rb.Spec.ParentRB)
		}

		if rbMerger.mergeResourceBinding(rb.Spec.ParentRB, rbMerger.fieldsRBsOfParentRB, chanResult, rb) {
			err := rbMerger.deleteRBsCollected(rbMerger.fieldsRBsOfParentRB[rb.Spec.ParentRB].NamesOfFiledRBs)
			if err != nil {
				klog.Infof("Successful merged RB from %q, but failed to delete RBs locally.", rb.Spec.ParentRB)
				return err
			}
			if rbMerger.canDeleteAppID(descName, rb.Spec.TotalPeer) {
				rbMerger.deleteGlobalAppIDKey(descName)
				postErr := rbMerger.postMergedRBs(descName)
				if postErr != nil {
					return postErr
				}
			}
		}
		rbMerger.mu.Unlock()
	}

	return nil
}

func (rbMerger *RBMerger) mergeResourceBinding(parentRBName string, fieldsRBsOfParentRB map[string]*FieldsRBs, chanResult chan []*appv1alpha1.ResourceBindingApps, rb *appv1alpha1.ResourceBinding) bool {

	var childrens [][]*appv1alpha1.ResourceBindingApps
	if fieldsRbs, ok := fieldsRBsOfParentRB[parentRBName]; ok {
		if fieldsRbs.countCls == len(fieldsRbs.rbsOfFields) {
			for _, filedRBs := range fieldsRbs.rbsOfFields {
				childrens = append(childrens, filedRBs.rbsOfParentRB)
			}

			chanResult = cartesian.Iter(childrens...)

			// deploy the Merged ResourceBinding
			rbMerger.getMergedResourceBindings(chanResult, &parentRBName, rb)
			return true
		}
	}

	return false
}

func (rbMerger *RBMerger) getMergedResourceBindings(chanResult chan []*appv1alpha1.ResourceBindingApps, parentRBName *string, rb *appv1alpha1.ResourceBinding) {
	// deploy the Merged ResourceBinding
	index := 0
	descName := rb.GetLabels()[common.GaiaDescriptionLabel]
	for rbN := range chanResult {
		// create new result ResourceBinding
		newResultRB := &appv1alpha1.ResourceBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%d", *parentRBName, index),
				Namespace: common.GaiaRBMergedReservedNamespace,
				Labels: map[string]string{
					common.StatusScheduler:      string(appv1alpha1.ResourceBindingmerged),
					common.GaiaDescriptionLabel: rb.GetLabels()[common.GaiaDescriptionLabel],
				},
			},
			Spec: appv1alpha1.ResourceBindingSpec{
				AppID:           descName,
				TotalPeer:       rb.Spec.TotalPeer,
				ParentRB:        rb.Spec.ParentRB,
				RbApps:          rbN,
				NetworkPath:     rb.Spec.NetworkPath,
				StatusScheduler: appv1alpha1.ResourceBindingmerged,
			},
		}
		newResultRB.Kind = "ResourceBinding"
		newResultRB.APIVersion = "apps.gaia.io/v1alpha1"

		_, err := rbMerger.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRBMergedReservedNamespace).Create(context.TODO(), newResultRB, metav1.CreateOptions{})
		if err != nil {
			klog.V(3).InfoS("ResourceBinding of %q merge success, but not created success %q.", *parentRBName, common.GaiaRSToBeMergedReservedNamespace, err)
		} else {
			klog.Infof("ResourceBinding %q successfully merged and created.", *parentRBName)
		}

		index += 1
		// limit amount
		if index > 1 {
			break
		}
	}

}

func (rbMerger *RBMerger) canCreateCollectedRBs(rb *appv1alpha1.ResourceBinding) bool {
	descName := rb.GetLabels()[common.GaiaDescriptionLabel]
	totalPeer, err := strconv.Atoi(rb.GetLabels()[common.TotalPeerOfParentRB])
	if err != nil {
		klog.V(5).Infof("Failed to get totalPeer from label.")
		totalPeer = 0
	}
	if rb.Spec.TotalPeer != 0 && totalPeer == len(rbMerger.parentsRBsOfAPPid[descName]) && len(rbMerger.rbsOfParentRB[rb.Spec.ParentRB].rbsOfParentRB) == rb.Spec.TotalPeer {
		for _, parentRB := range rbMerger.parentsRBsOfAPPid[descName] {
			if len(rbMerger.rbsOfParentRB[parentRB].rbNames) != rbMerger.rbsOfParentRB[parentRB].count {
				return false
			}
		}
		return true
	}
	return false
}

func (rbMerger *RBMerger) createCollectedRBs(rb *appv1alpha1.ResourceBinding) bool {

	descName := rb.GetLabels()[common.GaiaDescriptionLabel]
	// field: all rb collected by parentRB
	for _, parentRB := range rbMerger.parentsRBsOfAPPid[descName] {

		var rbApps []*appv1alpha1.ResourceBindingApps
		for index, rbAppChild := range rbMerger.rbsOfParentRB[parentRB].rbsOfParentRB {
			rbApp := &appv1alpha1.ResourceBindingApps{
				ClusterName: rbMerger.rbsOfParentRB[parentRB].rbNames[index],
				Children:    []*appv1alpha1.ResourceBindingApps{rbAppChild},
			}

			rbApps = append(rbApps, rbApp)
		}

		totalPeer, err := strconv.Atoi(rb.GetLabels()[common.TotalPeerOfParentRB])
		if err != nil {
			klog.V(5).Infof("Failed to get totalPeer from label.")
			totalPeer = 0
		}

		// create new result ResourceBinding in parent cluster
		newResultRB := &appv1alpha1.ResourceBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s", parentRB, rbMerger.selfClusterName),
				Namespace: common.GaiaRSToBeMergedReservedNamespace,
				Labels: map[string]string{
					common.GaiaDescriptionLabel: rb.GetLabels()[common.GaiaDescriptionLabel],
				},
			},
			Spec: appv1alpha1.ResourceBindingSpec{
				AppID:           descName,
				TotalPeer:       totalPeer,
				ParentRB:        parentRB,
				RbApps:          rbApps,
				NetworkPath:     rb.Spec.NetworkPath,
				StatusScheduler: appv1alpha1.ResourceBindingMerging,
			},
		}
		newResultRB.Kind = "ResourceBinding"
		newResultRB.APIVersion = "apps.gaia.io/v1alpha1"

		_, err = rbMerger.parentGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRSToBeMergedReservedNamespace).Create(context.TODO(), newResultRB, metav1.CreateOptions{})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			klog.Infof("Failed to create ResourceBinding  %q.", newResultRB.Name, err)
			return false
		} else {
			klog.Infof("successfully create ResourceBinding %q to ParentCluster", newResultRB.Name)
		}

	}

	return true
}

func (rbMerger *RBMerger) deleteRBsUnselected(rb *appv1alpha1.ResourceBinding) error {
	klog.V(4).Infof("Delete unselected RBs of desc %q.", common.GaiaRSToBeMergedReservedNamespace)

	rb.GetLabels()[common.StatusScheduler] = string(appv1alpha1.ResourceBindingSelected)
	_, err := rbMerger.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRBMergedReservedNamespace).Update(context.TODO(), rb, metav1.UpdateOptions{})
	if err != nil {
		klog.V(4).Infof("Failed to update RB %q.", rb.Name)
		return err
	}

	descName := rb.GetLabels()[common.GaiaDescriptionLabel]
	err = rbMerger.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRBMergedReservedNamespace).
			DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{
				common.StatusScheduler:      string(appv1alpha1.ResourceBindingmerged),
				common.GaiaDescriptionLabel: descName,
			}).String()})
	if err != nil {
		klog.Infof("failed to delete rbs in %s namespace", common.GaiaRBMergedReservedNamespace, err)
		return err
	}
	return nil
}

func (rbMerger *RBMerger) deleteRBsCollected(rbNames []string) error {
	for _, name := range rbNames {
		err := rbMerger.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRSToBeMergedReservedNamespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		if err != nil {
			klog.Infof("failed to delete rb %q in %s namespace", name, common.GaiaRSToBeMergedReservedNamespace, err)
			return err
		}
	}
	return nil
}

func (rbMerger *RBMerger) reCreateRBtoParent(rb *appv1alpha1.ResourceBinding) error {
	newRB := &appv1alpha1.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rb.Name,
			Namespace: common.GaiaRSToBeMergedReservedNamespace,
			Labels:    rb.Labels,
		},
		Spec: appv1alpha1.ResourceBindingSpec{
			AppID:           rb.GetLabels()[common.GaiaDescriptionLabel],
			TotalPeer:       rb.Spec.TotalPeer,
			ParentRB:        rb.Spec.ParentRB,
			RbApps:          rb.Spec.RbApps,
			NetworkPath:     rb.Spec.NetworkPath,
			StatusScheduler: rb.Spec.StatusScheduler,
		},
	}
	rb.Kind = "ResourceBinding"
	rb.APIVersion = "apps.gaia.io/v1alpha1"

	_, err := rbMerger.parentGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRSToBeMergedReservedNamespace).Create(context.TODO(), newRB, metav1.CreateOptions{})

	return err
}

func (rbMerger *RBMerger) deleteMapAppID(descName string) {
	for _, parentRB := range rbMerger.parentsRBsOfAPPid[descName] {
		delete(rbMerger.fieldsRBsOfParentRB, parentRB)
	}
	delete(rbMerger.parentsRBsOfAPPid, descName)
}

func (rbMerger *RBMerger) postMergedRBs(descName string) error {
	var rbList []appv1alpha1.ResourceBinding
	desc, err := rbMerger.localGaiaClient.AppsV1alpha1().Descriptions(common.GaiaReservedNamespace).Get(context.TODO(), descName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	rbs, err := rbMerger.rbLister.ResourceBindings(common.GaiaRBMergedReservedNamespace).List(labels.SelectorFromSet(labels.Set{
		common.GaiaDescriptionLabel: descName,
		common.StatusScheduler:      string(appv1alpha1.ResourceBindingmerged),
	}))
	if err != nil {
		return nil
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
		return err
	}

	res, err := http.Post(rbMerger.postURL, "application/json", bytes.NewReader(postBody))
	defer func() { _ = res.Body.Close() }()
	if err != nil {
		klog.Infof("PostHyperOM: ERROR: post to HyperOM error %q.", err)
		return err
	}
	content, _ := ioutil.ReadAll(res.Body)
	klog.Infof("PostHyperOM: post the ResourceBindings of desc %q to HyperOM, Response: %s", descName, content)

	return nil
}

func (rbMerger *RBMerger) deleteRB(rb *appv1alpha1.ResourceBinding) error {
	err := rbMerger.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRSToBeMergedReservedNamespace).Delete(context.TODO(), rb.Name, metav1.DeleteOptions{})
	if err != nil {
		klog.Infof("Resource Binding %q failed to delete. error: ", rb.Name, err)
	}
	return err
}

func (rbMerger *RBMerger) canDeleteAppID(descName string, totalPeer int) bool {
	if len(rbMerger.parentsRBsOfAPPid[descName]) == totalPeer {
		for _, parentRB := range rbMerger.parentsRBsOfAPPid[descName] {
			if len(rbMerger.fieldsRBsOfParentRB[parentRB].rbsOfFields) != rbMerger.fieldsRBsOfParentRB[parentRB].countCls {
				return false
			}
		}
		return true
	}
	return false
}

func (rbMerger *RBMerger) deleteGlobalAppIDKey(descName string) {
	for _, parentRB := range rbMerger.parentsRBsOfAPPid[descName] {
		for _, rbName := range rbMerger.fieldsRBsOfParentRB[parentRB].NamesOfFiledRBs {
			delete(rbMerger.rbsOfParentRB, rbName)
		}
		delete(rbMerger.fieldsRBsOfParentRB, parentRB)
	}
	delete(rbMerger.parentsRBsOfAPPid, descName)
}

func (rbMerger *RBMerger) deleteFieldAppIDKey(descName string) {
	for _, parentRB := range rbMerger.parentsRBsOfAPPid[descName] {
		delete(rbMerger.rbsOfParentRB, parentRB)
	}
	delete(rbMerger.parentsRBsOfAPPid, descName)
}
