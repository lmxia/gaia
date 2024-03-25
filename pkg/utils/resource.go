package utils

import (
	"context"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"k8s.io/klog/v2"
)

// GetDataFromPrometheus returns the result from Prometheus according to the specified metric in the cluster
func GetDataFromPrometheus(promPreURL, metric string) (model.Value, error) {
	client, err := api.NewClient(api.Config{
		Address: promPreURL,
	})
	if err != nil {
		klog.Warningf("Error creating client: %v", err)
		return nil, err
	}

	v1api := prometheusv1.NewAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, warnings, err := v1api.Query(ctx, metric, time.Now())
	if err != nil {
		klog.Warningf("Error querying Prometheus: %v", err)
		return nil, err
	}
	if len(warnings) > 0 {
		klog.Warningf("Warnings: %v\n", warnings)
	}

	return result, nil
}

// GetSubStringWithSpecifiedDecimalPlace returns a sub string based on the specified number of decimal places
func GetSubStringWithSpecifiedDecimalPlace(inputString string, m int) string {
	if inputString == "" {
		return ""
	}
	if m >= len(inputString) {
		return inputString
	}
	newString := strings.Split(inputString, ".")
	if len(newString) < 2 || m >= len(newString[1]) {
		return inputString
	}
	return newString[0] + "." + newString[1][:m]
}
