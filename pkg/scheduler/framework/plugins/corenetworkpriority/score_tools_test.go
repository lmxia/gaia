package corenetworkpriority

import (
	"testing"

	v1alpha12 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ScoreSuite struct {
	clusters map[string]*v1alpha1.ManagedCluster
	rbApps   []*v1alpha12.ResourceBindingApps
	suite.Suite
}

func (suite *ScoreSuite) SetupTest() {
	suite.clusters = map[string]*v1alpha1.ManagedCluster{
		"cluster0": {
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster0",
				Labels: map[string]string{
					v1alpha1.ParsedNetEnvironmentKey: common.NetworkLocationCore,
				},
			},
		},
		"cluster1": {
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster1",
				Labels: map[string]string{
					v1alpha1.ParsedNetEnvironmentKey: common.NetworkLocationCore,
				},
			},
		},
		"field0": {
			ObjectMeta: metav1.ObjectMeta{
				Name: "field0",
				Labels: map[string]string{
					v1alpha1.ParsedNetEnvironmentKey: common.NetworkLocationCore,
				},
			},
		},
		"field1": {
			ObjectMeta: metav1.ObjectMeta{
				Name: "field1",
			},
		},
		"field2": {
			ObjectMeta: metav1.ObjectMeta{
				Name: "field2",
				Labels: map[string]string{
					v1alpha1.ParsedNetEnvironmentKey: common.NetworkLocationCore,
				},
			},
		},
	}
	suite.rbApps = []*v1alpha12.ResourceBindingApps{
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
			Children: []*v1alpha12.ResourceBindingApps{
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
	}
}

func (suite *ScoreSuite) TestCalculateScore() {
	score := calculateScore(0, suite.rbApps, suite.clusters)
	suite.Equal(score, 44, "well...")
}

func TestScore(t *testing.T) {
	suite.Run(t, new(ScoreSuite))
}
