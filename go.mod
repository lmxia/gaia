module github.com/lmxia/gaia

go 1.14

require (
	github.com/google/go-cmp v0.5.7
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/onsi/gomega v1.17.0 // indirect
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/common v0.28.0
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/spf13/cobra v1.3.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8 // indirect
	gonum.org/v1/gonum v0.11.0
	k8s.io/api v0.23.5
	k8s.io/apimachinery v0.23.5
	k8s.io/client-go v0.23.5
	k8s.io/code-generator v0.23.5
	k8s.io/component-base v0.23.5
	k8s.io/component-helpers v0.23.5
	k8s.io/klog/v2 v2.30.0
	k8s.io/utils v0.0.0-20211116205334-6203023598ed
	sigs.k8s.io/controller-tools v0.8.0 // indirect
	sigs.k8s.io/yaml v1.3.0
)

replace github.com/spf13/afero => github.com/spf13/afero v1.5.1
