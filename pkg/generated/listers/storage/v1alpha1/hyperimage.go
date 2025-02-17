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
	storagev1alpha1 "github.com/lmxia/gaia/pkg/apis/storage/v1alpha1"
	labels "k8s.io/apimachinery/pkg/labels"
	listers "k8s.io/client-go/listers"
	cache "k8s.io/client-go/tools/cache"
)

// HyperImageLister helps list HyperImages.
// All objects returned here must be treated as read-only.
type HyperImageLister interface {
	// List lists all HyperImages in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*storagev1alpha1.HyperImage, err error)
	// HyperImages returns an object that can list and get HyperImages.
	HyperImages(namespace string) HyperImageNamespaceLister
	HyperImageListerExpansion
}

// hyperImageLister implements the HyperImageLister interface.
type hyperImageLister struct {
	listers.ResourceIndexer[*storagev1alpha1.HyperImage]
}

// NewHyperImageLister returns a new HyperImageLister.
func NewHyperImageLister(indexer cache.Indexer) HyperImageLister {
	return &hyperImageLister{listers.New[*storagev1alpha1.HyperImage](indexer, storagev1alpha1.Resource("hyperimage"))}
}

// HyperImages returns an object that can list and get HyperImages.
func (s *hyperImageLister) HyperImages(namespace string) HyperImageNamespaceLister {
	return hyperImageNamespaceLister{listers.NewNamespaced[*storagev1alpha1.HyperImage](s.ResourceIndexer, namespace)}
}

// HyperImageNamespaceLister helps list and get HyperImages.
// All objects returned here must be treated as read-only.
type HyperImageNamespaceLister interface {
	// List lists all HyperImages in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*storagev1alpha1.HyperImage, err error)
	// Get retrieves the HyperImage from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*storagev1alpha1.HyperImage, error)
	HyperImageNamespaceListerExpansion
}

// hyperImageNamespaceLister implements the HyperImageNamespaceLister
// interface.
type hyperImageNamespaceLister struct {
	listers.ResourceIndexer[*storagev1alpha1.HyperImage]
}
