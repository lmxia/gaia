package runtimetype

import (
	"context"
	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
	framework "github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"testing"
)

func TestRuntimeType_Filter(t *testing.T) {
	type fields struct {
		handle interfaces.Handle
	}
	type args struct {
		ctx     context.Context
		com     *v1alpha1.Component
		cluster *clusterapi.ManagedCluster
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *framework.Status
	}{
		// TODO: Add test cases.
		{
			name: "success",
			args: args{
				com: &v1alpha1.Component{
					RuntimeType: "kata",
				},
				cluster: &clusterapi.ManagedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster1",
						Labels: map[string]string{
							clusterapi.ParsedRuntimeStateKey: "kata",
						},
					},
				},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pl := &RuntimeType{
				handle: tt.fields.handle,
			}
			if got := pl.Filter(tt.args.ctx, tt.args.com, tt.args.cluster); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Filter() = %v, want %v", got, tt.want)
			}
		})
	}
}
