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
	v1alpha1 "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	platformv1alpha1 "github.com/lmxia/gaia/pkg/generated/clientset/versioned/typed/platform/v1alpha1"
	gentype "k8s.io/client-go/gentype"
)

// fakeTargets implements TargetInterface
type fakeTargets struct {
	*gentype.FakeClientWithList[*v1alpha1.Target, *v1alpha1.TargetList]
	Fake *FakePlatformV1alpha1
}

func newFakeTargets(fake *FakePlatformV1alpha1) platformv1alpha1.TargetInterface {
	return &fakeTargets{
		gentype.NewFakeClientWithList[*v1alpha1.Target, *v1alpha1.TargetList](
			fake.Fake,
			"",
			v1alpha1.SchemeGroupVersion.WithResource("targets"),
			v1alpha1.SchemeGroupVersion.WithKind("Target"),
			func() *v1alpha1.Target { return &v1alpha1.Target{} },
			func() *v1alpha1.TargetList { return &v1alpha1.TargetList{} },
			func(dst, src *v1alpha1.TargetList) { dst.ListMeta = src.ListMeta },
			func(list *v1alpha1.TargetList) []*v1alpha1.Target { return gentype.ToPointerSlice(list.Items) },
			func(list *v1alpha1.TargetList, items []*v1alpha1.Target) {
				list.Items = gentype.FromPointerSlice(items)
			},
		),
		fake,
	}
}
