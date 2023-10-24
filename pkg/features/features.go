package features

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"
)

const (
	AbnormalScheduler featuregate.Feature = "AbnormalScheduler"
)

var (
	DefaultMutableFeatureGate featuregate.MutableFeatureGate = featuregate.NewFeatureGate()
	// DefaultFeatureGate        featuregate.FeatureGate        = DefaultMutableFeatureGate
	DefaultVectorFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
		AbnormalScheduler: {Default: false, PreRelease: featuregate.Alpha},
	}
)

func init() {
	runtime.Must(DefaultMutableFeatureGate.Add(DefaultVectorFeatureGates))
}
