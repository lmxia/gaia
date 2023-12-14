package npcore

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/networkfilter/logx"
	ncsnp "github.com/lmxia/gaia/pkg/networkfilter/model"
	"github.com/lmxia/gaia/pkg/networkfilter/nputil"
)

func SetRbsAndNetReqAvailable() ([]*v1alpha1.ResourceBinding, *v1alpha1.NetworkRequirement) {
	var rbs []*v1alpha1.ResourceBinding
	rb0 := v1alpha1.ResourceBinding{
		Spec: v1alpha1.ResourceBindingSpec{
			AppID: "0",
			RbApps: []*v1alpha1.ResourceBindingApps{
				0: {
					ClusterName: "Domain1",
					Replicas: map[string]int32{
						"a": 2,
						"b": 0,
					},
				},
				1: {
					ClusterName: "Domain4",
					Replicas: map[string]int32{
						"a": 0,
						"b": 1,
					},
				},
				2: {
					ClusterName: "Domain3",
					Replicas: map[string]int32{
						"a": 0,
						"c": 2,
					},
				},
			},
		},
	}
	rbs = append(rbs, &rb0)
	rb1 := v1alpha1.ResourceBinding{
		Spec: v1alpha1.ResourceBindingSpec{
			AppID: "0",
			RbApps: []*v1alpha1.ResourceBindingApps{
				0: {
					ClusterName: "Domain1",
					Replicas: map[string]int32{
						"b": 2,
						"c": 1,
					},
				},
				1: {
					ClusterName: "Domain4",
					Replicas: map[string]int32{
						"b": 1,
						"c": 1,
					},
				},
				2: {
					ClusterName: "Domain5",
					Replicas: map[string]int32{
						"a": 1,
						"d": 2,
					},
				},
			},
		},
	}
	rbs = append(rbs, &rb1)

	networkReq := v1alpha1.NetworkRequirement{
		Spec: v1alpha1.NetworkRequirementSpec{
			WorkloadComponents: v1alpha1.WorkloadComponents{
				Scns: []v1alpha1.Scn{
					0: {
						Name:   "a",
						SelfID: []string{"sca1", "sca2"},
					},
					1: {
						Name:   "b",
						SelfID: []string{"scb1"},
					},
					2: {
						Name:   "c",
						SelfID: []string{"scc1"},
					},
				},

				Links: []v1alpha1.Link{
					0: {
						LinkName:      "link-a-b",
						SourceID:      "sca1",
						DestinationID: "scb1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "sca1-sk1",
								Value: "sca1-sv1",
							},
							1: {
								Key:   "sca1-sk2",
								Value: "sca1-sv2",
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
					1: {
						LinkName:      "link-a-c",
						SourceID:      "sca2",
						DestinationID: "scc1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "sca2-sk1",
								Value: "sca2-sv1",
							},
							1: {
								Key:   "sca2-sk2",
								Value: "sca2-sv2",
							},
						},
						DestinationAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scc1-dk1",
								Value: "scc1-dv1",
							},
							1: {
								Key:   "scc1-dk2",
								Value: "scc1-dv2",
							},
						},
					},
					2: {
						LinkName:      "link-b-c",
						SourceID:      "scb1",
						DestinationID: "scc1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scb1-sk1",
								Value: "scb1-sv1",
							},
							1: {
								Key:   "scb1-sk2",
								Value: "scb1-sv2",
							},
						},
						DestinationAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scc1-dk1",
								Value: "scc1-dv1",
							},
							1: {
								Key:   "scc1-dk2",
								Value: "scc1-dv2",
							},
						},
					},
				},
			},
			Deployconditions: v1alpha1.DeploymentCondition{
				Mandatory: []v1alpha1.Condition{
					0: {
						Subject: v1alpha1.Xject{
							Name: "link-a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "provider",
							Type: "label",
						},
						Relation: "In",
						Extent:   []byte("[\"Fabric12\"]"),
					},
					1: {
						Subject: v1alpha1.Xject{
							Name: "link-a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "sla",
							Type: "sla",
						},
						Relation: "Is",
						Extent:   []byte("{\"delay\":10000,\"lostValue\":100,\"jitterValue\":100,\"throughputValue\":100}"),
					},
					2: {
						Subject: v1alpha1.Xject{
							Name: "link-a-c",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "rtt",
							Type: "rtt",
						},
						Relation: "Is",
						Extent:   []byte("{\"rtt\":50}"),
					},
				},
				BestEffort: []v1alpha1.Condition{
					0: {
						Subject: v1alpha1.Xject{
							Name: "link-b-c",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "sla",
							Type: "sla",
						},
						Relation: "Is",
						Extent:   []byte("{\"delay\":10000,\"lost\":100,\"jitter\":100,\"bandwidth\":100}"),
					},
					1: {
						Subject: v1alpha1.Xject{
							Name: "link-b-c",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "accelerate",
							Type: "accelerate",
						},
						Relation: "Is",
						Extent:   []byte("{\"accelerate\": true}"),
					},
				},
			},
		},
	}
	return rbs, &networkReq
}

func SetRbsAndNetReqRtt() ([]*v1alpha1.ResourceBinding, *v1alpha1.NetworkRequirement) {
	var rbs []*v1alpha1.ResourceBinding
	rb0 := v1alpha1.ResourceBinding{
		Spec: v1alpha1.ResourceBindingSpec{
			AppID: "0",
			RbApps: []*v1alpha1.ResourceBindingApps{
				0: {
					ClusterName: "Domain1",
					Replicas: map[string]int32{
						"a": 2,
						"b": 0,
					},
				},
				1: {
					ClusterName: "Domain4",
					Replicas: map[string]int32{
						"a": 0,
						"b": 1,
					},
				},
				2: {
					ClusterName: "Domain3",
					Replicas: map[string]int32{
						"a": 0,
						"c": 2,
					},
				},
			},
		},
	}
	rbs = append(rbs, &rb0)

	networkReq := v1alpha1.NetworkRequirement{
		Spec: v1alpha1.NetworkRequirementSpec{
			WorkloadComponents: v1alpha1.WorkloadComponents{
				Scns: []v1alpha1.Scn{
					0: {
						Name:   "a",
						SelfID: []string{"sca1", "sca2"},
					},
					1: {
						Name:   "b",
						SelfID: []string{"scb1"},
					},
				},

				Links: []v1alpha1.Link{
					0: {
						LinkName:      "link-a-b",
						SourceID:      "sca1",
						DestinationID: "scb1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "sca1-sk1",
								Value: "sca1-sv1",
							},
							1: {
								Key:   "sca1-sk2",
								Value: "sca1-sv2",
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
					0: {
						Subject: v1alpha1.Xject{
							Name: "link-a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "rtt",
							Type: "rtt",
						},
						Relation: "Is",
						Extent:   []byte("{\"Rtt\":100}"),
					},
					1: {
						Subject: v1alpha1.Xject{
							Name: "link-a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "accelerate",
							Type: "accelerate",
						},
						Relation: "Is",
						Extent:   []byte("{\"Accelerate\": true}"),
					},
				},
			},
		},
	}
	return rbs, &networkReq
}

func SetRbsAndNetReqProviders() ([]*v1alpha1.ResourceBinding, *v1alpha1.NetworkRequirement) {
	var rbs []*v1alpha1.ResourceBinding
	rb0 := v1alpha1.ResourceBinding{
		Spec: v1alpha1.ResourceBindingSpec{
			AppID: "0",
			RbApps: []*v1alpha1.ResourceBindingApps{
				0: {
					ClusterName: "Domain1",
					Replicas: map[string]int32{
						"a": 2,
						"b": 0,
					},
				},
				1: {
					ClusterName: "Domain4",
					Replicas: map[string]int32{
						"a": 0,
						"b": 1,
					},
				},
				2: {
					ClusterName: "Domain3",
					Replicas: map[string]int32{
						"a": 0,
						"c": 2,
					},
				},
			},
		},
	}
	rbs = append(rbs, &rb0)

	networkReq := v1alpha1.NetworkRequirement{
		Spec: v1alpha1.NetworkRequirementSpec{
			WorkloadComponents: v1alpha1.WorkloadComponents{
				Scns: []v1alpha1.Scn{
					0: {
						Name:   "a",
						SelfID: []string{"sca1", "sca2"},
					},
					1: {
						Name:   "b",
						SelfID: []string{"scb1"},
					},
				},

				Links: []v1alpha1.Link{
					0: {
						LinkName:      "link-a-b",
						SourceID:      "sca1",
						DestinationID: "scb1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "sca1-sk1",
								Value: "sca1-sv1",
							},
							1: {
								Key:   "sca1-sk2",
								Value: "sca1-sv2",
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
					0: {
						Subject: v1alpha1.Xject{
							Name: "link-a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "provider",
							Type: "label",
						},
						Relation: "In",
						Extent:   []byte("[\"Fabric12\",\"Fabric13\",\"Fabric23\",\"Fabric34\"]"),
					},
				},
			},
		},
	}
	return rbs, &networkReq
}

func SetRbsAndNetReqProvidersNULL() ([]*v1alpha1.ResourceBinding, *v1alpha1.NetworkRequirement) {
	var rbs []*v1alpha1.ResourceBinding
	rb0 := v1alpha1.ResourceBinding{
		Spec: v1alpha1.ResourceBindingSpec{
			AppID: "0",
			RbApps: []*v1alpha1.ResourceBindingApps{
				0: {
					ClusterName: "Domain1",
					Replicas: map[string]int32{
						"a": 2,
						"b": 0,
					},
				},
				1: {
					ClusterName: "Domain4",
					Replicas: map[string]int32{
						"a": 0,
						"b": 1,
					},
				},
				2: {
					ClusterName: "Domain3",
					Replicas: map[string]int32{
						"a": 0,
						"c": 2,
					},
				},
			},
		},
	}
	rbs = append(rbs, &rb0)

	networkReq := v1alpha1.NetworkRequirement{
		Spec: v1alpha1.NetworkRequirementSpec{
			WorkloadComponents: v1alpha1.WorkloadComponents{
				Scns: []v1alpha1.Scn{
					0: {
						Name:   "a",
						SelfID: []string{"sca1", "sca2"},
					},
					1: {
						Name:   "b",
						SelfID: []string{"scb1"},
					},
				},

				Links: []v1alpha1.Link{
					0: {
						LinkName:      "link-a-b",
						SourceID:      "sca1",
						DestinationID: "scb1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "sca1-sk1",
								Value: "sca1-sv1",
							},
							1: {
								Key:   "sca1-sk2",
								Value: "sca1-sv2",
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
					0: {
						Subject: v1alpha1.Xject{
							Name: "link-a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "provider",
							Type: "label",
						},
						Relation: "In",
						Extent:   []byte("[]"),
					},
				},
			},
		},
	}
	return rbs, &networkReq
}

func SetRbsAndNetReqAccelerate() ([]*v1alpha1.ResourceBinding, *v1alpha1.NetworkRequirement) {
	var rbs []*v1alpha1.ResourceBinding
	rb0 := v1alpha1.ResourceBinding{
		Spec: v1alpha1.ResourceBindingSpec{
			AppID: "0",
			RbApps: []*v1alpha1.ResourceBindingApps{
				0: {
					ClusterName: "Domain1",
					Replicas: map[string]int32{
						"a": 2,
						"b": 0,
					},
				},
				1: {
					ClusterName: "Domain4",
					Replicas: map[string]int32{
						"a": 0,
						"b": 1,
					},
				},
				2: {
					ClusterName: "Domain3",
					Replicas: map[string]int32{
						"a": 0,
						"c": 2,
					},
				},
			},
		},
	}
	rbs = append(rbs, &rb0)

	networkReq := v1alpha1.NetworkRequirement{
		Spec: v1alpha1.NetworkRequirementSpec{
			WorkloadComponents: v1alpha1.WorkloadComponents{
				Scns: []v1alpha1.Scn{
					0: {
						Name:   "a",
						SelfID: []string{"sca1", "sca2"},
					},
					1: {
						Name:   "b",
						SelfID: []string{"scb1"},
					},
				},

				Links: []v1alpha1.Link{
					0: {
						LinkName:      "link-a-b",
						SourceID:      "sca1",
						DestinationID: "scb1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "sca1-sk1",
								Value: "sca1-sv1",
							},
							1: {
								Key:   "sca1-sk2",
								Value: "sca1-sv2",
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
					0: {
						Subject: v1alpha1.Xject{
							Name: "link-a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "accelerate",
							Type: "accelerate",
						},
						Relation: "Is",
						Extent:   []byte("{\"Accelerate\": true}"),
					},
				},
			},
		},
	}
	return rbs, &networkReq
}

func SetRbsAndNetReqBestEffort() ([]*v1alpha1.ResourceBinding, *v1alpha1.NetworkRequirement) {
	var rbs []*v1alpha1.ResourceBinding
	rb0 := v1alpha1.ResourceBinding{
		Spec: v1alpha1.ResourceBindingSpec{
			AppID: "0",
			RbApps: []*v1alpha1.ResourceBindingApps{
				0: {
					ClusterName: "Domain1",
					Replicas: map[string]int32{
						"a": 2,
						"b": 0,
					},
				},
				1: {
					ClusterName: "Domain4",
					Replicas: map[string]int32{
						"a": 0,
						"b": 1,
					},
				},
				2: {
					ClusterName: "Domain3",
					Replicas: map[string]int32{
						"a": 0,
						"c": 2,
					},
				},
			},
		},
	}
	rbs = append(rbs, &rb0)

	networkReq := v1alpha1.NetworkRequirement{
		Spec: v1alpha1.NetworkRequirementSpec{
			WorkloadComponents: v1alpha1.WorkloadComponents{
				Scns: []v1alpha1.Scn{
					0: {
						Name:   "a",
						SelfID: []string{"sca1", "sca2"},
					},
					1: {
						Name:   "b",
						SelfID: []string{"scb1"},
					},
					2: {
						Name:   "c",
						SelfID: []string{"scc1"},
					},
				},

				Links: []v1alpha1.Link{
					0: {
						LinkName:      "link-a-b",
						SourceID:      "sca1",
						DestinationID: "scb1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "sca1-sk1",
								Value: "sca1-sv1",
							},
							1: {
								Key:   "sca1-sk2",
								Value: "sca1-sv2",
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
					1: {
						LinkName:      "link-a-c",
						SourceID:      "sca2",
						DestinationID: "scc1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "sca2-sk1",
								Value: "sca2-sv1",
							},
							1: {
								Key:   "sca2-sk2",
								Value: "sca2-sv2",
							},
						},
						DestinationAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scc1-dk1",
								Value: "scc1-dv1",
							},
							1: {
								Key:   "scc1-dk2",
								Value: "scc1-dv2",
							},
						},
					},
					2: {
						LinkName:      "link-b-c",
						SourceID:      "scb1",
						DestinationID: "scc1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scb1-sk1",
								Value: "scb1-sv1",
							},
							1: {
								Key:   "scb1-sk2",
								Value: "scb1-sv2",
							},
						},
						DestinationAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scc1-dk1",
								Value: "scc1-dv1",
							},
							1: {
								Key:   "scc1-dk2",
								Value: "scc1-dv2",
							},
						},
					},
				},
			},
			Deployconditions: v1alpha1.DeploymentCondition{
				BestEffort: []v1alpha1.Condition{
					0: {
						Subject: v1alpha1.Xject{
							Name: "link-a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "provider",
							Type: "label",
						},
						Relation: "In",
						Extent:   []byte("[\"Fabric12\"]"),
					},
					1: {
						Subject: v1alpha1.Xject{
							Name: "link-a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "sla",
							Type: "sla",
						},
						Relation: "Is",
						Extent:   []byte("{\"delay\":1,\"lost\":100,\"jitter\":100,\"bandwidth\":100}"),
					},
					2: {
						Subject: v1alpha1.Xject{
							Name: "link-a-c",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "rtt",
							Type: "rtt",
						},
						Relation: "Is",
						Extent:   []byte("{\"Rtt\":20}"),
					},
					3: {
						Subject: v1alpha1.Xject{
							Name: "link-a-c",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "accelerate",
							Type: "accelerate",
						},
						Relation: "Is",
						Extent:   []byte("{\"Accelerate\": true}"),
					},
				},
			},
		},
	}
	return rbs, &networkReq
}

func SetRbsAndNetReqSameDomain() ([]*v1alpha1.ResourceBinding, *v1alpha1.NetworkRequirement) {
	var rbs []*v1alpha1.ResourceBinding
	rb0 := v1alpha1.ResourceBinding{
		Spec: v1alpha1.ResourceBindingSpec{
			AppID: "0",
			RbApps: []*v1alpha1.ResourceBindingApps{
				0: {
					ClusterName: "Domain1",
					Replicas: map[string]int32{
						"a": 2,
						"b": 1,
					},
				},
			},
		},
	}
	rbs = append(rbs, &rb0)

	networkReq := v1alpha1.NetworkRequirement{
		Spec: v1alpha1.NetworkRequirementSpec{
			WorkloadComponents: v1alpha1.WorkloadComponents{
				Scns: []v1alpha1.Scn{
					0: {
						Name:   "a",
						SelfID: []string{"sca1", "sca2"},
					},
					1: {
						Name:   "b",
						SelfID: []string{"scb1"},
					},
				},

				Links: []v1alpha1.Link{
					0: {
						LinkName:      "link-a-b",
						SourceID:      "sca1",
						DestinationID: "scb1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "sca1-sk1",
								Value: "sca1-sv1",
							},
							1: {
								Key:   "sca1-sk2",
								Value: "sca1-sv2",
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
					0: {
						Subject: v1alpha1.Xject{
							Name: "link-a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "sla",
							Type: "sla",
						},
						Relation: "Is",
						Extent:   []byte("{\"delay\":10000,\"lost\":100,\"jitter\":100,\"bandwidth\":100}"),
					},
				},
			},
		},
	}

	return rbs, &networkReq
}

func SetRbsAndNetReqTopoFailed() ([]*v1alpha1.ResourceBinding, *v1alpha1.NetworkRequirement) {
	var rbs []*v1alpha1.ResourceBinding
	rb0 := v1alpha1.ResourceBinding{
		Spec: v1alpha1.ResourceBindingSpec{
			AppID: "0",
			RbApps: []*v1alpha1.ResourceBindingApps{
				0: {
					ClusterName: "Domain1",
					Replicas: map[string]int32{
						"a": 2,
						"b": 0,
					},
				},
				1: {
					ClusterName: "Domain4",
					Replicas: map[string]int32{
						"a": 0,
						"b": 1,
					},
				},
				2: {
					ClusterName: "Domain5",
					Replicas: map[string]int32{
						"a": 0,
						"c": 2,
					},
				},
			},
		},
	}
	rbs = append(rbs, &rb0)

	networkReq := v1alpha1.NetworkRequirement{
		Spec: v1alpha1.NetworkRequirementSpec{
			WorkloadComponents: v1alpha1.WorkloadComponents{
				Scns: []v1alpha1.Scn{
					0: {
						Name:   "a",
						SelfID: []string{"sca1", "sca2"},
					},
					1: {
						Name:   "b",
						SelfID: []string{"scb1"},
					},
					2: {
						Name:   "c",
						SelfID: []string{"scc1"},
					},
				},

				Links: []v1alpha1.Link{
					0: {
						LinkName:      "link-a-b",
						SourceID:      "sca1",
						DestinationID: "scb1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "sca1-sk1",
								Value: "sca1-sv1",
							},
							1: {
								Key:   "sca1-sk2",
								Value: "sca1-sv2",
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
					1: {
						LinkName:      "link-b-c",
						SourceID:      "scb1",
						DestinationID: "scc1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scb1-sk1",
								Value: "scb1-sv1",
							},
							1: {
								Key:   "scb1-sk2",
								Value: "scb1-sv2",
							},
						},
						DestinationAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scc1-dk1",
								Value: "scc1-dv1",
							},
							1: {
								Key:   "scc1-dk2",
								Value: "scc1-dv2",
							},
						},
					},
				},
			},
			Deployconditions: v1alpha1.DeploymentCondition{
				Mandatory: []v1alpha1.Condition{
					0: {
						Subject: v1alpha1.Xject{
							Name: "link-a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "sla",
							Type: "sla",
						},
						Relation: "Is",
						Extent:   []byte("{\"delay\":10000,\"lost\":100,\"jitter\":100,\"bandwidth\":100}"),
					},
				},
				BestEffort: []v1alpha1.Condition{
					0: {
						Subject: v1alpha1.Xject{
							Name: "link-b-c",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "sla",
							Type: "sla",
						},
						Relation: "Is",
						Extent:   []byte("{\"delay\":10000,\"lost\":100,\"jitter\":100,\"bandwidth\":100}"),
					},
				},
			},
		},
	}

	return rbs, &networkReq
}

func SetRbsAndNetReqDelaySlaFailed() ([]*v1alpha1.ResourceBinding, *v1alpha1.NetworkRequirement) {
	var rbs []*v1alpha1.ResourceBinding
	rb0 := v1alpha1.ResourceBinding{
		Spec: v1alpha1.ResourceBindingSpec{
			AppID: "0",
			RbApps: []*v1alpha1.ResourceBindingApps{
				0: {
					ClusterName: "Domain1",
					Replicas: map[string]int32{
						"a": 2,
						"b": 0,
					},
				},
				1: {
					ClusterName: "Domain2",
					Replicas: map[string]int32{
						"a": 0,
						"b": 1,
					},
				},
				2: {
					ClusterName: "Domain4",
					Replicas: map[string]int32{
						"a": 0,
						"c": 2,
					},
				},
			},
		},
	}
	rbs = append(rbs, &rb0)

	networkReq := v1alpha1.NetworkRequirement{
		Spec: v1alpha1.NetworkRequirementSpec{
			WorkloadComponents: v1alpha1.WorkloadComponents{
				Scns: []v1alpha1.Scn{
					0: {
						Name:   "a",
						SelfID: []string{"sca1", "sca2"},
					},
					1: {
						Name:   "b",
						SelfID: []string{"scb1"},
					},
					2: {
						Name:   "c",
						SelfID: []string{"scc1"},
					},
				},

				Links: []v1alpha1.Link{
					0: {
						LinkName:      "link-a-b",
						SourceID:      "sca1",
						DestinationID: "scb1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "sca1-sk1",
								Value: "sca1-sv1",
							},
							1: {
								Key:   "sca1-sk2",
								Value: "sca1-sv2",
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
					1: {
						LinkName:      "link-b-c",
						SourceID:      "scb1",
						DestinationID: "scc1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scb1-sk1",
								Value: "scb1-sv1",
							},
							1: {
								Key:   "scb1-sk2",
								Value: "scb1-sv2",
							},
						},
						DestinationAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scc1-dk1",
								Value: "scc1-dv1",
							},
							1: {
								Key:   "scc1-dk2",
								Value: "scc1-dv2",
							},
						},
					},
				},
			},
			Deployconditions: v1alpha1.DeploymentCondition{
				Mandatory: []v1alpha1.Condition{
					0: {
						Subject: v1alpha1.Xject{
							Name: "link-a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "sla",
							Type: "sla",
						},
						Relation: "Is",
						Extent:   []byte("{\"delay\":2,\"lost\":100,\"jitter\":100,\"bandwidth\":100}"),
					},
					2: {
						Subject: v1alpha1.Xject{
							Name: "link-a-c",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "rtt",
							Type: "rtt",
						},
						Relation: "Is",
						Extent:   []byte("{\"Rtt\":50}"),
					},
				},
			},
		},
	}

	return rbs, &networkReq
}

func SetRbsAndNetReqThroughputSla() ([]*v1alpha1.ResourceBinding, *v1alpha1.NetworkRequirement) {
	var rbs []*v1alpha1.ResourceBinding
	rb0 := v1alpha1.ResourceBinding{
		Spec: v1alpha1.ResourceBindingSpec{
			AppID: "0",
			RbApps: []*v1alpha1.ResourceBindingApps{
				0: {
					ClusterName: "Domain1",
					Replicas: map[string]int32{
						"a": 2,
						"b": 0,
					},
				},
				1: {
					ClusterName: "Domain4",
					Replicas: map[string]int32{
						"a": 0,
						"b": 1,
					},
				},
			},
		},
	}
	rbs = append(rbs, &rb0)

	networkReq := v1alpha1.NetworkRequirement{
		Spec: v1alpha1.NetworkRequirementSpec{
			WorkloadComponents: v1alpha1.WorkloadComponents{
				Scns: []v1alpha1.Scn{
					0: {
						Name:   "a",
						SelfID: []string{"sca1", "sca2"},
					},
					1: {
						Name:   "b",
						SelfID: []string{"scb1"},
					},
					2: {
						Name:   "c",
						SelfID: []string{"scc1"},
					},
				},

				Links: []v1alpha1.Link{
					0: {
						LinkName:      "link-a-b",
						SourceID:      "sca1",
						DestinationID: "scb1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "sca1-sk1",
								Value: "sca1-sv1",
							},
							1: {
								Key:   "sca1-sk2",
								Value: "sca1-sv2",
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
					1: {
						LinkName:      "link-a-c",
						SourceID:      "sca2",
						DestinationID: "scc1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "sca2-sk1",
								Value: "sca2-sv1",
							},
							1: {
								Key:   "sca2-sk2",
								Value: "sca2-sv2",
							},
						},
						DestinationAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scc1-dk1",
								Value: "scc1-dv1",
							},
							1: {
								Key:   "scc1-dk2",
								Value: "scc1-dv2",
							},
						},
					},
					2: {
						LinkName:      "link-b-c",
						SourceID:      "scb1",
						DestinationID: "scc1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scb1-sk1",
								Value: "scb1-sv1",
							},
							1: {
								Key:   "scb1-sk2",
								Value: "scb1-sv2",
							},
						},
						DestinationAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scc1-dk1",
								Value: "scc1-dv1",
							},
							1: {
								Key:   "scc1-dk2",
								Value: "scc1-dv2",
							},
						},
					},
				},
			},
			Deployconditions: v1alpha1.DeploymentCondition{
				Mandatory: []v1alpha1.Condition{
					0: {
						Subject: v1alpha1.Xject{
							Name: "link-a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "provider",
							Type: "label",
						},
						Relation: "In",
						Extent:   []byte("[\"Fabric12\"]"),
					},
					1: {
						Subject: v1alpha1.Xject{
							Name: "link-a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "sla",
							Type: "sla",
						},
						Relation: "Is",
						Extent:   []byte("{\"delay\":10000,\"lost\":100,\"jitter\":100,\"bandwidth\":100}"),
					},
					2: {
						Subject: v1alpha1.Xject{
							Name: "link-a-c",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "rtt",
							Type: "rtt",
						},
						Relation: "Is",
						Extent:   []byte("{\"Rtt\":50}"),
					},
				},
				BestEffort: []v1alpha1.Condition{
					0: {
						Subject: v1alpha1.Xject{
							Name: "link-b-c",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "sla",
							Type: "sla",
						},
						Relation: "Is",
						Extent:   []byte("{\"delay\":10000,\"lost\":100,\"jitter\":100,\"bandwidth\":100}"),
					},
					1: {
						Subject: v1alpha1.Xject{
							Name: "link-b-c",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "accelerate",
							Type: "accelerate",
						},
						Relation: "Is",
						Extent:   []byte("{\"Accelerate\": true}"),
					},
				},
			},
		},
	}

	return rbs, &networkReq
}

func SetRbsAndNetReqThroughputSlaFailed() ([]*v1alpha1.ResourceBinding, *v1alpha1.NetworkRequirement) {
	var rbs []*v1alpha1.ResourceBinding
	rb0 := v1alpha1.ResourceBinding{
		Spec: v1alpha1.ResourceBindingSpec{
			AppID: "0",
			RbApps: []*v1alpha1.ResourceBindingApps{
				0: {
					ClusterName: "Domain1",
					Replicas: map[string]int32{
						"a": 2,
						"b": 0,
					},
				},
				1: {
					ClusterName: "Domain4",
					Replicas: map[string]int32{
						"a": 0,
						"b": 1,
					},
				},
			},
		},
	}
	rbs = append(rbs, &rb0)

	networkReq := v1alpha1.NetworkRequirement{
		Spec: v1alpha1.NetworkRequirementSpec{
			WorkloadComponents: v1alpha1.WorkloadComponents{
				Scns: []v1alpha1.Scn{
					0: {
						Name:   "a",
						SelfID: []string{"sca1", "sca2"},
					},
					1: {
						Name:   "b",
						SelfID: []string{"scb1"},
					},
					2: {
						Name:   "c",
						SelfID: []string{"scc1"},
					},
				},

				Links: []v1alpha1.Link{
					0: {
						LinkName:      "link-a-b",
						SourceID:      "sca1",
						DestinationID: "scb1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "sca1-sk1",
								Value: "sca1-sv1",
							},
							1: {
								Key:   "sca1-sk2",
								Value: "sca1-sv2",
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
					1: {
						LinkName:      "link-a-c",
						SourceID:      "sca2",
						DestinationID: "scc1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "sca2-sk1",
								Value: "sca2-sv1",
							},
							1: {
								Key:   "sca2-sk2",
								Value: "sca2-sv2",
							},
						},
						DestinationAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scc1-dk1",
								Value: "scc1-dv1",
							},
							1: {
								Key:   "scc1-dk2",
								Value: "scc1-dv2",
							},
						},
					},
					2: {
						LinkName:      "link-b-c",
						SourceID:      "scb1",
						DestinationID: "scc1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scb1-sk1",
								Value: "scb1-sv1",
							},
							1: {
								Key:   "scb1-sk2",
								Value: "scb1-sv2",
							},
						},
						DestinationAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scc1-dk1",
								Value: "scc1-dv1",
							},
							1: {
								Key:   "scc1-dk2",
								Value: "scc1-dv2",
							},
						},
					},
				},
			},
			Deployconditions: v1alpha1.DeploymentCondition{
				Mandatory: []v1alpha1.Condition{
					0: {
						Subject: v1alpha1.Xject{
							Name: "link-a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "provider",
							Type: "label",
						},
						Relation: "In",
						Extent:   []byte("[\"Fabric12\"]"),
					},
					1: {
						Subject: v1alpha1.Xject{
							Name: "link-a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "sla",
							Type: "sla",
						},
						Relation: "Is",
						Extent:   []byte("{\"delay\":10000,\"lost\":100,\"jitter\":100,\"bandwidth\":100}"),
					},
					2: {
						Subject: v1alpha1.Xject{
							Name: "link-a-c",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "rtt",
							Type: "rtt",
						},
						Relation: "Is",
						Extent:   []byte("{\"Rtt\":50}"),
					},
				},
				BestEffort: []v1alpha1.Condition{
					0: {
						Subject: v1alpha1.Xject{
							Name: "link-b-c",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "sla",
							Type: "sla",
						},
						Relation: "Is",
						Extent:   []byte("{\"delay\":10000,\"lost\":100,\"jitter\":100,\"bandwidth\":100}"),
					},
					1: {
						Subject: v1alpha1.Xject{
							Name: "link-b-c",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "accelerate",
							Type: "accelerate",
						},
						Relation: "Is",
						Extent:   []byte("{\"Accelerate\": true}"),
					},
				},
			},
		},
	}

	return rbs, &networkReq
}

func SetRbsAndNetReqNoInterCommunication() ([]*v1alpha1.ResourceBinding, *v1alpha1.NetworkRequirement) {
	var rbs []*v1alpha1.ResourceBinding
	rb0 := v1alpha1.ResourceBinding{
		Spec: v1alpha1.ResourceBindingSpec{
			AppID: "0",
			RbApps: []*v1alpha1.ResourceBindingApps{
				0: {
					ClusterName: "Domain1",
					Replicas: map[string]int32{
						"a": 2,
						"b": 0,
					},
				},
				1: {
					ClusterName: "Domain4",
					Replicas: map[string]int32{
						"a": 0,
						"b": 1,
					},
				},
				2: {
					ClusterName: "Domain5",
					Replicas: map[string]int32{
						"a": 0,
						"c": 2,
					},
				},
			},
		},
	}
	rbs = append(rbs, &rb0)

	networkReq := v1alpha1.NetworkRequirement{
		Spec: v1alpha1.NetworkRequirementSpec{
			WorkloadComponents: v1alpha1.WorkloadComponents{
				Scns: []v1alpha1.Scn{
					0: {
						Name:   "a",
						SelfID: []string{"sca1", "sca2"},
					},
					1: {
						Name:   "b",
						SelfID: []string{"scb1"},
					},
					2: {
						Name:   "c",
						SelfID: []string{"scc1"},
					},
				},

				Links: []v1alpha1.Link{
					0: {
						LinkName:      "link-a-b",
						SourceID:      "sca1",
						DestinationID: "scb1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "sca1-sk1",
								Value: "sca1-sv1",
							},
							1: {
								Key:   "sca1-sk2",
								Value: "sca1-sv2",
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
					1: {
						LinkName:      "link-a-c",
						SourceID:      "sca2",
						DestinationID: "scc1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "sca2-sk1",
								Value: "sca2-sv1",
							},
							1: {
								Key:   "sca2-sk2",
								Value: "sca2-sv2",
							},
						},
						DestinationAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scc1-dk1",
								Value: "scc1-dv1",
							},
							1: {
								Key:   "scc1-dk2",
								Value: "scc1-dv2",
							},
						},
					},
					2: {
						LinkName:      "link-b-c",
						SourceID:      "scb1",
						DestinationID: "scc1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scb1-sk1",
								Value: "scb1-sv1",
							},
							1: {
								Key:   "scb1-sk2",
								Value: "scb1-sv2",
							},
						},
						DestinationAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scc1-dk1",
								Value: "scc1-dv1",
							},
							1: {
								Key:   "scc1-dk2",
								Value: "scc1-dv2",
							},
						},
					},
				},
			},
			Deployconditions: v1alpha1.DeploymentCondition{
				Mandatory: []v1alpha1.Condition{
					0: {
						Subject: v1alpha1.Xject{
							Name: "link-a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "provider",
							Type: "label",
						},
						Relation: "In",
						Extent:   []byte("[\"Fabric12\"]"),
					},
					1: {
						Subject: v1alpha1.Xject{
							Name: "link-a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "sla",
							Type: "sla",
						},
						Relation: "Is",
						Extent:   []byte("{\"delay\":10000,\"lost\":100,\"jitter\":100,\"bandwidth\":100}"),
					},
					2: {
						Subject: v1alpha1.Xject{
							Name: "link-a-c",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "rtt",
							Type: "rtt",
						},
						Relation: "Is",
						Extent:   []byte("{\"Rtt\":50}"),
					},
				},
				BestEffort: []v1alpha1.Condition{
					0: {
						Subject: v1alpha1.Xject{
							Name: "link-b-c",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "sla",
							Type: "sla",
						},
						Relation: "Is",
						Extent:   []byte("{\"delay\":10000,\"lost\":100,\"jitter\":100,\"bandwidth\":100}"),
					},
					1: {
						Subject: v1alpha1.Xject{
							Name: "link-b-c",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "accelerate",
							Type: "accelerate",
						},
						Relation: "Is",
						Extent:   []byte("{\"Accelerate\": true}"),
					},
				},
			},
		},
	}

	return rbs, &networkReq
}

// Case 5
func SetRbsAndNetReqInterCommunication() ([]*v1alpha1.ResourceBinding, *v1alpha1.NetworkRequirement) {
	var rbs []*v1alpha1.ResourceBinding
	rb0 := v1alpha1.ResourceBinding{
		Spec: v1alpha1.ResourceBindingSpec{
			AppID: "0",
			RbApps: []*v1alpha1.ResourceBindingApps{
				0: {
					ClusterName: "Domain1",
					Replicas: map[string]int32{
						"a": 2,
						"b": 0,
					},
				},
				1: {
					ClusterName: "Domain4",
					Replicas: map[string]int32{
						"a": 0,
						"b": 1,
					},
				},
			},
		},
	}
	rbs = append(rbs, &rb0)

	networkReq := v1alpha1.NetworkRequirement{
		Spec: v1alpha1.NetworkRequirementSpec{
			WorkloadComponents: v1alpha1.WorkloadComponents{
				Scns: []v1alpha1.Scn{
					0: {
						Name:   "a",
						SelfID: []string{"sca1", "sca2"},
					},
					1: {
						Name:   "b",
						SelfID: []string{"scb1"},
					},
					2: {
						Name:   "c",
						SelfID: []string{"scc1"},
					},
				},

				Links: []v1alpha1.Link{
					0: {
						LinkName:      "link-a-b",
						SourceID:      "sca1",
						DestinationID: "scb1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "sca1-sk1",
								Value: "sca1-sv1",
							},
							1: {
								Key:   "sca1-sk2",
								Value: "sca1-sv2",
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
					1: {
						LinkName:      "link-a-c",
						SourceID:      "sca2",
						DestinationID: "scc1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "sca2-sk1",
								Value: "sca2-sv1",
							},
							1: {
								Key:   "sca2-sk2",
								Value: "sca2-sv2",
							},
						},
						DestinationAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scc1-dk1",
								Value: "scc1-dv1",
							},
							1: {
								Key:   "scc1-dk2",
								Value: "scc1-dv2",
							},
						},
					},
					2: {
						LinkName:      "link-b-c",
						SourceID:      "scb1",
						DestinationID: "scc1",
						SourceAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scb1-sk1",
								Value: "scb1-sv1",
							},
							1: {
								Key:   "scb1-sk2",
								Value: "scb1-sv2",
							},
						},
						DestinationAttributes: []v1alpha1.Attributes{
							0: {
								Key:   "scc1-dk1",
								Value: "scc1-dv1",
							},
							1: {
								Key:   "scc1-dk2",
								Value: "scc1-dv2",
							},
						},
					},
				},
			},
			Deployconditions: v1alpha1.DeploymentCondition{
				Mandatory: []v1alpha1.Condition{
					0: {
						Subject: v1alpha1.Xject{
							Name: "link-a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "provider",
							Type: "label",
						},
						Relation: "In",
						Extent:   []byte("[\"Fabric12\"]"),
					},
					1: {
						Subject: v1alpha1.Xject{
							Name: "link-a-b",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "sla",
							Type: "sla",
						},
						Relation: "Is",
						Extent:   []byte("{\"delay\":10000,\"lost\":100,\"jitter\":100,\"bandwidth\":100}"),
					},
					2: {
						Subject: v1alpha1.Xject{
							Name: "link-a-c",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "rtt",
							Type: "rtt",
						},
						Relation: "Is",
						Extent:   []byte("{\"Rtt\":50}"),
					},
				},
				BestEffort: []v1alpha1.Condition{
					0: {
						Subject: v1alpha1.Xject{
							Name: "link-b-c",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "sla",
							Type: "sla",
						},
						Relation: "Is",
						Extent:   []byte("{\"delay\":10000,\"lost\":100,\"jitter\":100,\"bandwidth\":100}"),
					},
					1: {
						Subject: v1alpha1.Xject{
							Name: "link-b-c",
							Type: "link",
						},
						Object: v1alpha1.Xject{
							Name: "accelerate",
							Type: "accelerate",
						},
						Relation: "Is",
						Extent:   []byte("{\"Accelerate\": true}"),
					},
				},
			},
		},
	}

	return rbs, &networkReq
}

func BuildNetworkDomainEdge() map[string]clusterapi.Topo {
	nputil.TraceInfoBegin("------------------------------------------------------")

	var domainTopoCacheArry []*ncsnp.DomainTopoCacheNotify
	domainTopoMsg := make(map[string]clusterapi.Topo)
	topoInfo := clusterapi.Topo{}

	domainTopoCache1 := new(ncsnp.DomainTopoCacheNotify)
	domainTopoCache1.LocalDomainId = 1
	domainTopoCache1.LocalDomainName = "Domain1"
	domainTopoCache1.LocalNodeSN = "1-1"
	domainVLink12 := new(ncsnp.DomainVLink)
	domainVLink12.LocalDomainName = "Domain1"
	domainVLink12.LocalDomainId = 1
	domainVLink12.RemoteDomainName = "Domain2"
	domainVLink12.RemoteDomainId = 2
	domainVLink12.LocalNodeSN = "Node12"
	domainVLink12.RemoteNodeSN = "Node21"
	domainVLink12.AttachDomainId = 1012
	domainVLink12.AttachDomainName = "Fabric12"
	vLinkSlaAttr12 := new(ncsnp.VLinkSla)
	vLinkSlaAttr12.Delay = 1
	vLinkSlaAttr12.Bandwidth = 15000
	vLinkSlaAttr12.FreeBandwidth = 15000
	domainVLink12.VLinkSlaAttr = vLinkSlaAttr12
	domainTopoCache1.DomainVLinkArray = append(domainTopoCache1.DomainVLinkArray, domainVLink12)
	domainVLink13 := new(ncsnp.DomainVLink)
	domainVLink13.LocalDomainName = "Domain1"
	domainVLink13.LocalDomainId = 1
	domainVLink13.RemoteDomainName = "Domain3"
	domainVLink13.RemoteDomainId = 3
	domainVLink13.LocalNodeSN = "Node13"
	domainVLink13.RemoteNodeSN = "Node31"
	domainVLink13.AttachDomainId = 1013
	domainVLink13.AttachDomainName = "Fabric13"
	vLinkSlaAttr13 := new(ncsnp.VLinkSla)
	vLinkSlaAttr13.Delay = 1
	vLinkSlaAttr13.Bandwidth = 10000
	vLinkSlaAttr13.FreeBandwidth = 10000
	domainVLink13.VLinkSlaAttr = vLinkSlaAttr13
	domainVLink13.OpaqueValue = ""
	domainTopoCache1.DomainVLinkArray = append(domainTopoCache1.DomainVLinkArray, domainVLink13)
	domainTopoCacheArry = append(domainTopoCacheArry, domainTopoCache1)
	content, err := proto.Marshal(domainTopoCache1)
	if err != nil {
		nputil.TraceErrorWithStack(err)
		return nil
	}
	topoInfo.Field = domainTopoCache1.LocalDomainName
	topoInfo.Content = base64.StdEncoding.EncodeToString(content)
	domainTopoMsg[domainTopoCache1.LocalDomainName] = topoInfo

	topoInfo = clusterapi.Topo{}
	domainTopoCache2 := new(ncsnp.DomainTopoCacheNotify)
	domainTopoCache2.LocalDomainId = 2
	domainTopoCache2.LocalDomainName = "Domain2"
	domainTopoCache2.LocalNodeSN = "2-1"
	domainVLink23 := new(ncsnp.DomainVLink)
	domainVLink23.LocalDomainName = "Domain2"
	domainVLink23.LocalDomainId = 2
	domainVLink23.RemoteDomainName = "Domain3"
	domainVLink23.RemoteDomainId = 3
	domainVLink23.LocalNodeSN = "Node23"
	domainVLink23.RemoteNodeSN = "Node32"
	domainVLink23.AttachDomainId = 1023
	domainVLink23.AttachDomainName = "Fabric23"
	vLinkSlaAttr23 := new(ncsnp.VLinkSla)
	vLinkSlaAttr23.Delay = 2
	vLinkSlaAttr23.Bandwidth = 15000
	vLinkSlaAttr23.FreeBandwidth = 15000
	domainVLink23.VLinkSlaAttr = vLinkSlaAttr23
	domainTopoCache2.DomainVLinkArray = append(domainTopoCache2.DomainVLinkArray, domainVLink23)

	domainVLink21 := new(ncsnp.DomainVLink)
	domainVLink21.LocalDomainName = "Domain2"
	domainVLink21.LocalDomainId = 2
	domainVLink21.RemoteDomainName = "Domain1"
	domainVLink21.RemoteDomainId = 1
	domainVLink21.LocalNodeSN = "Node21"
	domainVLink21.RemoteNodeSN = "Node12"
	domainVLink21.AttachDomainId = 1012
	domainVLink21.AttachDomainName = "Fabric12"
	vLinkSlaAttr21 := new(ncsnp.VLinkSla)
	vLinkSlaAttr21.Delay = 1
	vLinkSlaAttr21.Bandwidth = 15000
	vLinkSlaAttr21.FreeBandwidth = 15000
	domainVLink21.VLinkSlaAttr = vLinkSlaAttr21
	domainTopoCache2.DomainVLinkArray = append(domainTopoCache2.DomainVLinkArray, domainVLink21)
	domainTopoCacheArry = append(domainTopoCacheArry, domainTopoCache2)
	content, err = proto.Marshal(domainTopoCache2)
	if err != nil {
		nputil.TraceErrorWithStack(err)
		return nil
	}
	topoInfo.Field = domainTopoCache2.LocalDomainName
	topoInfo.Content = base64.StdEncoding.EncodeToString(content)
	domainTopoMsg[domainTopoCache2.LocalDomainName] = topoInfo

	topoInfo = clusterapi.Topo{}
	domainTopoCache3 := new(ncsnp.DomainTopoCacheNotify)
	domainTopoCache3.LocalDomainId = 3
	domainTopoCache3.LocalDomainName = "Domain3"
	domainTopoCache3.LocalNodeSN = "3-1"
	domainVLink34 := new(ncsnp.DomainVLink)
	domainVLink34.LocalDomainName = "Domain3"
	domainVLink34.LocalDomainId = 3
	domainVLink34.RemoteDomainName = "Domain4"
	domainVLink34.RemoteDomainId = 4
	domainVLink34.LocalNodeSN = "Node34"
	domainVLink34.RemoteNodeSN = "Node43"
	domainVLink34.AttachDomainId = 1034
	domainVLink34.AttachDomainName = "Fabric34"
	vLinkSlaAttr34 := new(ncsnp.VLinkSla)
	vLinkSlaAttr34.Delay = 3
	vLinkSlaAttr34.Bandwidth = 15000
	vLinkSlaAttr34.FreeBandwidth = 15000
	domainVLink34.VLinkSlaAttr = vLinkSlaAttr34
	domainTopoCache3.DomainVLinkArray = append(domainTopoCache3.DomainVLinkArray, domainVLink34)
	domainVLink32 := new(ncsnp.DomainVLink)
	domainVLink32.LocalDomainName = "Domain3"
	domainVLink32.LocalDomainId = 3
	domainVLink32.RemoteDomainName = "Domain2"
	domainVLink32.RemoteDomainId = 2
	domainVLink32.LocalNodeSN = "Node32"
	domainVLink32.RemoteNodeSN = "Node23"
	domainVLink32.AttachDomainId = 1023
	domainVLink32.AttachDomainName = "Fabric23"
	vLinkSlaAttr32 := new(ncsnp.VLinkSla)
	vLinkSlaAttr32.Delay = 2
	vLinkSlaAttr32.Bandwidth = 15000
	vLinkSlaAttr32.FreeBandwidth = 15000
	domainVLink32.VLinkSlaAttr = vLinkSlaAttr32
	domainTopoCache3.DomainVLinkArray = append(domainTopoCache3.DomainVLinkArray, domainVLink32)
	domainTopoCacheArry = append(domainTopoCacheArry, domainTopoCache3)
	content, err = proto.Marshal(domainTopoCache3)
	if err != nil {
		nputil.TraceErrorWithStack(err)
		return nil
	}
	topoInfo.Field = domainTopoCache3.LocalDomainName
	topoInfo.Content = base64.StdEncoding.EncodeToString(content)
	domainTopoMsg[domainTopoCache3.LocalDomainName] = topoInfo

	topoInfo = clusterapi.Topo{}
	domainTopoCache4 := new(ncsnp.DomainTopoCacheNotify)
	domainTopoCache4.LocalDomainId = 4
	domainTopoCache4.LocalDomainName = "Domain4"
	domainTopoCache4.LocalNodeSN = "Node43"
	domainVLink43 := new(ncsnp.DomainVLink)
	domainVLink43.LocalDomainName = "Domain4"
	domainVLink43.LocalDomainId = 4
	domainVLink43.RemoteDomainName = "Domain3"
	domainVLink43.RemoteDomainId = 3
	domainVLink43.LocalNodeSN = "Node43"
	domainVLink43.RemoteNodeSN = "Node34"
	domainVLink43.AttachDomainId = 1034
	domainVLink43.AttachDomainName = "Fabric34"
	vLinkSlaAttr43 := new(ncsnp.VLinkSla)
	vLinkSlaAttr43.Delay = 3
	vLinkSlaAttr43.Bandwidth = 15000
	vLinkSlaAttr43.FreeBandwidth = 15000
	domainVLink43.VLinkSlaAttr = vLinkSlaAttr43
	domainTopoCache4.DomainVLinkArray = append(domainTopoCache4.DomainVLinkArray, domainVLink43)

	domainVLink42 := new(ncsnp.DomainVLink)
	domainVLink42.LocalDomainName = "Domain4"
	domainVLink42.LocalDomainId = 4
	domainVLink42.RemoteDomainName = "Domain2"
	domainVLink42.RemoteDomainId = 2
	domainVLink42.LocalNodeSN = "Node42"
	domainVLink42.RemoteNodeSN = "Node24"
	domainVLink42.AttachDomainId = 1024
	domainVLink42.AttachDomainName = "Fabric24"
	vLinkSlaAttr42 := new(ncsnp.VLinkSla)
	vLinkSlaAttr42.Delay = 2
	vLinkSlaAttr42.Bandwidth = 15000
	vLinkSlaAttr42.FreeBandwidth = 15000
	domainVLink42.VLinkSlaAttr = vLinkSlaAttr42
	domainTopoCache4.DomainVLinkArray = append(domainTopoCache4.DomainVLinkArray, domainVLink42)
	domainTopoCacheArry = append(domainTopoCacheArry, domainTopoCache4)
	content, err = proto.Marshal(domainTopoCache4)
	if err != nil {
		nputil.TraceErrorWithStack(err)
		return nil
	}
	topoInfo.Field = domainTopoCache4.LocalDomainName
	topoInfo.Content = base64.StdEncoding.EncodeToString(content)
	domainTopoMsg[domainTopoCache4.LocalDomainName] = topoInfo

	for domainName, domainTopoCache := range domainTopoMsg {
		infoString := fmt.Sprintf("domainTopoMsg of domain (%s) is (%+v)\n", domainName, domainTopoCache)
		nputil.TraceInfo(infoString)
	}

	nputil.TraceInfoEnd("------------------------------------------------------")
	return domainTopoMsg
}

func BuildNetworkDomainEdgeForJointDebug() map[string]clusterapi.Topo {
	nputil.TraceInfoBegin("------------BuildNetworkDomainEdgeForJointDebug-------------------")

	var domainTopoCacheArry []*ncsnp.DomainTopoCacheNotify
	domainTopoMsg := make(map[string]clusterapi.Topo)
	topoInfo := clusterapi.Topo{}

	domainTopoCache1 := new(ncsnp.DomainTopoCacheNotify)
	domainTopoCache1.LocalDomainId = 1
	domainTopoCache1.LocalDomainName = "Domain1"
	domainTopoCache1.LocalNodeSN = "1-1"
	domainVLink12 := new(ncsnp.DomainVLink)
	domainVLink12.LocalDomainName = "Domain1"
	domainVLink12.LocalDomainId = 1
	domainVLink12.RemoteDomainName = "Domain2"
	domainVLink12.RemoteDomainId = 2
	domainVLink12.LocalNodeSN = "Node12"
	domainVLink12.RemoteNodeSN = "Node21"
	domainVLink12.AttachDomainId = 1012
	domainVLink12.AttachDomainName = "Fabric12"
	vLinkSlaAttr12 := new(ncsnp.VLinkSla)
	vLinkSlaAttr12.Delay = 8
	vLinkSlaAttr12.Bandwidth = 20000
	vLinkSlaAttr12.FreeBandwidth = 20000
	domainVLink12.VLinkSlaAttr = vLinkSlaAttr12
	domainTopoCache1.DomainVLinkArray = append(domainTopoCache1.DomainVLinkArray, domainVLink12)
	domainVLink14 := new(ncsnp.DomainVLink)
	domainVLink14.LocalDomainName = "Domain1"
	domainVLink14.LocalDomainId = 1
	domainVLink14.RemoteDomainName = "Domain4"
	domainVLink14.RemoteDomainId = 4
	domainVLink14.LocalNodeSN = "Node14"
	domainVLink14.RemoteNodeSN = "Node41"
	domainVLink14.AttachDomainId = 1014
	domainVLink14.AttachDomainName = "Fabric14"
	vLinkSlaAttr14 := new(ncsnp.VLinkSla)
	vLinkSlaAttr14.Delay = 15
	vLinkSlaAttr14.Bandwidth = 20000
	vLinkSlaAttr14.FreeBandwidth = 20000
	domainVLink14.VLinkSlaAttr = vLinkSlaAttr14
	domainVLink14.OpaqueValue = ""
	domainTopoCache1.DomainVLinkArray = append(domainTopoCache1.DomainVLinkArray, domainVLink14)
	domainTopoCacheArry = append(domainTopoCacheArry, domainTopoCache1)
	content, err := proto.Marshal(domainTopoCache1)
	if err != nil {
		nputil.TraceErrorWithStack(err)
		return nil
	}
	topoInfo.Field = domainTopoCache1.LocalDomainName
	topoInfo.Content = base64.StdEncoding.EncodeToString(content)
	domainTopoMsg[domainTopoCache1.LocalDomainName] = topoInfo

	topoInfo = clusterapi.Topo{}
	domainTopoCache2 := new(ncsnp.DomainTopoCacheNotify)
	domainTopoCache2.LocalDomainId = 2
	domainTopoCache2.LocalDomainName = "Domain2"
	domainTopoCache2.LocalNodeSN = "2-1"
	domainVLink24 := new(ncsnp.DomainVLink)
	domainVLink24.LocalDomainName = "Domain2"
	domainVLink24.LocalDomainId = 2
	domainVLink24.RemoteDomainName = "Domain4"
	domainVLink24.RemoteDomainId = 4
	domainVLink24.LocalNodeSN = "Node24"
	domainVLink24.RemoteNodeSN = "Node42"
	domainVLink24.AttachDomainId = 1024
	domainVLink24.AttachDomainName = "Fabric24"
	vLinkSlaAttr24 := new(ncsnp.VLinkSla)
	vLinkSlaAttr24.Delay = 2
	vLinkSlaAttr24.Bandwidth = 20000
	vLinkSlaAttr24.FreeBandwidth = 20000
	domainVLink24.VLinkSlaAttr = vLinkSlaAttr24
	domainTopoCache2.DomainVLinkArray = append(domainTopoCache2.DomainVLinkArray, domainVLink24)

	domainVLink21 := new(ncsnp.DomainVLink)
	domainVLink21.LocalDomainName = "Domain2"
	domainVLink21.LocalDomainId = 2
	domainVLink21.RemoteDomainName = "Domain1"
	domainVLink21.RemoteDomainId = 1
	domainVLink21.LocalNodeSN = "Node21"
	domainVLink21.RemoteNodeSN = "Node12"
	domainVLink21.AttachDomainId = 1012
	domainVLink21.AttachDomainName = "Fabric12"
	vLinkSlaAttr21 := new(ncsnp.VLinkSla)
	vLinkSlaAttr21.Delay = 2
	vLinkSlaAttr21.Bandwidth = 20000
	vLinkSlaAttr21.FreeBandwidth = 20000
	domainVLink21.VLinkSlaAttr = vLinkSlaAttr21
	domainTopoCache2.DomainVLinkArray = append(domainTopoCache2.DomainVLinkArray, domainVLink21)
	domainTopoCacheArry = append(domainTopoCacheArry, domainTopoCache2)
	content, err = proto.Marshal(domainTopoCache2)
	if err != nil {
		nputil.TraceErrorWithStack(err)
		return nil
	}
	topoInfo.Field = domainTopoCache2.LocalDomainName
	topoInfo.Content = base64.StdEncoding.EncodeToString(content)
	domainTopoMsg[domainTopoCache2.LocalDomainName] = topoInfo

	topoInfo = clusterapi.Topo{}
	domainTopoCache4 := new(ncsnp.DomainTopoCacheNotify)
	domainTopoCache4.LocalDomainId = 4
	domainTopoCache4.LocalDomainName = "Domain4"
	domainTopoCache4.LocalNodeSN = "4-1"
	domainVLink41 := new(ncsnp.DomainVLink)
	domainVLink41.LocalDomainName = "Domain3"
	domainVLink41.LocalDomainId = 4
	domainVLink41.RemoteDomainName = "Domain1"
	domainVLink41.RemoteDomainId = 1
	domainVLink41.LocalNodeSN = "Node41"
	domainVLink41.RemoteNodeSN = "Node14"
	domainVLink41.AttachDomainId = 1014
	domainVLink41.AttachDomainName = "Fabric14"
	vLinkSlaAttr41 := new(ncsnp.VLinkSla)
	vLinkSlaAttr41.Delay = 4
	vLinkSlaAttr41.Bandwidth = 20000
	vLinkSlaAttr41.FreeBandwidth = 20000
	domainVLink41.VLinkSlaAttr = vLinkSlaAttr41
	domainTopoCache4.DomainVLinkArray = append(domainTopoCache4.DomainVLinkArray, domainVLink41)
	domainVLink42 := new(ncsnp.DomainVLink)
	domainVLink42.LocalDomainName = "Domain4"
	domainVLink42.LocalDomainId = 4
	domainVLink42.RemoteDomainName = "Domain2"
	domainVLink42.RemoteDomainId = 2
	domainVLink42.LocalNodeSN = "Node42"
	domainVLink42.RemoteNodeSN = "Node24"
	domainVLink42.AttachDomainId = 1024
	domainVLink42.AttachDomainName = "Fabric24"
	vLinkSlaAttr42 := new(ncsnp.VLinkSla)
	vLinkSlaAttr42.Delay = 1
	vLinkSlaAttr42.Bandwidth = 20000
	vLinkSlaAttr42.FreeBandwidth = 20000
	domainVLink42.VLinkSlaAttr = vLinkSlaAttr42
	domainTopoCache4.DomainVLinkArray = append(domainTopoCache4.DomainVLinkArray, domainVLink42)
	domainTopoCacheArry = append(domainTopoCacheArry, domainTopoCache4)
	content, err = proto.Marshal(domainTopoCache4)
	if err != nil {
		nputil.TraceErrorWithStack(err)
		return nil
	}
	topoInfo.Field = domainTopoCache4.LocalDomainName
	topoInfo.Content = base64.StdEncoding.EncodeToString(content)
	domainTopoMsg[domainTopoCache4.LocalDomainName] = topoInfo

	for domainName, domainTopoCache := range domainTopoMsg {
		infoString := fmt.Sprintf("domainTopoMsg of domain (%s) is (%+v)\n", domainName, domainTopoCache)
		nputil.TraceInfo(infoString)
	}

	nputil.TraceInfoEnd("------------------------------------------------------")
	return domainTopoMsg
}

func PrintKspPathBase64() {
	logx.NewLogger()
	infoString := fmt.Sprintf("=== RUN   PrintKspPathBase64  BEGIN ===")
	nputil.TraceInfo(infoString)

	nputil.TraceInfoBegin("------------TestBindingSelectedDomainPathAvailable-------------------")
	npBase64String := "Q3YwQkNxUUJDbG9LS1M5cGJtNWxjaTF3Y21sMllYUmxMMk5sYkdWMlpXNHZZMkZ6WlRFeFkyOXRjRzl1Wlc1ME1pOHhFaWt2YVc1dVpYSXRjSEpwZG1GMFpTOWpaV3hsZG1WdUwyTmhjMlV4TVdOdmJYQnZibVZ1ZERNdk1TZ0VNQU1TQ2dnZUVHUVl2NFE5SUFvYUhRb05VRTlNU1VOWlgxTlBWVkpEUlJJTWNHOXNhV041WDJsdWMzUXhJaHNLQzFCUFRFbERXVjlFUlZOVUVneHdiMnhwWTNsZmFXNXpkRElTRWdvTVkyOXlaVFF0Wm1sbGJHUXhFQVFZQVJJc0NpWnFhV0Z1WjI1aGJpMW1ZV0p5YVdNdE1UQXpMWE5rZDJGdUxYTnliM1V0YUhsd1pYSnZjeEJuR0FJU0Vnb01ZMjl5WlRNdFptbGxiR1F4RUFNWUFRcTlBUXFtQVFwY0Npa3ZhVzV1WlhJdGNISnBkbUYwWlM5alpXeGxkbVZ1TDJOaGMyVXhNV052YlhCdmJtVnVkREl2TVJJcEwybHVibVZ5TFhCeWFYWmhkR1V2WTJWc1pYWmxiaTlqWVhObE1URmpiMjF3YjI1bGJuUXpMekVZQVNnRE1BTVNDZ2dlRUdRWXY0UTlJQW9hSFFvTlVFOU1TVU5aWDFOUFZWSkRSUklNY0c5c2FXTjVYMmx1YzNReEloc0tDMUJQVEVsRFdWOUVSVk5VRWd4d2IyeHBZM2xmYVc1emRESVNFZ29NWTI5eVpUTXRabWxsYkdReEVBTVlBUXIrQVFxbEFRcGFDaWt2YVc1dVpYSXRjSEpwZG1GMFpTOWpaV3hsZG1WdUwyTmhjMlV4TVdOdmJYQnZibVZ1ZERNdk1SSXBMMmx1Ym1WeUxYQnlhWFpoZEdVdlkyVnNaWFpsYmk5allYTmxNVEZqYjIxd2IyNWxiblF5THpFb0F6QUVFZ3NJa0U0UVpCaS9oRDBnQ2hvZENnMVFUMHhKUTFsZlUwOVZVa05GRWd4d2IyeHBZM2xmYVc1emRERWlHd29MVUU5TVNVTlpYMFJGVTFRU0RIQnZiR2xqZVY5cGJuTjBNaElTQ2d4amIzSmxNeTFtYVdWc1pERVFBeGdCRWl3S0ptcHBZVzVuYm1GdUxXWmhZbkpwWXkweE1ETXRjMlIzWVc0dGMzSnZkUzFvZVhCbGNtOXpFR2NZQWhJU0NneGpiM0psTkMxbWFXVnNaREVRQkJnQkNvQUNDcWNCQ2x3S0tTOXBibTVsY2kxd2NtbDJZWFJsTDJObGJHVjJaVzR2WTJGelpURXhZMjl0Y0c5dVpXNTBNeTh4RWlrdmFXNXVaWEl0Y0hKcGRtRjBaUzlqWld4bGRtVnVMMk5oYzJVeE1XTnZiWEJ2Ym1WdWRESXZNUmdCS0FNd0JCSUxDSkJPRUdRWXY0UTlJQW9hSFFvTlVFOU1TVU5aWDFOUFZWSkRSUklNY0c5c2FXTjVYMmx1YzNReEloc0tDMUJQVEVsRFdWOUVSVk5VRWd4d2IyeHBZM2xmYVc1emRESVNFZ29NWTI5eVpUTXRabWxsYkdReEVBTVlBUklzQ2lacWFXRnVaMjVoYmkxbVlXSnlhV010TVRBekxYTmtkMkZ1TFhOeWIzVXRhSGx3WlhKdmN4Qm5HQUlTRWdvTVkyOXlaVFF0Wm1sbGJHUXhFQVFZQVFyQUFRcXBBUXBlQ2lrdmFXNXVaWEl0Y0hKcGRtRjBaUzlqWld4bGRtVnVMMk5oYzJVeE1XTnZiWEJ2Ym1WdWRETXZNUklwTDJsdWJtVnlMWEJ5YVhaaGRHVXZZMlZzWlhabGJpOWpZWE5sTVRGamIyMXdiMjVsYm5ReUx6RVlBaUFCS0FNd0F4SUxDSkJPRUdRWXY0UTlJQW9hSFFvTlVFOU1TVU5aWDFOUFZWSkRSUklNY0c5c2FXTjVYMmx1YzNReEloc0tDMUJQVEVsRFdWOUVSVk5VRWd4d2IyeHBZM2xmYVc1emRESVNFZ29NWTI5eVpUTXRabWxsYkdReEVBTVlBUT09"

	// Verify the unmarshal action
	npBase64Bytes, _ := base64.StdEncoding.DecodeString(npBase64String)
	dbuf := make([]byte, base64.StdEncoding.DecodedLen(len(npBase64Bytes)))
	n, _ := base64.StdEncoding.Decode(dbuf, []byte(npBase64Bytes))
	npConentByteConvert := dbuf[:n]
	infoString = fmt.Sprintf("npConentByteConvert bytes is: [%+v]", npConentByteConvert)
	nputil.TraceInfo(infoString)
	TmpRbDomainPaths := new(ncsnp.BindingSelectedDomainPath)
	err := proto.Unmarshal(npConentByteConvert, TmpRbDomainPaths)
	if err != nil {
		infoString = fmt.Sprintf("TmpRbDomainPaths Proto unmarshal is failed!")
		nputil.TraceInfo(infoString)
	}
	infoString = fmt.Sprintf("The Umarshal BindingSelectedDomainPath is: [%+v]", TmpRbDomainPaths)
	nputil.TraceInfoAlwaysPrint(infoString)

	/*
		npBase64Bytes, _ := base64.StdEncoding.DecodeString(npBase64String)
		infoString = fmt.Sprintf("The npBase64Bytes string is: [%+v]", string(npBase64Bytes))
		nputil.TraceInfoAlwaysPrint(infoString)
		npBytes := make([]byte, base64.StdEncoding.DecodedLen(len(npBase64Bytes)))
		_, _ = base64.StdEncoding.Decode(npBytes, npBase64Bytes)
		infoString = fmt.Sprintf("The NpBase64Byte is: [%+v]", npBase64Bytes)
		nputil.TraceInfoAlwaysPrint(infoString)
		TmpRbDomainPaths := new(ncsnp.BindingSelectedDomainPath)
		err := proto.Unmarshal(npBytes, TmpRbDomainPaths)
		if err != nil {
			infoString = fmt.Sprintf("TmpRbDomainPaths Proto unmarshal is failed!")
			nputil.TraceInfoAlwaysPrint(infoString)
		} else {
			infoString = fmt.Sprintf("The Umarshal BindingSelectedDomainPath is: [%+v]", TmpRbDomainPaths)
			nputil.TraceInfoAlwaysPrint(infoString)
		}*/

	infoString = fmt.Sprintf("=== RUN   TestBindingDomainPathAvailable  END ===")
	nputil.TraceInfo(infoString)
}

func PrintKspPath() {
	logx.NewLogger()
	infoString := fmt.Sprintf("=== RUN   PrintKspPath  BEGIN ===")
	nputil.TraceInfo(infoString)

	nputil.TraceInfoBegin("------------TestBindingSelectedDomainPathAvailable-------------------")
	// npBase64String := "CogCCm4KJgoPL3BtbC9jYXNlMi9jMS8xEg8vcG1sL2Nhc2UyL2MyLzEoATADEggIZBBkGGQgZBodCg1QT0xJQ1lfU09VUkNFEgxwb2xpY3lfaW5zdDEiGwoLUE9MSUNZX0RFU1QSDHBvbGljeV9pbnN0MhISCgxjb3JlMS1maWVsZDEQARgBEiwKJmppYW5nbmFuLWZhYnJpYy0xMDEtc2R3YW4tc3JvdS1oeXBlcm9zEGUYAhISCgxjb3JlMi1maWVsZDEQAhgBEiwKJmppYW5nbmFuLWZhYnJpYy0xMDItc2R3YW4tc3JvdS1oeXBlcm\n9zEGYYAhISCgxjb3JlMy1maWVsZDEQAxgBCsYBCm4KJgoPL3BtbC9jYXNlMi9jMi8xEg8vcG1sL2Nhc2UyL2MxLzEoAzABEggIZBBkGGQgZBodCg1QT0xJQ1lfU09VUkNFEgxwb2xpY3lfaW5zdDEiGwoLUE9MSUNZX0RFU1QSDHBvbGljeV9pbnN0MhISCgxjb3JlMy1maWVsZDEQAxgBEiwKJmppYW5nbmFuLWZhYnJpYy0xMDMtc2R3YW4tc3JvdS1oeXBlcm9zEGcYAhISCgxjb3JlMS1maWVsZDEQARgB"
	npBase64String := "Q3YwQkNxUUJDbG9LS1M5cGJtNWxjaTF3Y21sMllYUmxMMk5sYkdWMlpXNHZZMkZ6WlRFeFkyOXRjRzl1Wlc1ME1pOHhFaWt2YVc1dVpYSXRjSEpwZG1GMFpTOWpaV3hsZG1WdUwyTmhjMlV4TVdOdmJYQnZibVZ1ZERNdk1TZ0VNQU1TQ2dnZUVHUVl2NFE5SUFvYUhRb05VRTlNU1VOWlgxTlBWVkpEUlJJTWNHOXNhV041WDJsdWMzUXhJaHNLQzFCUFRFbERXVjlFUlZOVUVneHdiMnhwWTNsZmFXNXpkRElTRWdvTVkyOXlaVFF0Wm1sbGJHUXhFQVFZQVJJc0NpWnFhV0Z1WjI1aGJpMW1ZV0p5YVdNdE1UQXpMWE5rZDJGdUxYTnliM1V0YUhsd1pYSnZjeEJuR0FJU0Vnb01ZMjl5WlRNdFptbGxiR1F4RUFNWUFRcTlBUXFtQVFwY0Npa3ZhVzV1WlhJdGNISnBkbUYwWlM5alpXeGxkbVZ1TDJOaGMyVXhNV052YlhCdmJtVnVkREl2TVJJcEwybHVibVZ5TFhCeWFYWmhkR1V2WTJWc1pYWmxiaTlqWVhObE1URmpiMjF3YjI1bGJuUXpMekVZQVNnRE1BTVNDZ2dlRUdRWXY0UTlJQW9hSFFvTlVFOU1TVU5aWDFOUFZWSkRSUklNY0c5c2FXTjVYMmx1YzNReEloc0tDMUJQVEVsRFdWOUVSVk5VRWd4d2IyeHBZM2xmYVc1emRESVNFZ29NWTI5eVpUTXRabWxsYkdReEVBTVlBUXErQVFxbkFRcGNDaWt2YVc1dVpYSXRjSEpwZG1GMFpTOWpaV3hsZG1WdUwyTmhjMlV4TVdOdmJYQnZibVZ1ZERNdk1SSXBMMmx1Ym1WeUxYQnlhWFpoZEdVdlkyVnNaWFpsYmk5allYTmxNVEZqYjIxd2IyNWxiblF5THpFZ0FTZ0RNQU1TQ3dpUVRoQmtHTCtFUFNBS0doMEtEVkJQVEVsRFdWOVRUMVZTUTBVU0RIQnZiR2xqZVY5cGJuTjBNU0liQ2d0UVQweEpRMWxmUkVWVFZCSU1jRzlzYVdONVgybHVjM1F5RWhJS0RHTnZjbVV6TFdacFpXeGtNUkFER0FFS3dnSUtwd0VLWEFvcEwybHVibVZ5TFhCeWFYWmhkR1V2WTJWc1pYWmxiaTlqWVhObE1URmpiMjF3YjI1bGJuUXpMekVTS1M5cGJtNWxjaTF3Y21sMllYUmxMMk5sYkdWMlpXNHZZMkZ6WlRFeFkyOXRjRzl1Wlc1ME1pOHhHQUVvQXpBRUVnc0lrRTRRWkJpL2hEMGdDaG9kQ2cxUVQweEpRMWxmVTA5VlVrTkZFZ3h3YjJ4cFkzbGZhVzV6ZERFaUd3b0xVRTlNU1VOWlgwUkZVMVFTREhCdmJHbGplVjlwYm5OME1oSVNDZ3hqYjNKbE15MW1hV1ZzWkRFUUF4Z0JFaXdLSm1wcFlXNW5ibUZ1TFdaaFluSnBZeTB4TURJdGMyUjNZVzR0YzNKdmRTMW9lWEJsY205ekVHWVlBaElTQ2d4amIzSmxNaTFtYVdWc1pERVFBaGdCRWl3S0ptcHBZVzVuYm1GdUxXWmhZbkpwWXkweE1ERXRjMlIzWVc0dGMzSnZkUzFvZVhCbGNtOXpFR1VZQWhJU0NneGpiM0psTkMxbWFXVnNaREVRQkJnQkNzSUNDcWNCQ2x3S0tTOXBibTVsY2kxd2NtbDJZWFJsTDJObGJHVjJaVzR2WTJGelpURXhZMjl0Y0c5dVpXNTBNeTh4RWlrdmFXNXVaWEl0Y0hKcGRtRjBaUzlqWld4bGRtVnVMMk5oYzJVeE1XTnZiWEJ2Ym1WdWRESXZNUmdDS0FNd0JCSUxDSkJPRUdRWXY0UTlJQW9hSFFvTlVFOU1TVU5aWDFOUFZWSkRSUklNY0c5c2FXTjVYMmx1YzNReEloc0tDMUJQVEVsRFdWOUVSVk5VRWd4d2IyeHBZM2xmYVc1emRESVNFZ29NWTI5eVpUTXRabWxsYkdReEVBTVlBUklzQ2lacWFXRnVaMjVoYmkxbVlXSnlhV010TVRBeUxYTmtkMkZ1TFhOeWIzVXRhSGx3WlhKdmN4Qm1HQUlTRWdvTVkyOXlaVEl0Wm1sbGJHUXhFQUlZQVJJc0NpWnFhV0Z1WjI1aGJpMW1ZV0p5YVdNdE1UQXhMWE5rZDJGdUxYTnliM1V0YUhsd1pYSnZjeEJsR0FJU0Vnb01ZMjl5WlRRdFptbGxiR1F4RUFRWUFRPT0="
	// npBase64Bytes, _ := base64.StdEncoding.DecodeString(npBase64String)
	npBase64Bytes := ([]byte)(npBase64String)
	npBytes := make([]byte, base64.StdEncoding.DecodedLen(len(npBase64Bytes)))
	_, _ = base64.StdEncoding.Decode(npBytes, npBase64Bytes)
	infoString = fmt.Sprintf("The NpBase64Byte is: [%+v]", npBase64Bytes)
	nputil.TraceInfoAlwaysPrint(infoString)
	TmpRbDomainPaths := new(ncsnp.BindingSelectedDomainPath)
	err := proto.Unmarshal(npBytes, TmpRbDomainPaths)
	if err != nil {
		infoString = fmt.Sprintf("TmpRbDomainPaths Proto unmarshal is failed!")
		nputil.TraceInfoAlwaysPrint(infoString)
	} else {
		infoString = fmt.Sprintf("The Umarshal BindingSelectedDomainPath is: [%+v]", TmpRbDomainPaths)
		nputil.TraceInfoAlwaysPrint(infoString)
	}

	infoString = fmt.Sprintf("=== RUN   TestBindingDomainPathAvailable  END ===")
	nputil.TraceInfo(infoString)
}

func GetSchCacheTopoInfo() {
	var topoContents map[string]string
	topoContents = make(map[string]string)

	topoContents["core2-field1"] = "COIrEAIaDGNvcmUyLWZpZWxkMSr7AQoMY29yZTItZmllbGQxEAIaDGNvcmU0LWZpZWxkMSAEKiAzYmFkZjU2MjA3MWM0Y2ZmYTJhYzM4NTBmM2Q2MDBmZDIgZjMyZmIwN2Q1MzJkNDM2OGI4MDI5YTM4OTM4ZmMyZWI6GWxvb3AtR2lnYWJpdEV0aGVybmV0MC9hLzBAZUoMCBQggK3iBCiAreIEWiZqaWFuZ25hbi1mYWJyaWMtMTAxLXNkd2FuLXNyb3UtaHlwZXJvc2IgZThkOTZiODM5MWM3NGI5ZmI4OWFjNGQ1OGY1M2ZiMmJqIGQyZjRkNWIzZWNhOTQ4OWRhOTJmYjJlMTJiZTcwZGRlKvsBCgxjb3JlMi1maWVsZDEQAhoMY29yZTMtZmllbGQxIAMqIDAwMzg0Y2Y5N2E2MjRmMGQ4MTE3NmJkMDQ5ZjdmNmVkMiBiMTU2Nzc5NTZjYTY0OGZjYmVkOGRmOGVjNTI0NGYzNzoZbG9vcC1HaWdhYml0RXRoZXJuZXQwL2EvMEBmSgwIDyCAreIEKICt4gRaJmppYW5nbmFuLWZhYnJpYy0xMDItc2R3YW4tc3JvdS1oeXBlcm9zYiA3Y2I1ZWE3Mzk1NmU0YWYxOWRjZTI1NzY2ZDFlMTM1YWogYjdlNDU5MTM4YjY3NDlmZGJiMjQ4YmEyNDg0YzlhNGUqgQEKDGNvcmUyLWZpZWxkMRACGgxjb3JlMy1maWVsZDEgAyogMDAzODRjZjk3YTYyNGYwZDgxMTc2YmQwNDlmN2Y2ZWQyIGIxNTY3Nzk1NmNhNjQ4ZmNiZWQ4ZGY4ZWM1MjQ0ZjM3Ohlsb29wLUdpZ2FiaXRFdGhlcm5ldDAvYi8wSgAqgQEKDGNvcmUyLWZpZWxkMRACGgxjb3JlNC1maWVsZDEgBCogM2JhZGY1NjIwNzFjNGNmZmEyYWMzODUwZjNkNjAwZmQyIGYzMmZiMDdkNTMyZDQzNjhiODAyOWEzODkzOGZjMmViOhlsb29wLUdpZ2FiaXRFdGhlcm5ldDAvYi8wSgA="
	topoContents["core3-field1"] = "CNXgOBADGgxjb3JlMy1maWVsZDEq0wEKDGNvcmUzLWZpZWxkMRADGgxjb3JlMi1maWVsZDEgAiogYjE1Njc3OTU2Y2E2NDhmY2JlZDhkZjhlYzUyNDRmMzcyIDAwMzg0Y2Y5N2E2MjRmMGQ4MTE3NmJkMDQ5ZjdmNmVkOhlsb29wLUdpZ2FiaXRFdGhlcm5ldDAvYS8wQGZKDAgPIICt4gQogK3iBGIgYjdlNDU5MTM4YjY3NDlmZGJiMjQ4YmEyNDg0YzlhNGVqIDdjYjVlYTczOTU2ZTRhZjE5ZGNlMjU3NjZkMWUxMzVhKtMBCgxjb3JlMy1maWVsZDEQAxoMY29yZTQtZmllbGQxIAQqIDIwNGFmZGE5ZjlmNjRhNGVhZTdiMWRiMGNiYzA0NzMwMiAzYjI1ZjZmMDVkYzQ0OGZhYTUzYzBjZGEzOTcxZTI3YzoZbG9vcC1HaWdhYml0RXRoZXJuZXQwL2IvMEBnSgwIFCCAreIEKICt4gRiIGM2ZTA3MThhZWQ2YzQ4MmU5YjQzZjA4ZDNkMmMxNWEzaiA2YWU0ZDVmNTdmZWM0OTA4OTMwOWYxOTY3NDBmMmU1MSqBAQoMY29yZTMtZmllbGQxEAMaDGNvcmU0LWZpZWxkMSAEKiAyMDRhZmRhOWY5ZjY0YTRlYWU3YjFkYjBjYmMwNDczMDIgM2IyNWY2ZjA1ZGM0NDhmYWE1M2MwY2RhMzk3MWUyN2M6GWxvb3AtR2lnYWJpdEV0aGVybmV0MC9hLzBKACqBAQoMY29yZTMtZmllbGQxEAMaDGNvcmUyLWZpZWxkMSACKiBiMTU2Nzc5NTZjYTY0OGZjYmVkOGRmOGVjNTI0NGYzNzIgMDAzODRjZjk3YTYyNGYwZDgxMTc2YmQwNDlmN2Y2ZWQ6GWxvb3AtR2lnYWJpdEV0aGVybmV0MC9iLzBKAA=="
	topoContents["core4-field1"] = "CJWeNxAEGgxjb3JlNC1maWVsZDEq+wEKDGNvcmU0LWZpZWxkMRAEGgxjb3JlMi1maWVsZDEgAiogZjMyZmIwN2Q1MzJkNDM2OGI4MDI5YTM4OTM4ZmMyZWIyIDNiYWRmNTYyMDcxYzRjZmZhMmFjMzg1MGYzZDYwMGZkOhlsb29wLUdpZ2FiaXRFdGhlcm5ldDAvYS8wQGVKDAgUIICt4gQogK3iBFomamlhbmduYW4tZmFicmljLTEwMS1zZHdhbi1zcm91LWh5cGVyb3NiIGQyZjRkNWIzZWNhOTQ4OWRhOTJmYjJlMTJiZTcwZGRlaiBlOGQ5NmI4MzkxYzc0YjlmYjg5YWM0ZDU4ZjUzZmIyYir7AQoMY29yZTQtZmllbGQxEAQaDGNvcmUzLWZpZWxkMSADKiAzYjI1ZjZmMDVkYzQ0OGZhYTUzYzBjZGEzOTcxZTI3YzIgMjA0YWZkYTlmOWY2NGE0ZWFlN2IxZGIwY2JjMDQ3MzA6GWxvb3AtR2lnYWJpdEV0aGVybmV0MC9hLzBAZ0oMCBQggK3iBCiAreIEWiZqaWFuZ25hbi1mYWJyaWMtMTAzLXNkd2FuLXNyb3UtaHlwZXJvc2IgNmFlNGQ1ZjU3ZmVjNDkwODkzMDlmMTk2NzQwZjJlNTFqIGM2ZTA3MThhZWQ2YzQ4MmU5YjQzZjA4ZDNkMmMxNWEzKoEBCgxjb3JlNC1maWVsZDEQBBoMY29yZTMtZmllbGQxIAMqIDNiMjVmNmYwNWRjNDQ4ZmFhNTNjMGNkYTM5NzFlMjdjMiAyMDRhZmRhOWY5ZjY0YTRlYWU3YjFkYjBjYmMwNDczMDoZbG9vcC1HaWdhYml0RXRoZXJuZXQwL2IvMEoAKoEBCgxjb3JlNC1maWVsZDEQBBoMY29yZTItZmllbGQxIAIqIGYzMmZiMDdkNTMyZDQzNjhiODAyOWEzODkzOGZjMmViMiAzYmFkZjU2MjA3MWM0Y2ZmYTJhYzM4NTBmM2Q2MDBmZDoZbG9vcC1HaWdhYml0RXRoZXJuZXQwL2IvMEoA"
	for filedName, topoMsg := range topoContents {
		domainTopoByte64, _ := base64.StdEncoding.DecodeString(topoMsg)
		domainTopoCache := new(ncsnp.DomainTopoCacheNotify)
		err := proto.Unmarshal(domainTopoByte64, domainTopoCache)
		if err != nil {
			nputil.TraceError(err)
			return
		}
		infoString := fmt.Sprintf("Domain(%s)'s domainTopoCache is:(%+v)", filedName, *domainTopoCache)
		nputil.TraceInfoAlwaysPrint(infoString)
	}
}

func TestParseBindingDomainPathAvailable(t *testing.T) {
	logx.NewLogger()
	infoString := fmt.Sprintf("=== RUN   TestBindingDomainPathAvailable  BEGIN ===")
	nputil.TraceInfo(infoString)

	npBase64String := "Q293QkNqUUtKZ29QTDNCdGJDOXdjbWwyWVhSbEwyTXpFZzh2Y0cxc0wzQnlhWFpoZEdVdll6WW9BVEFERWdvSUtCQmtHTCtFUFNBS0VoSUtER052Y21VeExXWnBaV3hrTVJBQkdBRVNMQW9tYW1saGJtZHVZVzR0Wm1GaWNtbGpMVEV3TXkxelpIZGhiaTF6Y205MUxXaDVjR1Z5YjNNUVp4Z0NFaElLREdOdmNtVXpMV1pwWld4a01SQURHQUVLakFFS05Bb21DZzh2Y0cxc0wzQnlhWFpoZEdVdll6VVNEeTl3Yld3dmNISnBkbUYwWlM5ak55Z0RNQUVTQ2dnb0VHUVl2NFE5SUdRU0Vnb01ZMjl5WlRNdFptbGxiR1F4RUFNWUFSSXNDaVpxYVdGdVoyNWhiaTFtWVdKeWFXTXRNVEF6TFhOa2QyRnVMWE55YjNVdGFIbHdaWEp2Y3hCbkdBSVNFZ29NWTI5eVpURXRabWxsYkdReEVBRVlBUXJRQVFvMkNpZ0tEeTl3Yld3dmNISnBkbUYwWlM5ak5SSVBMM0J0YkM5d2NtbDJZWFJsTDJNM0dBRW9BekFCRWdvSUtCQmtHTCtFUFNCa0VoSUtER052Y21VekxXWnBaV3hrTVJBREdBRVNMQW9tYW1saGJtZHVZVzR0Wm1GaWNtbGpMVEV3TWkxelpIZGhiaTF6Y205MUxXaDVjR1Z5YjNNUVpoZ0NFaElLREdOdmNtVXlMV1pwWld4a01SQUNHQUVTTEFvbWFtbGhibWR1WVc0dFptRmljbWxqTFRFd01TMXpaSGRoYmkxemNtOTFMV2g1Y0dWeWIzTVFaUmdDRWhJS0RHTnZjbVV4TFdacFpXeGtNUkFCR0FFS2pnRUtOZ29vQ2c4dmNHMXNMM0J5YVhaaGRHVXZZelVTRHk5d2JXd3ZjSEpwZG1GMFpTOWpOeGdDS0FNd0FSSUtDQ2dRWkJpL2hEMGdaQklTQ2d4amIzSmxNeTFtYVdWc1pERVFBeGdCRWl3S0ptcHBZVzVuYm1GdUxXWmhZbkpwWXkweE1ETXRjMlIzWVc0dGMzSnZkUzFvZVhCbGNtOXpFR2NZQWhJU0NneGpiM0psTVMxbWFXVnNaREVRQVJnQg=="
	npBase64Bytes, _ := base64.StdEncoding.DecodeString(npBase64String)
	npBytes := make([]byte, base64.StdEncoding.DecodedLen(len(npBase64Bytes)))
	_, _ = base64.StdEncoding.Decode(npBytes, npBase64Bytes)
	infoString = fmt.Sprintf("The NpBase64Byte is: [%+v]", npBase64Bytes)
	nputil.TraceInfo(infoString)
	TmpRbDomainPaths := new(ncsnp.BindingSelectedDomainPath)
	err := proto.Unmarshal(npBytes, TmpRbDomainPaths)
	if err != nil {
		infoString = fmt.Sprintf("TmpRbDomainPaths Proto unmarshal is failed!")
		nputil.TraceInfo(infoString)
	} else {
		infoString = fmt.Sprintf("The Umarshal BindingSelectedDomainPath is: [%+v]", TmpRbDomainPaths)
		nputil.TraceInfo(infoString)
	}

	infoString = fmt.Sprintf("=== RUN   TestBindingDomainPathAvailable  END ===")
	nputil.TraceInfo(infoString)
}

func TestNetworkFilterAvailable(t *testing.T) {
	logx.NewLogger()
	infoString := fmt.Sprintf("=== RUN   TestNetworkFilterAvailable  BEGIN ===")
	nputil.TraceInfo(infoString)

	rbs, networkRequirement := SetRbsAndNetReqRtt()
	networkInfoMap := BuildNetworkDomainEdge()
	rbsRet := NetworkFilter(rbs, networkRequirement, networkInfoMap)
	if reflect.DeepEqual(0, len(rbsRet)) {
		infoString := fmt.Sprintf("The rbs should be available!")
		nputil.TraceErrorString(infoString)
		t.Errorf("The rbs should be available!")
	}

	infoString = fmt.Sprintf("=== RUN   TestNetworkFilterAvailable  END ===")
	nputil.TraceInfo(infoString)
}

func TestNetworkFilterRtt(t *testing.T) {
	logx.NewLogger()
	infoString := fmt.Sprintf("=== RUN   TestNetworkFilterRtt  BEGIN ===")
	nputil.TraceInfo(infoString)

	rbs, networkRequirement := SetRbsAndNetReqRtt()
	networkInfoMap := BuildNetworkDomainEdge()
	rbsRet := NetworkFilter(rbs, networkRequirement, networkInfoMap)
	if reflect.DeepEqual(0, len(rbsRet)) {
		infoString := fmt.Sprintf("The rbs should be available!")
		nputil.TraceErrorString(infoString)
		t.Errorf("The rbs should be available!")
	}

	infoString = fmt.Sprintf("=== RUN   TestNetworkFilterRtt  END ===")
	nputil.TraceInfo(infoString)
}

func TestNetworkFilterProviders(t *testing.T) {
	logx.NewLogger()
	infoString := fmt.Sprintf("=== RUN   TestNetworkFilterProviders  BEGIN ===")
	nputil.TraceInfo(infoString)

	rbs, networkRequirement := SetRbsAndNetReqProviders()
	networkInfoMap := BuildNetworkDomainEdge()
	rbsRet := NetworkFilter(rbs, networkRequirement, networkInfoMap)
	if reflect.DeepEqual(0, len(rbsRet)) {
		infoString := fmt.Sprintf("The rbs should be available!")
		nputil.TraceErrorString(infoString)
		t.Errorf("The rbs should be available!")
	}

	infoString = fmt.Sprintf("=== RUN   TestNetworkFilterProviders  END ===")
	nputil.TraceInfo(infoString)
}

func TestNetworkFilterAccelerate(t *testing.T) {
	logx.NewLogger()
	infoString := fmt.Sprintf("=== RUN   TestNetworkFilterAccelerate  BEGIN ===")
	nputil.TraceInfo(infoString)

	rbs, networkRequirement := SetRbsAndNetReqAccelerate()
	networkInfoMap := BuildNetworkDomainEdge()
	rbsRet := NetworkFilter(rbs, networkRequirement, networkInfoMap)
	if reflect.DeepEqual(0, len(rbsRet)) {
		infoString := fmt.Sprintf("The rbs should be available!")
		nputil.TraceErrorString(infoString)
		t.Errorf("The rbs should be available!")
	}

	infoString = fmt.Sprintf("=== RUN   TestNetworkFilterAccelerate  END ===")
	nputil.TraceInfo(infoString)
}

func TestNetworkFilterBestEffort(t *testing.T) {
	logx.NewLogger()
	infoString := fmt.Sprintf("=== RUN   TestNetworkFilterBestEffort  BEGIN ===")
	nputil.TraceInfo(infoString)

	rbs, networkRequirement := SetRbsAndNetReqBestEffort()
	networkInfoMap := BuildNetworkDomainEdge()
	rbsRet := NetworkFilter(rbs, networkRequirement, networkInfoMap)
	if reflect.DeepEqual(0, len(rbsRet)) {
		infoString := fmt.Sprintf("The rbs should be available!")
		nputil.TraceErrorString(infoString)
		t.Errorf("The rbs should be available!")
	}

	infoString = fmt.Sprintf("=== RUN   TestNetworkFilterBestEffort  END ===")
	nputil.TraceInfo(infoString)
}

func TestNetworkFilterAvailableForJointDebug(t *testing.T) {
	logx.NewLogger()
	infoString := fmt.Sprintf("=== RUN   TestNetworkFilterAvailableForJointDebug  BEGIN ===")
	nputil.TraceInfo(infoString)

	rbs, networkRequirement := SetRbsAndNetReqThroughputSla()
	networkInfoMap := BuildNetworkDomainEdgeForJointDebug()
	rbsRet := NetworkFilter(rbs, networkRequirement, networkInfoMap)
	if reflect.DeepEqual(0, len(rbsRet)) {
		infoString := fmt.Sprintf("The rbs should be available!")
		nputil.TraceErrorString(infoString)
		t.Errorf("The rbs should be available!")
	}

	infoString = fmt.Sprintf("=== RUN  TestNetworkFilterAvailableForJointDebug  END ===")
	nputil.TraceInfo(infoString)
}

func TestNetworkFilterUnAvailableTopoFailed(t *testing.T) {
	logx.NewLogger()

	infoString := fmt.Sprintf("=== RUN   TestNetworkFilterUnAvailableTopoFailed  BEGIN ===")
	nputil.TraceInfo(infoString)

	rbs, networkRequirement := SetRbsAndNetReqTopoFailed()
	networkInfoMap := BuildNetworkDomainEdge()
	rbsRet := NetworkFilter(rbs, networkRequirement, networkInfoMap)
	if !reflect.DeepEqual(0, len(rbsRet)) {
		infoString := fmt.Sprintf("The rbs should be unavailable due to topo failed!")
		nputil.TraceErrorString(infoString)
		t.Errorf("The rbs should be unavailable due to topo failed!")
	}

	infoString = fmt.Sprintf("=== RUN   TestNetworkFilterUnAvailableTopoFailed  END ===")
	nputil.TraceInfo(infoString)
}

func TestNetworkFilterDelaySlaFailed(t *testing.T) {
	logx.NewLogger()

	infoString := fmt.Sprintf("=== RUN   TestNetworkFilterDelaySlaFailed  BEGIN ===")
	nputil.TraceInfo(infoString)

	rbs, networkRequirement := SetRbsAndNetReqDelaySlaFailed()
	networkInfoMap := BuildNetworkDomainEdge()
	rbsRet := NetworkFilter(rbs, networkRequirement, networkInfoMap)
	if !reflect.DeepEqual(0, len(rbsRet)) {
		infoString := fmt.Sprintf("The rbs should be unavailable due to delay sla failed!")
		nputil.TraceErrorString(infoString)
		t.Errorf("The rbs should be unavailable due to sla failed!")
	}

	infoString = fmt.Sprintf("=== RUN   TestNetworkFilterDelaySlaFailed  END ===")
	nputil.TraceInfo(infoString)
}

func TestNetworkFilterThroughputSla(t *testing.T) {
	logx.NewLogger()

	infoString := fmt.Sprintf("=== RUN   TestNetworkFilterThroughputSla  BEGIN ===")
	nputil.TraceInfo(infoString)

	rbs, networkRequirement := SetRbsAndNetReqThroughputSla()
	networkInfoMap := BuildNetworkDomainEdge()
	rbsRet := NetworkFilter(rbs, networkRequirement, networkInfoMap)
	if !reflect.DeepEqual(2, len(rbsRet[0].Spec.NetworkPath)) {
		infoString := fmt.Sprintf("The rbs should be available and the len of rbs NetworkPath is 3!")
		nputil.TraceErrorString(infoString)
		t.Errorf("The rbs should be available and the len of rbs  NetworkPath is 3!")
	}

	infoString = fmt.Sprintf("=== RUN   TestNetworkFilterThroughputSla  END ===")
	nputil.TraceInfo(infoString)
}

func TestNetworkFilterThroughputSlaFailed(t *testing.T) {
	logx.NewLogger()
	infoString := fmt.Sprintf("=== RUN   TestNetworkFilterThroughputSlaFailed  BEGIN ===")
	nputil.TraceInfo(infoString)

	rbs, networkRequirement := SetRbsAndNetReqThroughputSlaFailed()
	networkInfoMap := BuildNetworkDomainEdge()
	rbsRet := NetworkFilter(rbs, networkRequirement, networkInfoMap)
	if !reflect.DeepEqual(0, len(rbsRet)) {
		infoString := fmt.Sprintf("The rbs should be unavailable due to throughput sla failed!")
		nputil.TraceErrorString(infoString)
		t.Errorf("The rbs should be unavailable due to throughput sla failed!")
	}

	infoString = fmt.Sprintf("=== RUN   TestNetworkFilterThroughputSlaFailed  END ===")
	nputil.TraceInfo(infoString)
}

func TestNetworkFilterNoInterCommunication(t *testing.T) {
	logx.NewLogger()

	infoString := fmt.Sprintf("=== RUN   TestNetworkFilterNoInterCommunication  BEGIN ===")
	nputil.TraceInfo(infoString)

	rbs, networkRequirement := SetRbsAndNetReqNoInterCommunication()
	networkInfoMap := BuildNetworkDomainEdge()
	rbsRet := NetworkFilter(rbs, networkRequirement, networkInfoMap)
	if reflect.DeepEqual(0, len(rbsRet)) {
		infoString := fmt.Sprintf("There is no interCommunication, the rb should be available!")
		nputil.TraceErrorString(infoString)
		t.Errorf("There is no interCommunication, the rb should be available!")
	}

	infoString = fmt.Sprintf("=== RUN   TestNetworkFilterNoInterCommunication  END ===")
	nputil.TraceInfo(infoString)
}

func TestNetworkFilterSameDomain(t *testing.T) {
	logx.NewLogger()

	infoString := fmt.Sprintf("=== RUN   TestNetworkFilterSameDomain  BEGIN ===")
	nputil.TraceInfo(infoString)

	rbs, networkRequirement := SetRbsAndNetReqSameDomain()
	networkInfoMap := BuildNetworkDomainEdge()
	rbsRet := NetworkFilter(rbs, networkRequirement, networkInfoMap)
	if len(rbsRet) != 0 {
		infoString := fmt.Sprintf("The rbs is available for TestNetworkFilterSameDomain!")
		nputil.TraceInfo(infoString)
	}
	if reflect.DeepEqual(0, len(rbsRet)) {
		infoString := fmt.Sprintf("The rb should be available, because components in the same filed!")
		nputil.TraceErrorString(infoString)
		t.Errorf("The rb should be available, because components in the same filed!")
	}

	infoString = fmt.Sprintf("=== RUN   TestNetworkFilterSameDomain  END ===")
	nputil.TraceInfo(infoString)
}

// Case 5: Component存在相互的连接属性
func TestNetworkFilterInterCommunication(t *testing.T) {
	logx.NewLogger()

	infoString := fmt.Sprintf("=== RUN   TestNetworkFilterInterCommunication  BEGIN ===")
	nputil.TraceInfo(infoString)

	rbs, networkRequirement := SetRbsAndNetReqInterCommunication()
	networkInfoMap := BuildNetworkDomainEdge()
	rbsRet := NetworkFilter(rbs, networkRequirement, networkInfoMap)
	if reflect.DeepEqual(0, len(rbsRet)) {
		infoString := fmt.Sprintf("The rb should be available and communicate to each other!")
		nputil.TraceErrorString(infoString)
		t.Errorf("The rb should be available and communicate to each other!")
	}

	infoString = fmt.Sprintf("=== RUN   TestNetworkFilterInterCommunication  END ===")
	nputil.TraceInfo(infoString)
}
