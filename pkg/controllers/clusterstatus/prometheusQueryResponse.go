package clusterstatus

type PrometheusQueryResponse struct {
	// Status is the result of http request
	Status string                      `json:"status,omitempty"`
	Data   PrometheusQueryResponseData `json:"data,omitempty"`
}

type PrometheusQueryResponseData struct {
	// ResultType is the one of the "matrix", "vector", "scalar" or "string"
	ResultType string                              `json:"resultType,omitempty"`
	Result     []PrometheusQueryResponseDataResult `json:"result,omitempty"`
}

type PrometheusQueryResponseDataResult struct {
	// Metric is a struct that contains details of the prometheus metric
	Metric PrometheusQueryResponseDataResultMetric `json:"metric,omitempty"`
	Value  []string                                `json:"value,omitempty"`
}

type PrometheusQueryResponseDataResultMetric struct {
	// Name is a metric name
	Name      string `json:"__name__,omitempty"`
	Container string `json:"container,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
	Instance  string `json:"instance,omitempty"`
	Job       string `json:"job,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Node      string `json:"node,omitempty"`
	Pod       string `json:"pod,omitempty"`
	Resource  string `json:"resource,omitempty"`
	Service   string `json:"service,omitempty"`
	Unit      string `json:"unit,omitempty"`
}
