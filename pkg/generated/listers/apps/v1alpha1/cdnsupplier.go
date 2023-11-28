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
	v1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// CdnSupplierLister helps list CdnSuppliers.
// All objects returned here must be treated as read-only.
type CdnSupplierLister interface {
	// List lists all CdnSuppliers in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.CdnSupplier, err error)
	// CdnSuppliers returns an object that can list and get CdnSuppliers.
	CdnSuppliers(namespace string) CdnSupplierNamespaceLister
	CdnSupplierListerExpansion
}

// cdnSupplierLister implements the CdnSupplierLister interface.
type cdnSupplierLister struct {
	indexer cache.Indexer
}

// NewCdnSupplierLister returns a new CdnSupplierLister.
func NewCdnSupplierLister(indexer cache.Indexer) CdnSupplierLister {
	return &cdnSupplierLister{indexer: indexer}
}

// List lists all CdnSuppliers in the indexer.
func (s *cdnSupplierLister) List(selector labels.Selector) (ret []*v1alpha1.CdnSupplier, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.CdnSupplier))
	})
	return ret, err
}

// CdnSuppliers returns an object that can list and get CdnSuppliers.
func (s *cdnSupplierLister) CdnSuppliers(namespace string) CdnSupplierNamespaceLister {
	return cdnSupplierNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// CdnSupplierNamespaceLister helps list and get CdnSuppliers.
// All objects returned here must be treated as read-only.
type CdnSupplierNamespaceLister interface {
	// List lists all CdnSuppliers in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.CdnSupplier, err error)
	// Get retrieves the CdnSupplier from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.CdnSupplier, error)
	CdnSupplierNamespaceListerExpansion
}

// cdnSupplierNamespaceLister implements the CdnSupplierNamespaceLister
// interface.
type cdnSupplierNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all CdnSuppliers in the indexer for a given namespace.
func (s cdnSupplierNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.CdnSupplier, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.CdnSupplier))
	})
	return ret, err
}

// Get retrieves the CdnSupplier from the indexer for a given namespace and name.
func (s cdnSupplierNamespaceLister) Get(name string) (*v1alpha1.CdnSupplier, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("cdnsupplier"), name)
	}
	return obj.(*v1alpha1.CdnSupplier), nil
}