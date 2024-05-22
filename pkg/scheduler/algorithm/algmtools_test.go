package algorithm

import (
	"fmt"
	"reflect"
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
					Labels: map[string]string{
						"clusters.gaia.io/yuanlao": "no",
					},
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
					Labels: map[string]string{
						"clusters.gaia.io/yuanlao": "no",
					},
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
					Labels: map[string]string{
						"clusters.gaia.io/yuanlao": "yes",
					},
				},
				Spec: platformv1alpha1.ManagedClusterSpec{
					ClusterID: "2",
				},
			},
			Total: 10,
		},
		3: {
			Cluster: &platformv1alpha1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster3",
					Labels: map[string]string{
						"clusters.gaia.io/yuanlao": "no",
					},
				},
				Spec: platformv1alpha1.ManagedClusterSpec{
					ClusterID: "3",
				},
			},
			Total: 0,
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
			0, 0, 0, 0, 4,
			0, 0, 0, 0, 0,
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
	mat := makeDeployPlans(suite.capability, 10, 1)
	fmt.Printf("%v", mat)
	suite.Equal(5, 5, "test")
}

func (suite *DeploymentSuite) TestGenerateRandomNumber() {
	result, err := GenerateRandomClusterInfos(suite.capability, 1)
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

func (suite *DeploymentSuite) TestChosenOneInArrow() {
	clusters := make([]*platformv1alpha1.ManagedCluster, 0)
	for _, item := range suite.capability {
		clusters = append(clusters, item.Cluster)
	}
	result := chosenOneInArrow(suite.matries[0], clusters)
	fmt.Printf("%v", result)
	suite.Equal(24, 24, "")
}

func TestDeployment(t *testing.T) {
	suite.Run(t, new(DeploymentSuite))
}

func TestGetAffinityComPlan(t *testing.T) {
	type args struct {
		result   mat.Matrix
		replicas int64
	}
	tests := []struct {
		name string
		args args
		want mat.Matrix
	}{
		// affinity test
		0: {
			name: "affinity test1-1",
			args: args{
				result: mat.NewDense(2, 3, []float64{
					0, 10, 0,
					10, 0, 0,
				}),
				replicas: 8,
			},
			want: mat.NewDense(2, 3, []float64{
				0, 8, 0,
				8, 0, 0,
			}),
		},
		1: {
			name: "affinity test1-2",
			args: args{
				result: mat.NewDense(2, 3, []float64{
					5, 5, 0,
					0, 5, 5,
				}),
				replicas: 8,
			},
			want: mat.NewDense(2, 3, []float64{
				4, 4, 0,
				0, 4, 4,
			}),
		},
		2: {
			name: "affinity test1-3",
			args: args{
				result: mat.NewDense(2, 3, []float64{
					3, 3, 4,
					2, 2, 6,
				}),
				replicas: 8,
			},
			want: mat.NewDense(2, 3, []float64{
				3, 2, 3,
				1, 2, 5,
			}),
		},
		3: {
			name: "affinity test2",
			args: args{
				result: mat.NewDense(3, 4, []float64{
					0, 1, 4, 0,
					0, 2, 3, 0,
					0, 3, 2, 0,
				}),
				replicas: 3,
			},
			want: mat.NewDense(3, 4, []float64{
				0, 1, 2, 0,
				0, 1, 2, 0,
				0, 2, 1, 0,
			}),
		},
		4: {
			name: "affinity test3 serverless",
			args: args{
				result: mat.NewDense(1, 4, []float64{
					1, 1, 1, 1,
				}),
				replicas: 2,
			},
			want: mat.NewDense(1, 4, []float64{
				0, 0, 1, 1,
			}),
		},
		5: {
			name: "affinity test4 zero",
			args: args{
				result: mat.NewDense(1, 4, []float64{
					0, 0, 0, 0,
				}),
				replicas: 2,
			},
			want: mat.NewDense(1, 4, []float64{
				0, 0, 0, 0,
			}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetAffinityComPlan(tt.args.result, tt.args.replicas); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAffinityComPlan() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAffinityComPlanForDeployment(t *testing.T) {
	type args struct {
		result     mat.Matrix
		replicas   int64
		dispersion bool
	}
	tests := []struct {
		name string
		args args
		want mat.Matrix
	}{
		// affinity test
		0: {
			name: "affinity test1-1",
			args: args{
				result: mat.NewDense(2, 3, []float64{
					0, 10, 0,
					10, 0, 0,
				}),
				replicas:   8,
				dispersion: false,
			},
			want: mat.NewDense(2, 3, []float64{
				0, 8, 0,
				8, 0, 0,
			}),
		},
		1: {
			name: "affinity test1-2",
			args: args{
				result: mat.NewDense(2, 3, []float64{
					5, 5, 0,
					0, 5, 5,
				}),
				replicas:   8,
				dispersion: false,
			},
			want: mat.NewDense(2, 3, []float64{
				8, 0, 0,
				0, 8, 0,
			}),
		},
		2: {
			name: "affinity test1-3",
			args: args{
				result: mat.NewDense(2, 3, []float64{
					3, 3, 4,
					2, 2, 6,
				}),
				replicas:   8,
				dispersion: true,
			},
			want: mat.NewDense(2, 3, []float64{
				3, 2, 3,
				1, 2, 5,
			}),
		},
		3: {
			name: "affinity test1-4 serverless",
			args: args{
				result: mat.NewDense(1, 4, []float64{
					1, 1, 1, 1,
				}),
				replicas:   2,
				dispersion: false,
			},
			want: mat.NewDense(1, 4, []float64{
				2, 0, 0, 0,
			}),
		},
		4: {
			name: "affinity test1-5 serverless",
			args: args{
				result: mat.NewDense(1, 4, []float64{
					1, 1, 1, 1,
				}),
				replicas:   2,
				dispersion: true,
			},
			want: mat.NewDense(1, 4, []float64{
				0, 0, 1, 1,
			}),
		},
		5: {
			name: "affinity test1-6 zero, dispersion true",
			args: args{
				result: mat.NewDense(1, 4, []float64{
					0, 0, 0, 0,
				}),
				replicas:   2,
				dispersion: true,
			},
			want: mat.NewDense(1, 4, []float64{
				0, 0, 0, 0,
			}),
		},
		6: {
			name: "affinity test1-7 zero, dispersion false",
			args: args{
				result: mat.NewDense(1, 4, []float64{
					0, 0, 0, 0,
				}),
				replicas:   2,
				dispersion: false,
			},
			want: mat.NewDense(1, 4, []float64{
				0, 0, 0, 0,
			}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetAffinityComPlanForDeployment(tt.args.result, tt.args.replicas,
				tt.args.dispersion); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAffinityComPlanForDeployment() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_checkMatrix(t *testing.T) {
	type args struct {
		m mat.Matrix
		i int
		j int
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// test case.
		0: {
			name: "test1",
			args: args{
				mat.NewDense(4, 2, []float64{
					3, 0,
					1, 0,
					1, 1,
					1, 0,
				}), 1, 0,
			},
			want: true,
		},
		1: {
			name: "test2",
			args: args{
				mat.NewDense(4, 2, []float64{
					3, 0,
					1, 0,
					1, 1,
					1, 0,
				}), 2, 0,
			},
			want: false,
		},
		2: {
			name: "test3: A serverless, B deployment, dispersion false",
			args: args{
				mat.NewDense(2, 2, []float64{
					1, 1,
					3, 0,
				}), 1, 0,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkMatrix(tt.args.m, tt.args.i, tt.args.j); got != tt.want {
				t.Errorf("checkMatix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAffinityComPlanForServerless(t *testing.T) {
	type args struct {
		m mat.Matrix
	}
	tests := []struct {
		name string
		args args
		want mat.Matrix
	}{
		// test case.
		0: {
			name: "test1 A: deployment",
			args: args{
				mat.NewDense(2, 2, []float64{
					3, 0,
					0, 1,
				}),
			},
			want: mat.NewDense(2, 2, []float64{
				1, 0,
				0, 1,
			}),
		},
		1: {
			name: "test2 A: deployment",
			args: args{
				mat.NewDense(2, 3, []float64{
					3, 0, 4,
					0, 1, 6,
				}),
			},
			want: mat.NewDense(2, 3, []float64{
				1, 0, 1,
				0, 1, 1,
			}),
		},
		2: {
			name: "test3 A: serverless",
			args: args{
				mat.NewDense(2, 2, []float64{
					1, 0,
					0, 1,
				}),
			},
			want: mat.NewDense(2, 2, []float64{
				1, 0,
				0, 1,
			}),
		},
		3: {
			name: "test4 A: serverless",
			args: args{
				mat.NewDense(2, 3, []float64{
					1, 0, 1,
					0, 1, 1,
				}),
			},
			want: mat.NewDense(2, 3, []float64{
				1, 0, 1,
				0, 1, 1,
			}),
		},
		4: {
			name: "test5 zero mat",
			args: args{
				mat.NewDense(2, 3, []float64{
					0, 0, 0,
					0, 0, 0,
				}),
			},
			want: mat.NewDense(2, 3, []float64{
				0, 0, 0,
				0, 0, 0,
			}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetAffinityComPlanForServerless(tt.args.m); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAffinityComPlanForServerless() = %v, want %v", got, tt.want)
			}
		})
	}
}

func (suite *DeploymentSuite) Test_makeUniqeDeployPlans() {
	// [0,30, 10, 0, 0]
	got := makeUniqeDeployPlans(suite.capability, 12, 2)
	fmt.Printf("%v", got)
	data := []float64{
		0, 9, 3, 0, 0,
	}
	matrix := mat.NewDense(1, 5, data)
	suite.Equal(got, matrix, "")
}

func (suite *DeploymentSuite) Test_makeUniqeDeployPlansOne() {
	// [0,30, 10, 0, 0]
	got := makeUniqeDeployPlans(suite.capability, 12, 1)
	fmt.Printf("%v", got)
	data := []float64{
		0, 12, 0, 0, 0,
	}
	matrix := mat.NewDense(1, 5, data)
	suite.Equal(got, matrix, "")
}
