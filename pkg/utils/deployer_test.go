package utils

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"github.com/stretchr/testify/suite"
)

type DeployerSuite struct {
	clustername string
	rb0         []*v1alpha1.ResourceBindingApps
	network1    *v1alpha1.NetworkRequirement
	network2    *v1alpha1.NetworkRequirement
	suite.Suite
}

func (suite *DeployerSuite) SetupTest() {
	suite.clustername = "field1"

	suite.rb0 = []*v1alpha1.ResourceBindingApps{
		0: {
			ClusterName: "field1",
			Replicas: map[string]int32{
				"a": 10,
				"b": 5,
			},
		},
		1: {
			ClusterName: "field2",
			Replicas: map[string]int32{
				"a": 5,
				"b": 0,
			},
		},
		2: {
			ClusterName: "field3",
			Replicas: map[string]int32{
				"a": 0,
				"b": 8,
			},
		},
	}

	suite.network1 = &v1alpha1.NetworkRequirement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "case4",
			Namespace: "default",
		},
		Spec: v1alpha1.NetworkRequirementSpec{
			NetworkCommunication: []v1alpha1.NetworkCommunication{
				0: {
					Name:   "a",
					SelfID: []string{"sca1", "sca2"},
					InterSCNID: []v1alpha1.InterSCNID{
						0: {
							Source: v1alpha1.Direction{
								Id: "sca1",
								Attributes: []v1alpha1.Attributes{
									0: {
										Key:   "sca1-dk1",
										Value: "sca1-dv1",
									},
									1: {
										Key:   "sca1-dk2",
										Value: "sca1-dv2",
									},
								},
							},
							Destination: v1alpha1.Direction{
								Id: "scb1",
								Attributes: []v1alpha1.Attributes{
									0: {
										Key:   "scb1-dk1",
										Value: "scb1-dv1",
									},
									1: {
										Key:   "scb1-dk2",
										Value: "scb1-dv2",
									},
								},
							},
							Sla: v1alpha1.AppSlaAttr{
								Delay:     10000,
								Lost:      10000,
								Jitter:    1000,
								Bandwidth: 6000,
							},
						},
					},
				},
				1: {
					Name:   "b",
					SelfID: []string{"scb1"},
					InterSCNID: []v1alpha1.InterSCNID{
						0: {
							Source: v1alpha1.Direction{
								Id: "scb1",
							},
							Destination: v1alpha1.Direction{
								Id: "sca1",
							},
							Sla: v1alpha1.AppSlaAttr{
								Delay:     10000,
								Lost:      10000,
								Jitter:    1000,
								Bandwidth: 6000,
							},
						},
					},
				},
			},
		},
	}

	suite.network2 = &v1alpha1.NetworkRequirement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "case4",
			Namespace: "default",
		},
		Spec: v1alpha1.NetworkRequirementSpec{
			NetworkCommunication: []v1alpha1.NetworkCommunication{
				0: {
					Name:   "a",
					SelfID: []string{"sca1", "sca2"},
					InterSCNID: []v1alpha1.InterSCNID{
						0: {
							Source: v1alpha1.Direction{
								Id: "sca1",
								Attributes: []v1alpha1.Attributes{
									0: {
										Key:   "sca1-dk1",
										Value: "sca1-dv1",
									},
									1: {
										Key:   "sca1-dk2",
										Value: "sca1-dv2",
									},
								},
							},
							Destination: v1alpha1.Direction{
								Id: "scb1",
								Attributes: []v1alpha1.Attributes{
									0: {
										Key:   "scb1-dk1",
										Value: "scb1-dv1",
									},
									1: {
										Key:   "scb1-dk2",
										Value: "scb1-dv2",
									},
								},
							},
							Sla: v1alpha1.AppSlaAttr{
								Delay:     10000,
								Lost:      10000,
								Jitter:    1000,
								Bandwidth: 6000,
							},
						},
					},
				},
				1: {
					Name:   "b",
					SelfID: []string{"scb1"},
				},
			},
		},
	}
}

func (suite *DeployerSuite) TestNeedBindNetworkInCluster() {
	result1 := NeedBindNetworkInCluster(suite.rb0, "field1", suite.network1)
	suite.Equal(true, result1, "test1")
	result2 := NeedBindNetworkInCluster(suite.rb0, "field2", suite.network1)
	suite.Equal(true, result2, "test2")
	result3 := NeedBindNetworkInCluster(suite.rb0, "field3", suite.network1)
	suite.Equal(true, result3, "test3")
}

func (suite *DeployerSuite) TestNeedBindNetworkInCluster2() {
	result1 := NeedBindNetworkInCluster(suite.rb0, "field1", suite.network2)
	suite.Equal(true, result1, "test1")
	result2 := NeedBindNetworkInCluster(suite.rb0, "field2", suite.network2)
	suite.Equal(true, result2, "test2")
	result3 := NeedBindNetworkInCluster(suite.rb0, "field3", suite.network2)
	suite.Equal(false, result3, "test3")
}

func TestDeployment(t *testing.T) {
	suite.Run(t, new(DeployerSuite))
}
