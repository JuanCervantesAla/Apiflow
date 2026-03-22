package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"fmt"
	"time"
)

type DelayNode struct{}

type DelayParams struct {
	Duration int    `json:"duration"` // Milliseconds
	Reason   string `json:"reason"`   // Optional: delay description
}

func (n *DelayNode) Execute(node *models.Node, context map[string]map[string]interface{}) (map[string]interface{}, error) {
	var params DelayParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, fmt.Errorf("error decoding parameters: %v", err)
	}

	// Validate duration
	if params.Duration <= 0 {
		return nil, fmt.Errorf("duration must be greater than 0ms")
	}

	// Maximum limit of 5 minutes (300000ms) to avoid excessive timeouts
	if params.Duration > 300000 {
		return nil, fmt.Errorf("maximum duration is 300000ms (5 minutes)")
	}

	// Execute the delay
	time.Sleep(time.Duration(params.Duration) * time.Millisecond)

	result := map[string]interface{}{
		"delayed":    true,
		"durationMs": params.Duration,
	}

	if params.Reason != "" {
		result["reason"] = params.Reason
	}

	return result, nil
}
