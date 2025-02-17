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

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	context "context"

	appsv1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	scheme "github.com/lmxia/gaia/pkg/generated/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
)

// ResourceBindingsGetter has a method to return a ResourceBindingInterface.
// A group's client should implement this interface.
type ResourceBindingsGetter interface {
	ResourceBindings(namespace string) ResourceBindingInterface
}

// ResourceBindingInterface has methods to work with ResourceBinding resources.
type ResourceBindingInterface interface {
	Create(ctx context.Context, resourceBinding *appsv1alpha1.ResourceBinding, opts v1.CreateOptions) (*appsv1alpha1.ResourceBinding, error)
	Update(ctx context.Context, resourceBinding *appsv1alpha1.ResourceBinding, opts v1.UpdateOptions) (*appsv1alpha1.ResourceBinding, error)
	// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
	UpdateStatus(ctx context.Context, resourceBinding *appsv1alpha1.ResourceBinding, opts v1.UpdateOptions) (*appsv1alpha1.ResourceBinding, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*appsv1alpha1.ResourceBinding, error)
	List(ctx context.Context, opts v1.ListOptions) (*appsv1alpha1.ResourceBindingList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *appsv1alpha1.ResourceBinding, err error)
	ResourceBindingExpansion
}

// resourceBindings implements ResourceBindingInterface
type resourceBindings struct {
	*gentype.ClientWithList[*appsv1alpha1.ResourceBinding, *appsv1alpha1.ResourceBindingList]
}

// newResourceBindings returns a ResourceBindings
func newResourceBindings(c *AppsV1alpha1Client, namespace string) *resourceBindings {
	return &resourceBindings{
		gentype.NewClientWithList[*appsv1alpha1.ResourceBinding, *appsv1alpha1.ResourceBindingList](
			"resourcebindings",
			c.RESTClient(),
			scheme.ParameterCodec,
			namespace,
			func() *appsv1alpha1.ResourceBinding { return &appsv1alpha1.ResourceBinding{} },
			func() *appsv1alpha1.ResourceBindingList { return &appsv1alpha1.ResourceBindingList{} },
		),
	}
}
