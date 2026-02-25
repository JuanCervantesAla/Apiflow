package nodes

import (
	"encoding/json"
	"fmt"
	"time"

	"capyflow/api/models"
)

type ErrorHandlerNode struct{}

type ErrorHandlerParams struct {
	InputData    interface{} `json:"inputData"`
	MaxRetries   int         `json:"maxRetries"`   // Number of retry attempts
	RetryDelay   int         `json:"retryDelay"`   // Delay between retries in ms
	FallbackMode string      `json:"fallbackMode"` // "ignore", "default", "stop"
	DefaultValue interface{} `json:"defaultValue"` // Value to use if fallback is "default"
}

func (n *ErrorHandlerNode) Execute(node *models.Node, prev map[string]map[string]interface{}) (map[string]interface{}, error) {
	var params ErrorHandlerParams
	paramsJSON, _ := json.Marshal(node.Parameters)
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, fmt.Errorf("error parsing parameters: %v", err)
	}

	// Default values
	if params.MaxRetries == 0 {
		params.MaxRetries = 3
	}
	if params.RetryDelay == 0 {
		params.RetryDelay = 1000 // 1 second
	}
	if params.FallbackMode == "" {
		params.FallbackMode = "ignore"
	}

	// Check if there was an error in the input
	hasError := false
	errorMessage := ""

	// Check if inputData is an error object
	if inputMap, ok := params.InputData.(map[string]interface{}); ok {
		if errVal, exists := inputMap["error"]; exists && errVal != nil {
			hasError = true
			errorMessage = fmt.Sprintf("%v", errVal)
		}
	}

	result := map[string]interface{}{
		"hasError":     hasError,
		"errorMessage": errorMessage,
		"retries":      0,
		"fallbackUsed": false,
		"data":         params.InputData,
	}

	// If there's an error, handle it
	if hasError {
		// Simulate retry logic (in real implementation, this would retry the previous node)
		retryCount := 0
		for retryCount < params.MaxRetries {
			retryCount++

			// In production, this would actually retry the failed node
			// For now, we just simulate the delay
			if params.RetryDelay > 0 {
				time.Sleep(time.Duration(params.RetryDelay) * time.Millisecond)
			}

			// Check if retry succeeded (simulated - always fails for now)
			// In real implementation, this would re-execute the previous node
			succeeded := false

			if succeeded {
				result["hasError"] = false
				result["retries"] = retryCount
				break
			}
		}

		result["retries"] = retryCount

		// If retries exhausted, apply fallback
		if hasError {
			switch params.FallbackMode {
			case "default":
				result["data"] = params.DefaultValue
				result["fallbackUsed"] = true
				result["hasError"] = false
			case "ignore":
				result["data"] = nil
				result["fallbackUsed"] = true
				result["hasError"] = false
			case "stop":
				return nil, fmt.Errorf("error handler: %s (retries exhausted)", errorMessage)
			}
		}
	}

	return result, nil
}
