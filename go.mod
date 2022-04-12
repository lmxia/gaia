module github.com/lmxia/gaia

go 1.14

require (
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/common v0.28.0
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.23.5
	k8s.io/apimachinery v0.23.5
	k8s.io/client-go v0.23.5
	k8s.io/code-generator v0.23.5
	k8s.io/klog/v2 v2.30.0
	k8s.io/utils v0.0.0-20211116205334-6203023598ed
	sigs.k8s.io/controller-runtime v0.11.2
	sigs.k8s.io/yaml v1.3.0
)

replace github.com/spf13/afero => github.com/spf13/afero v1.5.1
