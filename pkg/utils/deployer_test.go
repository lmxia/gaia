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
			WorkloadComponents: v1alpha1.WorkloadComponents{
				Scns: []v1alpha1.Scn{
					{
						Name:   "a",
						SelfID: []string{"sca1", "sca2"},
					},
					{
						Name:   "b",
						SelfID: []string{"scb1"},
					},
				},
				Links: []v1alpha1.Link{
					{
						LinkName:      "a-b",
						SourceID:      "sca1",
						DestinationID: "scb1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "sca1-dk1",
								Value: "sca1-dv1",
							},
							1: {
								Key:   "sca1-dk2",
								Value: "sca1-dv2",
							},
						},
						DestinationAttributes: []v1alpha1.Attributes{
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
					{
						LinkName:      "b-a",
						SourceID:      "scb1",
						DestinationID: "sca1",
					},
				},
			},
			Deployconditions: v1alpha1.DeploymentCondition{
				Mandatory: []v1alpha1.Condition{
					{
						Subject: v1alpha1.Xject{
							Name: "a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "sla",
							Type: "sla",
						},
						Relation: "Is",
						Extent: []byte("eyJkZWxheVZhbHVlIjoxMDAsImxvc3RWYWx1ZSI6MTAwLCJqaXR0ZXJWYWx1ZSI6OTk5OTk5L" +
							"CJ0aHJvdWdocHV0VmFsdWUiOjEwMH0= # {\"delayValue\":100,\"lostValue\":100,\"jitterValue\":" +
							"999999,\"throughputValue\":100}"),
					},
					{
						Subject: v1alpha1.Xject{
							Name: "b-a",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "sla",
							Type: "sla",
						},
						Relation: "Is",
						Extent: []byte("eyJkZWxheVZhbHVlIjoxMDAsImxvc3RWYWx1ZSI6MTAwLCJqaXR0ZXJWYWx1ZSI6OTk5OTk5LCJ" +
							"0aHJvdWdocHV0VmFsdWUiOjEwMH0= # {\"delayValue\":100,\"lostValue\":100,\"jitterValue\":" +
							"999999,\"throughputValue\":100}"),
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
			WorkloadComponents: v1alpha1.WorkloadComponents{
				Scns: []v1alpha1.Scn{
					{
						Name:   "a",
						SelfID: []string{"sca1", "sca2"},
					},
					{
						Name:   "b",
						SelfID: []string{"scb1"},
					},
				},
				Links: []v1alpha1.Link{
					{
						LinkName:      "a-b",
						SourceID:      "sca1",
						DestinationID: "scb1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "sca1-dk1",
								Value: "sca1-dv1",
							},
							1: {
								Key:   "sca1-dk2",
								Value: "sca1-dv2",
							},
						},
						DestinationAttributes: []v1alpha1.Attributes{
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
				},
			},
			Deployconditions: v1alpha1.DeploymentCondition{
				Mandatory: []v1alpha1.Condition{
					{
						Subject: v1alpha1.Xject{
							Name: "a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "sla",
							Type: "sla",
						},
						Relation: "Is",
						Extent: []byte("eyJkZWxheVZhbHVlIjoxMDAsImxvc3RWYWx1ZSI6MTAwLCJqaXR0ZXJWYWx1ZSI6OTk5OTk5LCJ0" +
							"aHJvdWdocHV0VmFsdWUiOjEwMH0= # {\"delayValue\":100,\"lostValue\":100,\"jitterValue\":" +
							"999999,\"throughputValue\":100}"),
					},
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
