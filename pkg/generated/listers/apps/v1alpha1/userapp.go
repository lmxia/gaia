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

// UserAPPLister helps list UserAPPs.
// All objects returned here must be treated as read-only.
type UserAPPLister interface {
	// List lists all UserAPPs in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.UserAPP, err error)
	// UserAPPs returns an object that can list and get UserAPPs.
	UserAPPs(namespace string) UserAPPNamespaceLister
	UserAPPListerExpansion
}

// userAPPLister implements the UserAPPLister interface.
type userAPPLister struct {
	indexer cache.Indexer
}

// NewUserAPPLister returns a new UserAPPLister.
func NewUserAPPLister(indexer cache.Indexer) UserAPPLister {
	return &userAPPLister{indexer: indexer}
}

// List lists all UserAPPs in the indexer.
func (s *userAPPLister) List(selector labels.Selector) (ret []*v1alpha1.UserAPP, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.UserAPP))
	})
	return ret, err
}

// UserAPPs returns an object that can list and get UserAPPs.
func (s *userAPPLister) UserAPPs(namespace string) UserAPPNamespaceLister {
	return userAPPNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// UserAPPNamespaceLister helps list and get UserAPPs.
// All objects returned here must be treated as read-only.
type UserAPPNamespaceLister interface {
	// List lists all UserAPPs in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.UserAPP, err error)
	// Get retrieves the UserAPP from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.UserAPP, error)
	UserAPPNamespaceListerExpansion
}

// userAPPNamespaceLister implements the UserAPPNamespaceLister
// interface.
type userAPPNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all UserAPPs in the indexer for a given namespace.
func (s userAPPNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.UserAPP, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.UserAPP))
	})
	return ret, err
}

// Get retrieves the UserAPP from the indexer for a given namespace and name.
func (s userAPPNamespaceLister) Get(name string) (*v1alpha1.UserAPP, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("userapp"), name)
	}
	return obj.(*v1alpha1.UserAPP), nil
}
