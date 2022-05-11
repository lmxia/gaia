module github.com/lmxia/gaia

go 1.14

require (
	github.com/SUMMERLm/hyperNodes v0.0.0-20220411082828-8c56025edf7c
	github.com/google/go-cmp v0.5.7
	github.com/onsi/gomega v1.17.0 // indirect
	github.com/prometheus/client_golang v1.11.1
	github.com/prometheus/common v0.32.1
	github.com/spf13/cobra v1.3.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	gonum.org/v1/gonum v0.11.0
	k8s.io/api v0.23.5
	k8s.io/apimachinery v0.23.5
	k8s.io/client-go v0.23.5
	k8s.io/code-generator v0.23.5
	k8s.io/component-base v0.23.5
	k8s.io/component-helpers v0.23.5
	k8s.io/klog/v2 v2.60.1-0.20220317184644-43cc75f9ae89
	k8s.io/utils v0.0.0-20220210201930-3a6ce19ff2f9
	knative.dev/pkg v0.0.0-20220427171752-2d552be030f6 // indirect
	knative.dev/serving v0.31.0
	sigs.k8s.io/yaml v1.3.0
)

replace github.com/spf13/afero => github.com/spf13/afero v1.5.1
