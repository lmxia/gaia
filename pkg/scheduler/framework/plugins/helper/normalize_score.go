// This file was copied from k8s.io/kubernetes/pkg/scheduler/framework/plugins/helper/normalize_score.go and modified

package helper

import (
	framework "github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
)

// DefaultNormalizeScore generates a Normalize Score function that can normalize the
// scores to [0, maxPriority]. If reverse is set to true, it reverses the scores by
// subtracting it from maxPriority.
func DefaultNormalizeScore(maxPriority int64, reverse bool, scores framework.ClusterScoreList) *framework.Status {
	var maxCount int64
	for i := range scores {
		if scores[i].Score > maxCount {
			maxCount = scores[i].Score
		}
	}

	if maxCount == 0 {
		if reverse {
			for i := range scores {
				scores[i].Score = maxPriority
			}
		}
		return nil
	}

	for i := range scores {
		score := scores[i].Score

		score = maxPriority * score / maxCount
		if reverse {
			score = maxPriority - score
		}

		scores[i].Score = score
	}
	return nil
}
