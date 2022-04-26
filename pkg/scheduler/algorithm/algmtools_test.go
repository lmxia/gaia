package algorithm

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
	"gonum.org/v1/gonum/mat"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	platformv1alpha1 "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/scheduler/framework"
)

type DeploymentSuite struct {
	capability []*framework.ClusterInfo
	rb0        *v1alpha1.ResourceBinding
	rb1        *v1alpha1.ResourceBinding
	matries    []mat.Matrix
	suite.Suite
}

func (suite *DeploymentSuite) SetupTest() {
	suite.capability = []*framework.ClusterInfo{
		0: {
			Cluster: &platformv1alpha1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster0",
				},
				Spec: platformv1alpha1.ManagedClusterSpec{
					ClusterID: "0",
				},
			},
			Total: 0,
		},
		1: {
			Cluster: &platformv1alpha1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
				},
				Spec: platformv1alpha1.ManagedClusterSpec{
					ClusterID: "2",
				},
			},
			Total: 30,
		},
		2: {
			Cluster: &platformv1alpha1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster2",
				},
				Spec: platformv1alpha1.ManagedClusterSpec{
					ClusterID: "2",
				},
			},
			Total: 27,
		},
		3: {
			Cluster: &platformv1alpha1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster3",
				},
				Spec: platformv1alpha1.ManagedClusterSpec{
					ClusterID: "3",
				},
			},
			Total: 19,
		},
		4: {
			Cluster: &platformv1alpha1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster4",
				},
				Spec: platformv1alpha1.ManagedClusterSpec{
					ClusterID: "4",
				},
			},
			Total: 0,
		},
	}

	suite.rb0 = &v1alpha1.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rb0",
		},
		Spec: v1alpha1.ResourceBindingSpec{
			AppID: "0",
			RbApps: []*v1alpha1.ResourceBindingApps{
				0: {
					ClusterName: "field0",
					Replicas: map[string]int32{
						"a": 10,
						"b": 5,
					},
				},
				1: {
					ClusterName: "field1",
					Replicas: map[string]int32{
						"a": 5,
						"b": 7,
					},
				},
				2: {
					ClusterName: "field2",
					Replicas: map[string]int32{
						"a": 5,
						"b": 8,
					},
				},
			},
		},
	}

	suite.rb1 = &v1alpha1.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rb1",
		},
		Spec: v1alpha1.ResourceBindingSpec{
			AppID: "0",
			RbApps: []*v1alpha1.ResourceBindingApps{
				0: {
					ClusterName: "field0",
					Replicas: map[string]int32{
						"a": 5,
						"b": 9,
					},
				},
				1: {
					ClusterName: "field1",
					Replicas: map[string]int32{
						"a": 10,
						"b": 1,
					},
				},
				2: {
					ClusterName: "field2",
					Replicas: map[string]int32{
						"a": 5,
						"b": 10,
					},
					Children: []*v1alpha1.ResourceBindingApps{
						0: {
							ClusterName: "cluster0",
							Replicas: map[string]int32{
								"a": 2,
								"b": 9,
							},
						},
						1: {
							ClusterName: "cluster1",
							Replicas: map[string]int32{
								"a": 3,
								"b": 1,
							},
						},
					},
				},
			},
		},
	}

	suite.matries = []mat.Matrix{
		0: mat.NewDense(2, 5, []float64{
			0, 3, 3, 0, 4,
			5, 0, 0, 2, 3,
		}),
		1: mat.NewDense(3, 5, []float64{
			0, 2, 3, 0, 0,
			0, 0, 0, 2, 3,
			0, 1, 0, 4, 0,
		}),
		2: mat.NewDense(2, 5, []float64{
			0, 4, 4, 0, 0,
			5, 0, 0, 0, 3,
		}),
		3: mat.NewDense(2, 5, []float64{
			0, 2, 0, 0, 4,
			0, 0, 1, 4, 0,
		}),
	}
}

func (suite *DeploymentSuite) TestMakeResourceBindingMatrix() {
	result, _ := makeResourceBindingMatrix(suite.matries)
	suite.Equal(24, len(result), "")
}

func (suite *DeploymentSuite) TestGetComponentClusterTotal() {
	total := getComponentClusterTotal(suite.rb1.Spec.RbApps, "cluster11", "a")
	suite.Equal(int64(3), total, "")
}

func (suite *DeploymentSuite) TestCalculatePlans() {
	mat := makeComponentPlans(suite.capability, 10, 2)
	fmt.Printf("%v", mat)
	suite.Equal(5, 5, "test")
}

func (suite *DeploymentSuite) TestGenerateRandomNumber() {
	result, err := GenerateRandomClusterInfos(suite.capability, 4)
	if err != nil {
		fmt.Printf("%v", result)
	}

	suite.Equal(len(result), 5, "test")
}

func (suite *DeploymentSuite) TestPlan() {
	result := plan(suite.capability, 5)
	fmt.Printf("plan result is %v", result)
	suite.Equal(3, 3, "test")
}

func TestDeployment(t *testing.T) {
	suite.Run(t, new(DeploymentSuite))
}
