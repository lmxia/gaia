package utils

import (
	"encoding/json"
	lmmserverless "github.com/SUMMERLm/serverless/api/v1"
	appsv1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"testing"
)

func SetOldDescComponents() []appsv1alpha1.Component {
	components := []appsv1alpha1.Component{
		0: {
			Name:        "case7-component1",
			RuntimeType: string(appsv1alpha1.Kata),
			Module:      corev1.PodTemplateSpec{},
			Workload: appsv1alpha1.Workload{
				Workloadtype: appsv1alpha1.WorkloadTypeStatefulSet,
				TraitStatefulSet: &appsv1alpha1.TraitStatefulSet{
					Replicas: 199,
				},
			},
			SchedulePolicy: appsv1alpha1.SchedulePolicy{
				Level: map[appsv1alpha1.SchedulePolicyLevel]*metav1.LabelSelector{
					appsv1alpha1.SchedulePolicyMandatory: {
						MatchExpressions: []metav1.LabelSelectorRequirement{
							0: {Key: "geo-location", Operator: "In", Values: []string{"China-Huadong-Jiangsu-City-C21-District-E21"}},
							1: {Key: "netenvironment", Operator: "In", Values: []string{"edge"}},
						},
					},
				},
			},
		},
		1: {
			Name:        "case7-component2",
			RuntimeType: string(appsv1alpha1.Runc),
			Module:      corev1.PodTemplateSpec{},
			Workload: appsv1alpha1.Workload{
				Workloadtype: appsv1alpha1.WorkloadTypeUserApp,
				TraitUserAPP: &appsv1alpha1.TraitUserAPP{
					SN: "sn1",
				},
			},
			SchedulePolicy: appsv1alpha1.SchedulePolicy{
				Level: map[appsv1alpha1.SchedulePolicyLevel]*metav1.LabelSelector{
					appsv1alpha1.SchedulePolicyMandatory: {
						MatchExpressions: []metav1.LabelSelectorRequirement{
							0: {Key: "sn", Operator: "In", Values: []string{"sn1"}},
						},
					},
				},
			},
		},
		2: {
			Name:        "case7-component3",
			RuntimeType: string(appsv1alpha1.Runc),
			Module:      corev1.PodTemplateSpec{},
			Workload: appsv1alpha1.Workload{
				Workloadtype: appsv1alpha1.WorkloadTypeAffinityDaemon,
				TraitAffinityDaemon: &appsv1alpha1.TraitAffinityDaemon{
					SNS: []string{"sn1", "sn2"},
				},
			},
			SchedulePolicy: appsv1alpha1.SchedulePolicy{
				Level: map[appsv1alpha1.SchedulePolicyLevel]*metav1.LabelSelector{
					appsv1alpha1.SchedulePolicyMandatory: {
						MatchExpressions: []metav1.LabelSelectorRequirement{
							0: {Key: "sn", Operator: "In", Values: []string{"sn1", "sn2"}},
						},
					},
				},
			},
		},
		3: {
			Name:        "case7-component4",
			RuntimeType: string(appsv1alpha1.Runc),
			Module:      corev1.PodTemplateSpec{},
			Workload: appsv1alpha1.Workload{
				Workloadtype: appsv1alpha1.WorkloadTypeServerless,
				TraitServerless: &lmmserverless.TraitServerless{
					MaxReplicas:   1000,
					ResplicasStep: 1,
					Threshold:     "{\"cpuMax\":75,\"cpuMin\":25,\"memMax\":85,\"memMin\":25}",
				},
			},
			SchedulePolicy: appsv1alpha1.SchedulePolicy{
				Level: map[appsv1alpha1.SchedulePolicyLevel]*metav1.LabelSelector{},
			},
		},
		4: {
			Name:        "case7-component5",
			RuntimeType: string(appsv1alpha1.Runc),
			Module:      corev1.PodTemplateSpec{},
			Workload: appsv1alpha1.Workload{
				Workloadtype: appsv1alpha1.WorkloadTypeServerless,
				TraitServerless: &lmmserverless.TraitServerless{
					MaxReplicas:   50,
					ResplicasStep: 3,
					Threshold:     "{\"cpuMax\":80,\"cpuMin\":20,\"memMax\":70,\"memMin\":20,\"qpsMax\":85,\"qpsMin\":15}",
				},
			},
			SchedulePolicy: appsv1alpha1.SchedulePolicy{
				Level: map[appsv1alpha1.SchedulePolicyLevel]*metav1.LabelSelector{
					appsv1alpha1.SchedulePolicyMandatory: {
						MatchExpressions: []metav1.LabelSelectorRequirement{
							0: {Key: "geo-location", Operator: "In", Values: []string{"China-Huadong-Jiangsu-City-C21-District-E21", "China-Huadong-Jiangsu-City-C21-District-E22"}},
						},
					},
					appsv1alpha1.SchedulePolicyBestEffort: {
						MatchExpressions: []metav1.LabelSelectorRequirement{
							0: {Key: "netenvironment", Operator: "In", Values: []string{"edge"}},
						},
					},
				},
			},
		},
		5: {
			Name:        "case7-component6",
			RuntimeType: string(appsv1alpha1.Runc),
			Module:      corev1.PodTemplateSpec{},
			Workload: appsv1alpha1.Workload{
				Workloadtype: appsv1alpha1.WorkloadTypeServerless,
				TraitServerless: &lmmserverless.TraitServerless{
					MaxQPS:    100,
					QpsStep:   1,
					Threshold: "{\"qpsMax\":80,\"qpsMin\":20}",
				},
			},
			SchedulePolicy: appsv1alpha1.SchedulePolicy{
				Level: map[appsv1alpha1.SchedulePolicyLevel]*metav1.LabelSelector{},
			},
		},
		6: {
			Name:        "case7-component7",
			RuntimeType: string(appsv1alpha1.Runc),
			Module:      corev1.PodTemplateSpec{},
			Workload: appsv1alpha1.Workload{
				Workloadtype: appsv1alpha1.WorkloadTypeDeployment,
				TraitDeployment: &appsv1alpha1.TraitDeployment{
					Replicas: 10,
				},
			},
			SchedulePolicy: appsv1alpha1.SchedulePolicy{
				Level: map[appsv1alpha1.SchedulePolicyLevel]*metav1.LabelSelector{},
			},
		},
		7: {
			Name:        "case7-component8",
			RuntimeType: string(appsv1alpha1.Runc),
			Module:      corev1.PodTemplateSpec{},
			Workload: appsv1alpha1.Workload{
				Workloadtype: appsv1alpha1.WorkloadTypeTask,
				TraitTask: &appsv1alpha1.TraitTask{
					Completions: 899,
				},
			},
			SchedulePolicy: appsv1alpha1.SchedulePolicy{
				Level: map[appsv1alpha1.SchedulePolicyLevel]*metav1.LabelSelector{},
			},
		},
	}
	return components
}

func TestDescToComponents(t *testing.T) {
	oldDescComponents := SetOldDescComponents()
	type args struct {
		desc *appsv1alpha1.Description
	}
	tests := []struct {
		name            string
		args            args
		wantComponents  []appsv1alpha1.Component
		wantComLocation map[string]int
		wantAffinity    []int
	}{
		0: {
			name: "test1",
			args: args{desc: &appsv1alpha1.Description{
				Spec: appsv1alpha1.DescriptionSpec{
					AppID: "case7",
					WorkloadComponents: []appsv1alpha1.WorkloadComponent{
						0: {
							ComponentName: "case7-component1",
							Sandbox:       appsv1alpha1.Kata,
							Module:        corev1.PodTemplateSpec{},
							WorkloadType:  "stateful-system-service",
						},
						1: {
							ComponentName: "case7-component2",
							Sandbox:       appsv1alpha1.Runc,
							Module:        corev1.PodTemplateSpec{},
							WorkloadType:  "stateless-user-service",
						},
						2: {
							ComponentName: "case7-component3",
							Sandbox:       appsv1alpha1.Runc,
							Module:        corev1.PodTemplateSpec{},
							WorkloadType:  "stateless-system-service",
						},
						3: {
							ComponentName: "case7-component4",
							Sandbox:       appsv1alpha1.Runc,
							Module:        corev1.PodTemplateSpec{},
							WorkloadType:  "stateless-system-service",
						},
						4: {
							ComponentName: "case7-component5",
							Sandbox:       appsv1alpha1.Runc,
							Module:        corev1.PodTemplateSpec{},
							WorkloadType:  "stateless-system-service",
						},
						5: {
							ComponentName: "case7-component6",
							Sandbox:       appsv1alpha1.Runc,
							Module:        corev1.PodTemplateSpec{},
							WorkloadType:  "stateless-system-service",
						},
						6: {
							ComponentName: "case7-component7",
							Sandbox:       appsv1alpha1.Runc,
							Module:        corev1.PodTemplateSpec{},
							WorkloadType:  "stateless-system-service",
						},
						7: {
							ComponentName: "case7-component8",
							Sandbox:       appsv1alpha1.Runc,
							Module:        corev1.PodTemplateSpec{},
							WorkloadType:  "stateless-task-service",
						},
					},
					DeploymentCondition: appsv1alpha1.DeploymentCondition{
						Mandatory: []appsv1alpha1.Condition{
							0: {
								Subject: appsv1alpha1.Xject{
									Name: "case7-component1",
									Type: "component",
								},
								Object: appsv1alpha1.Xject{
									Name: "geo-location",
									Type: "label",
								},
								Relation: "In",
								Extent:   []byte("[\"China-Huadong-Jiangsu-City-C21-District-E21\"]"),
							},
							1: {
								Subject: appsv1alpha1.Xject{
									Name: "case7-component1",
									Type: "component",
								},
								Object: appsv1alpha1.Xject{
									Name: "netenvironment",
									Type: "label",
								},
								Relation: "In",
								Extent:   []byte("[\"edge\"]"),
							},
							2: {
								Subject: appsv1alpha1.Xject{
									Name: "case7-component5",
									Type: "component",
								},
								Object: appsv1alpha1.Xject{
									Name: "geo-location",
									Type: "label",
								},
								Relation: "In",
								Extent:   []byte("[\"China-Huadong-Jiangsu-City-C21-District-E21\",\"China-Huadong-Jiangsu-City-C21-District-E22\"]"),
							},
							3: {
								Subject: appsv1alpha1.Xject{
									Name: "case7-component4",
									Type: "component",
								},
								Object: appsv1alpha1.Xject{
									Name: "case7-component3",
									Type: "component",
								},
								Relation: "Affinity",
								Extent:   []byte(""),
							},
							4: {
								Subject: appsv1alpha1.Xject{
									Name: "case7-component2",
									Type: "component",
								},
								Object: appsv1alpha1.Xject{
									Name: "sn",
									Type: "label",
								},
								Relation: "In",
								Extent:   []byte("[\"sn1\"]"),
							},
							5: {
								Subject: appsv1alpha1.Xject{
									Name: "case7-component3",
									Type: "component",
								},
								Object: appsv1alpha1.Xject{
									Name: "sn",
									Type: "label",
								},
								Relation: "In",
								Extent:   []byte("[\"sn1\",\"sn2\"]"),
							},
						},
						BestEffort: []appsv1alpha1.Condition{
							0: {
								Subject: appsv1alpha1.Xject{
									Name: "case7-component5",
									Type: "component",
								},
								Object: appsv1alpha1.Xject{
									Name: "netenvironment",
									Type: "label",
								},
								Relation: "In",
								Extent:   []byte("[\"edge\"]"),
							},
						},
					},
					ExpectedPerformance: appsv1alpha1.ExpectedPerformance{
						Boundaries: appsv1alpha1.Boundaries{
							Inner: []appsv1alpha1.Boundary{
								0: {
									Name:    "boundary5",
									Subject: "case7-component4",
									Type:    "maxReplicas",
									Value:   []byte("1000"),
								},
								1: {
									Name:    "boundary6",
									Subject: "case7-component5",
									Type:    "cpu",
									Value:   []byte("20"),
								},
								2: {
									Name:    "boundary7",
									Subject: "case7-component5",
									Type:    "cpu",
									Value:   []byte("80"),
								},
								3: {
									Name:    "boundary8",
									Subject: "case7-component5",
									Type:    "mem",
									Value:   []byte("20"),
								},
								4: {
									Name:    "boundary9",
									Subject: "case7-component5",
									Type:    "mem",
									Value:   []byte("70"),
								},
								5: {
									Name:    "boundary10",
									Subject: "case7-component5",
									Type:    "QPS",
									Value:   []byte("15"),
								},
								6: {
									Name:    "boundary11",
									Subject: "case7-component5",
									Type:    "QPS",
									Value:   []byte("85"),
								},
								7: {
									Name:    "boundary1",
									Subject: "case7-component3",
									Type:    "daemonset",
									Value:   []byte("true"),
								},
								8: {
									Name:    "boundary2",
									Subject: "case7-component6",
									Type:    "maxQPS",
									Value:   []byte("100"),
								},
								9: {
									Name:    "boundary3",
									Subject: "case7-component7",
									Type:    "replicas",
									Value:   []byte("10"),
								},
								10: {
									Name:    "boundary4",
									Subject: "case7-component8",
									Type:    "replicas",
									Value:   []byte("899"),
								},
								11: {
									Name:    "boundary12",
									Subject: "case7-component1",
									Type:    "replicas",
									Value:   []byte("199"),
								},
								12: {
									Name:    "boundary13",
									Subject: "case7-component4",
									Type:    "cpu",
									Value:   []byte("25"),
								},
								13: {
									Name:    "boundary14",
									Subject: "case7-component4",
									Type:    "cpu",
									Value:   []byte("75"),
								},
								14: {
									Name:    "boundary15",
									Subject: "case7-component4",
									Type:    "mem",
									Value:   []byte("25"),
								},
								15: {
									Name:    "boundary16",
									Subject: "case7-component4",
									Type:    "mem",
									Value:   []byte("85"),
								},
								16: {
									Name:    "boundary17",
									Subject: "case7-component6",
									Type:    "QPS",
									Value:   []byte("20"),
								},
								17: {
									Name:    "boundary18",
									Subject: "case7-component6",
									Type:    "QPS",
									Value:   []byte("80"),
								},
								18: {
									Name:    "boundary19",
									Subject: "case7-component5",
									Type:    "maxReplicas",
									Value:   []byte("50"),
								},
							},
						},
						Maintenance: appsv1alpha1.Maintenance{
							HPA: []appsv1alpha1.XPA{
								0: {
									Name:    "decrease replicas1",
									Subject: "case7-component5",
									Trigger: "boundary6 && boundary8 && boundary10",
									Strategy: appsv1alpha1.XPAStrategy{
										Type:  "decrease",
										Value: []byte("3"),
									},
								},
								1: {
									Name:    "increase replicas1",
									Subject: "case7-component5",
									Trigger: "boundary7 || boundary9 || boundary11",
									Strategy: appsv1alpha1.XPAStrategy{
										Type:  "increase",
										Value: []byte("3"),
									},
								},
								2: {
									Name:    "decrease replicas 1",
									Subject: "case7-component4",
									Trigger: "boundary13",
									Strategy: appsv1alpha1.XPAStrategy{
										Type:  "decrease",
										Value: []byte("1"),
									},
								},
								3: {
									Name:    "increase replicas1",
									Subject: "case7-component4",
									Trigger: "boundary14",
									Strategy: appsv1alpha1.XPAStrategy{
										Type:  "increase",
										Value: []byte("1"),
									},
								},
								4: {
									Name:    "decrease replicas 1",
									Subject: "case7-component4",
									Trigger: "boundary15",
									Strategy: appsv1alpha1.XPAStrategy{
										Type:  "decrease",
										Value: []byte("1"),
									},
								},
								5: {
									Name:    "increase replicas1",
									Subject: "case7-component4",
									Trigger: "boundary16",
									Strategy: appsv1alpha1.XPAStrategy{
										Type:  "increase",
										Value: []byte("1"),
									},
								},
								6: {
									Name:    "decrease replicas 1",
									Subject: "case7-component6",
									Trigger: "boundary17",
									Strategy: appsv1alpha1.XPAStrategy{
										Type:  "decrease",
										Value: []byte("1"),
									},
								},
								7: {
									Name:    "increase replicas1",
									Subject: "case7-component6",
									Trigger: "boundary18",
									Strategy: appsv1alpha1.XPAStrategy{
										Type:  "increase",
										Value: []byte("1"),
									},
								},
							},
						},
					},
				}},
			},
			wantAffinity: []int{0, 1, 2, 2, 4, 5, 6, 7},
			wantComLocation: map[string]int{
				"case7-component1": 0,
				"case7-component2": 1,
				"case7-component3": 2,
				"case7-component4": 3,
				"case7-component5": 4,
				"case7-component6": 5,
				"case7-component7": 6,
				"case7-component8": 7,
			},
			wantComponents: oldDescComponents,
		},
		// TODO: Add test cases.

	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotComponents, gotComLocation, gotAffinity := DescToComponents(tt.args.desc)
			//for i, comn := range gotComponents {
			//	if !reflect.DeepEqual(comn, oldDescComponents[i]) {
			//		t.Errorf("   desc:com:%v = %+v", comn.Name, comn)
			//		t.Errorf("olddesc:com:%v = %+v", oldDescComponents[i].Name, oldDescComponents[i])
			//	}
			//}
			if !reflect.DeepEqual(gotComponents, tt.wantComponents) {
				t.Errorf("DescToComponents() gotComponents = %v, want %v", gotComponents, tt.wantComponents)
			}
			if !reflect.DeepEqual(gotComLocation, tt.wantComLocation) {
				t.Errorf("DescToComponents() gotComLocation = %v, want %v", gotComLocation, tt.wantComLocation)
			}
			if !reflect.DeepEqual(gotAffinity, tt.wantAffinity) {
				t.Errorf("DescToComponents() gotAffinity = %v, want %v", gotAffinity, tt.wantAffinity)
			}
		})
	}
}

func TestGetWorkloadType(t *testing.T) {
	type args struct {
		workloadType string
	}
	tests := []struct {
		name         string
		args         args
		wantWorkload appsv1alpha1.Workload
	}{
		// TODO: Add test cases.
		0: {
			name: "userappWorkloadTypeTest0",
			args: args{"stateful-user-service"},
			wantWorkload: appsv1alpha1.Workload{
				Workloadtype: appsv1alpha1.WorkloadTypeUserApp,
				TraitUserAPP: &appsv1alpha1.TraitUserAPP{
					SN: "",
				},
			},
		},
		1: {
			name: "userappWorkloadTypeTest1",
			args: args{"stateless-user-service"},
			wantWorkload: appsv1alpha1.Workload{
				Workloadtype: appsv1alpha1.WorkloadTypeUserApp,
				TraitUserAPP: &appsv1alpha1.TraitUserAPP{
					SN: "",
				},
			},
		},
		2: {
			name: "taskWorkloadTypeTest0",
			args: args{"stateless-system-task"},
			wantWorkload: appsv1alpha1.Workload{
				Workloadtype: appsv1alpha1.WorkloadTypeTask,
				TraitTask: &appsv1alpha1.TraitTask{
					Completions: 1,
				},
			},
		},
		3: {
			name: "taskWorkloadTypeTest1",
			args: args{"stateful-system-task"},
			wantWorkload: appsv1alpha1.Workload{
				Workloadtype: appsv1alpha1.WorkloadTypeTask,
				TraitTask: &appsv1alpha1.TraitTask{
					Completions: 1,
				},
			},
		},
		4: {
			name: "statefulsetworkloadtypeTest0",
			args: args{
				workloadType: "stateful-system-service",
			},
			wantWorkload: appsv1alpha1.Workload{
				Workloadtype: appsv1alpha1.WorkloadTypeStatefulSet,
				TraitStatefulSet: &appsv1alpha1.TraitStatefulSet{
					Replicas: 1,
				},
			},
		},
		5: {
			name: "othersworkloadtypeTest0",
			args: args{
				workloadType: "stateless-system-service",
			},
			wantWorkload: appsv1alpha1.Workload{
				Workloadtype: appsv1alpha1.WorkloadTypeDeployment,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotWorkload := GetWorkloadType(tt.args.workloadType); !reflect.DeepEqual(gotWorkload, tt.wantWorkload) {
				t.Errorf("GetWorkloadType() = %v, want %v", gotWorkload, tt.wantWorkload)
			}
		})
	}
}

func TestUnmarshalExtent(t *testing.T) {
	type args struct {
		b []byte
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		// TODO: Add test cases.
		0: {
			name: "geolocation",
			args: args{[]byte("[\"China-Huadong-Jiangsu-City-C21-District-E21\"]")},
			want: []string{"China-Huadong-Jiangsu-City-C21-District-E21"},
		},
		1: {
			name: "geolocation1",
			args: args{[]byte("[\"China-Huadong-Jiangsu-City-C21-District-E21\", \"China-Huadong-Jiangsu-City-C21-District-E22\"]")},
			want: []string{"China-Huadong-Jiangsu-City-C21-District-E21", "China-Huadong-Jiangsu-City-C21-District-E22"},
		},
		2: {
			name: "netenvironment",
			args: args{[]byte("[\"edge\"]")},
			want: []string{"edge"},
		},
		3: {
			name: "sn",
			args: args{[]byte("[\"sn1\", \"sn2\"]")},
			want: []string{"sn1", "sn2"},
		},
		4: {
			name: "provider",
			args: args{[]byte("[\"Alibaba\", \"Tencent\"]")},
			want: []string{"Alibaba", "Tencent"},
		},
	}
	var extentStr []string
	for _, tt := range tests {
		_ = json.Unmarshal(tt.args.b, &extentStr)
		t.Logf("after Unmarshal , extentStr is %+v", extentStr)
		t.Run(tt.name, func(t *testing.T) {
			if got := extentStr; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseExtent() = %v, want %v", got, tt.want)
			}
		})
	}
}
