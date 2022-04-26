package interfaces

import (
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	appsapi "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
)

// Diagnosis records the details to diagnose a scheduling failure.
type Diagnosis struct {
	ClusterToStatusMap   ClusterToStatusMap
	UnschedulablePlugins sets.String
}

// FitError describes a fit error of a subscription.
type FitError struct {
	Description    *appsapi.Description
	NumAllClusters int
	Diagnosis      Diagnosis
}

const (
	// NoClusterAvailableMsg is used to format message when no clusters available.
	NoClusterAvailableMsg = "0/%v clusters are available"
)

// Error returns detailed information of why the subscription failed to fit on each cluster
func (f *FitError) Error() string {
	reasons := make(map[string]int)
	for _, status := range f.Diagnosis.ClusterToStatusMap {
		for _, reason := range status.Reasons() {
			reasons[reason]++
		}
	}

	sortReasonsHistogram := func() []string {
		var reasonStrings []string
		for k, v := range reasons {
			reasonStrings = append(reasonStrings, fmt.Sprintf("%v %v", v, k))
		}
		sort.Strings(reasonStrings)
		return reasonStrings
	}
	reasonMsg := fmt.Sprintf(NoClusterAvailableMsg+": %v.", f.NumAllClusters, strings.Join(sortReasonsHistogram(), ", "))
	return reasonMsg
}
