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
	appsv1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	labels "k8s.io/apimachinery/pkg/labels"
	listers "k8s.io/client-go/listers"
	cache "k8s.io/client-go/tools/cache"
)

// UserAPPLister helps list UserAPPs.
// All objects returned here must be treated as read-only.
type UserAPPLister interface {
	// List lists all UserAPPs in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*appsv1alpha1.UserAPP, err error)
	// UserAPPs returns an object that can list and get UserAPPs.
	UserAPPs(namespace string) UserAPPNamespaceLister
	UserAPPListerExpansion
}

// userAPPLister implements the UserAPPLister interface.
type userAPPLister struct {
	listers.ResourceIndexer[*appsv1alpha1.UserAPP]
}

// NewUserAPPLister returns a new UserAPPLister.
func NewUserAPPLister(indexer cache.Indexer) UserAPPLister {
	return &userAPPLister{listers.New[*appsv1alpha1.UserAPP](indexer, appsv1alpha1.Resource("userapp"))}
}

// UserAPPs returns an object that can list and get UserAPPs.
func (s *userAPPLister) UserAPPs(namespace string) UserAPPNamespaceLister {
	return userAPPNamespaceLister{listers.NewNamespaced[*appsv1alpha1.UserAPP](s.ResourceIndexer, namespace)}
}

// UserAPPNamespaceLister helps list and get UserAPPs.
// All objects returned here must be treated as read-only.
type UserAPPNamespaceLister interface {
	// List lists all UserAPPs in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*appsv1alpha1.UserAPP, err error)
	// Get retrieves the UserAPP from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*appsv1alpha1.UserAPP, error)
	UserAPPNamespaceListerExpansion
}

// userAPPNamespaceLister implements the UserAPPNamespaceLister
// interface.
type userAPPNamespaceLister struct {
	listers.ResourceIndexer[*appsv1alpha1.UserAPP]
}
