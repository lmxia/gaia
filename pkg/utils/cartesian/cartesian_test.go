package cartesian_test

import (
	"fmt"
	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/utils/cartesian"
	"testing"
)

func TestIter(t *testing.T) {

	children1 := []*v1alpha1.ResourceBindingApps{
		0: {
			ClusterName: "cluster-0",
			Replicas: map[string]int32{
				"a": 1,
				"b": 1,
			},
		},
		1: {
			ClusterName: "cluster-1",
			Replicas: map[string]int32{
				"a": 3,
				"b": 3,
			},
		},
	}

	children2 := []*v1alpha1.ResourceBindingApps{
		0: {
			ClusterName: "cluster-a",
			Replicas: map[string]int32{
				"a": 2,
				"b": 2,
			},
		},
		1: {
			ClusterName: "cluster-b",
			Replicas: map[string]int32{
				"a": 4,
				"b": 4,
			},
		},
	}

	c := cartesian.Iter(children1, children2)

	// receive products through channel
	for product := range c {
		fmt.Println(product)
		for _, value := range product {
			fmt.Println(value)
		}
	}

	t.Log("Compare with the following results.")
	// Unordered Output:
	// [0xc0001f4ed0 0xc0001f4f90]
	// &{cluster-1 map[a:3 b:3] []}
	// &{cluster-b map[a:4 b:4] []}
	// [0xc0001f4ed0 0xc0001f4f30]
	// &{cluster-1 map[a:3 b:3] []}
	// &{cluster-a map[a:2 b:2] []}
	// [0xc0001f4e70 0xc0001f4f90]
	// &{cluster-0 map[a:1 b:1] []}
	// &{cluster-b map[a:4 b:4] []}
	// [0xc0001f4e70 0xc0001f4f30]
	// &{cluster-0 map[a:1 b:1] []}
	// &{cluster-a map[a:2 b:2] []}
}
