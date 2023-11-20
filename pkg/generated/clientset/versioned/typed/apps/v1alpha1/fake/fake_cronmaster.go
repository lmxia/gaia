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

package fake

import (
	"context"

	v1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeCronMasters implements CronMasterInterface
type FakeCronMasters struct {
	Fake *FakeAppsV1alpha1
	ns   string
}

var cronmastersResource = schema.GroupVersionResource{Group: "apps.gaia.io", Version: "v1alpha1", Resource: "cronmasters"}

var cronmastersKind = schema.GroupVersionKind{Group: "apps.gaia.io", Version: "v1alpha1", Kind: "CronMaster"}

// Get takes name of the cronMaster, and returns the corresponding cronMaster object, and an error if there is any.
func (c *FakeCronMasters) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.CronMaster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(cronmastersResource, c.ns, name), &v1alpha1.CronMaster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CronMaster), err
}

// List takes label and field selectors, and returns the list of CronMasters that match those selectors.
func (c *FakeCronMasters) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.CronMasterList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(cronmastersResource, cronmastersKind, c.ns, opts), &v1alpha1.CronMasterList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.CronMasterList{ListMeta: obj.(*v1alpha1.CronMasterList).ListMeta}
	for _, item := range obj.(*v1alpha1.CronMasterList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested cronMasters.
func (c *FakeCronMasters) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(cronmastersResource, c.ns, opts))

}

// Create takes the representation of a cronMaster and creates it.  Returns the server's representation of the cronMaster, and an error, if there is any.
func (c *FakeCronMasters) Create(ctx context.Context, cronMaster *v1alpha1.CronMaster, opts v1.CreateOptions) (result *v1alpha1.CronMaster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(cronmastersResource, c.ns, cronMaster), &v1alpha1.CronMaster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CronMaster), err
}

// Update takes the representation of a cronMaster and updates it. Returns the server's representation of the cronMaster, and an error, if there is any.
func (c *FakeCronMasters) Update(ctx context.Context, cronMaster *v1alpha1.CronMaster, opts v1.UpdateOptions) (result *v1alpha1.CronMaster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(cronmastersResource, c.ns, cronMaster), &v1alpha1.CronMaster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CronMaster), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeCronMasters) UpdateStatus(ctx context.Context, cronMaster *v1alpha1.CronMaster, opts v1.UpdateOptions) (*v1alpha1.CronMaster, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(cronmastersResource, "status", c.ns, cronMaster), &v1alpha1.CronMaster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CronMaster), err
}

// Delete takes name of the cronMaster and deletes it. Returns an error if one occurs.
func (c *FakeCronMasters) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(cronmastersResource, c.ns, name), &v1alpha1.CronMaster{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeCronMasters) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(cronmastersResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.CronMasterList{})
	return err
}

// Patch applies the patch and returns the patched cronMaster.
func (c *FakeCronMasters) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.CronMaster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(cronmastersResource, c.ns, name, pt, data, subresources...), &v1alpha1.CronMaster{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CronMaster), err
}
