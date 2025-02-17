/*
Copyright The Gaia Authors.

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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	servicev1alpha1 "github.com/lmxia/gaia/pkg/apis/service/v1alpha1"
	labels "k8s.io/apimachinery/pkg/labels"
	listers "k8s.io/client-go/listers"
	cache "k8s.io/client-go/tools/cache"
)

// HyperLabelLister helps list HyperLabels.
// All objects returned here must be treated as read-only.
type HyperLabelLister interface {
	// List lists all HyperLabels in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*servicev1alpha1.HyperLabel, err error)
	// HyperLabels returns an object that can list and get HyperLabels.
	HyperLabels(namespace string) HyperLabelNamespaceLister
	HyperLabelListerExpansion
}

// hyperLabelLister implements the HyperLabelLister interface.
type hyperLabelLister struct {
	listers.ResourceIndexer[*servicev1alpha1.HyperLabel]
}

// NewHyperLabelLister returns a new HyperLabelLister.
func NewHyperLabelLister(indexer cache.Indexer) HyperLabelLister {
	return &hyperLabelLister{listers.New[*servicev1alpha1.HyperLabel](indexer, servicev1alpha1.Resource("hyperlabel"))}
}

// HyperLabels returns an object that can list and get HyperLabels.
func (s *hyperLabelLister) HyperLabels(namespace string) HyperLabelNamespaceLister {
	return hyperLabelNamespaceLister{listers.NewNamespaced[*servicev1alpha1.HyperLabel](s.ResourceIndexer, namespace)}
}

// HyperLabelNamespaceLister helps list and get HyperLabels.
// All objects returned here must be treated as read-only.
type HyperLabelNamespaceLister interface {
	// List lists all HyperLabels in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*servicev1alpha1.HyperLabel, err error)
	// Get retrieves the HyperLabel from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*servicev1alpha1.HyperLabel, error)
	HyperLabelNamespaceListerExpansion
}

// hyperLabelNamespaceLister implements the HyperLabelNamespaceLister
// interface.
type hyperLabelNamespaceLister struct {
	listers.ResourceIndexer[*servicev1alpha1.HyperLabel]
}
