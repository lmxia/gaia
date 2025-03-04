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

	storagev1alpha1 "github.com/lmxia/gaia/pkg/apis/storage/v1alpha1"
	scheme "github.com/lmxia/gaia/pkg/generated/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
)

// HyperImagesGetter has a method to return a HyperImageInterface.
// A group's client should implement this interface.
type HyperImagesGetter interface {
	HyperImages(namespace string) HyperImageInterface
}

// HyperImageInterface has methods to work with HyperImage resources.
type HyperImageInterface interface {
	Create(ctx context.Context, hyperImage *storagev1alpha1.HyperImage, opts v1.CreateOptions) (*storagev1alpha1.HyperImage, error)
	Update(ctx context.Context, hyperImage *storagev1alpha1.HyperImage, opts v1.UpdateOptions) (*storagev1alpha1.HyperImage, error)
	// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
	UpdateStatus(ctx context.Context, hyperImage *storagev1alpha1.HyperImage, opts v1.UpdateOptions) (*storagev1alpha1.HyperImage, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*storagev1alpha1.HyperImage, error)
	List(ctx context.Context, opts v1.ListOptions) (*storagev1alpha1.HyperImageList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *storagev1alpha1.HyperImage, err error)
	HyperImageExpansion
}

// hyperImages implements HyperImageInterface
type hyperImages struct {
	*gentype.ClientWithList[*storagev1alpha1.HyperImage, *storagev1alpha1.HyperImageList]
}

// newHyperImages returns a HyperImages
func newHyperImages(c *StorageV1alpha1Client, namespace string) *hyperImages {
	return &hyperImages{
		gentype.NewClientWithList[*storagev1alpha1.HyperImage, *storagev1alpha1.HyperImageList](
			"hyperimages",
			c.RESTClient(),
			scheme.ParameterCodec,
			namespace,
			func() *storagev1alpha1.HyperImage { return &storagev1alpha1.HyperImage{} },
			func() *storagev1alpha1.HyperImageList { return &storagev1alpha1.HyperImageList{} },
		),
	}
}
