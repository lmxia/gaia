module github.com/lmxia/gaia

go 1.14

require (
	github.com/SUMMERLm/hyperNodes v0.0.0-20220411082828-8c56025edf7c
	github.com/SUMMERLm/serverless v0.0.0-20220609081834-791f80a7e62b
	github.com/antonfisher/nested-logrus-formatter v1.3.1
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.7
	github.com/lestrrat-go/file-rotatelogs v2.4.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.12.1
	github.com/prometheus/common v0.34.0
	github.com/rifflock/lfshook v0.0.0-20180920164130-b9218ef580f5
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.3.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.1
	github.com/timtadh/data-structures v0.5.3
	golang.org/x/exp v0.0.0-20200331195152-e8c3332aa8e5
	golang.org/x/oauth2 v0.0.0-20220309155454-6242fa91716a
	gonum.org/v1/gonum v0.11.0
	k8s.io/api v0.23.5
	k8s.io/apimachinery v0.23.5
	k8s.io/client-go v0.23.5
	k8s.io/code-generator v0.23.5
	k8s.io/component-base v0.23.5
	k8s.io/component-helpers v0.23.5
	k8s.io/klog/v2 v2.60.1-0.20220317184644-43cc75f9ae89
	k8s.io/utils v0.0.0-20220210201930-3a6ce19ff2f9
	knative.dev/serving v0.31.0
	sigs.k8s.io/yaml v1.3.0
)

replace github.com/spf13/afero => github.com/spf13/afero v1.5.1
