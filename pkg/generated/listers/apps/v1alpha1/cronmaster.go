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

// CronMasterLister helps list CronMasters.
// All objects returned here must be treated as read-only.
type CronMasterLister interface {
	// List lists all CronMasters in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.CronMaster, err error)
	// CronMasters returns an object that can list and get CronMasters.
	CronMasters(namespace string) CronMasterNamespaceLister
	CronMasterListerExpansion
}

// cronMasterLister implements the CronMasterLister interface.
type cronMasterLister struct {
	indexer cache.Indexer
}

// NewCronMasterLister returns a new CronMasterLister.
func NewCronMasterLister(indexer cache.Indexer) CronMasterLister {
	return &cronMasterLister{indexer: indexer}
}

// List lists all CronMasters in the indexer.
func (s *cronMasterLister) List(selector labels.Selector) (ret []*v1alpha1.CronMaster, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.CronMaster))
	})
	return ret, err
}

// CronMasters returns an object that can list and get CronMasters.
func (s *cronMasterLister) CronMasters(namespace string) CronMasterNamespaceLister {
	return cronMasterNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// CronMasterNamespaceLister helps list and get CronMasters.
// All objects returned here must be treated as read-only.
type CronMasterNamespaceLister interface {
	// List lists all CronMasters in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.CronMaster, err error)
	// Get retrieves the CronMaster from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.CronMaster, error)
	CronMasterNamespaceListerExpansion
}

// cronMasterNamespaceLister implements the CronMasterNamespaceLister
// interface.
type cronMasterNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all CronMasters in the indexer for a given namespace.
func (s cronMasterNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.CronMaster, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.CronMaster))
	})
	return ret, err
}

// Get retrieves the CronMaster from the indexer for a given namespace and name.
func (s cronMasterNamespaceLister) Get(name string) (*v1alpha1.CronMaster, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("cronmaster"), name)
	}
	return obj.(*v1alpha1.CronMaster), nil
}
