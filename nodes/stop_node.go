package nodes

import (
	"encoding/json"
	"fmt"

	"capyflow/api/models"
)

type StopNode struct{}

type StopParams struct {
	Condition   string      `json:"condition"`   // "always", "if-true", "if-false", "if-error"
	InputValue  interface{} `json:"inputValue"`  // Value to evaluate
	StopMessage string      `json:"stopMessage"` // Custom message when stopping
	StopCode    string      `json:"stopCode"`    // "success" or "error"
}

func (n *StopNode) Execute(node *models.Node, prev map[string]map[string]interface{}) (map[string]interface{}, error) {
	var params StopParams
	paramsJSON, _ := json.Marshal(node.Parameters)
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, fmt.Errorf("error parsing parameters: %v", err)
	}

	// Default values
	if params.Condition == "" {
		params.Condition = "always"
	}
	if params.StopCode == "" {
		params.StopCode = "success"
	}
	if params.StopMessage == "" {
		params.StopMessage = "Workflow stopped"
	}

	shouldStop := false

	// Evaluate condition
	switch params.Condition {
	case "always":
		shouldStop = true

	case "if-true":
		// Check if inputValue is truthy
		shouldStop = isTruthy(params.InputValue)

	case "if-false":
		// Check if inputValue is falsy
		shouldStop = !isTruthy(params.InputValue)

	case "if-error":
		// Check if inputValue contains an error
		if inputMap, ok := params.InputValue.(map[string]interface{}); ok {
			if errVal, exists := inputMap["error"]; exists && errVal != nil {
				shouldStop = true
			}
		}

	default:
		return nil, fmt.Errorf("invalid condition: %s", params.Condition)
	}

	result := map[string]interface{}{
		"stopped":    shouldStop,
		"condition":  params.Condition,
		"message":    params.StopMessage,
		"code":       params.StopCode,
		"inputValue": params.InputValue,
	}

	// If should stop, return an error to halt execution
	if shouldStop {
		if params.StopCode == "error" {
			return result, fmt.Errorf("STOP: %s", params.StopMessage)
		}
		// For success stop, we return a special marker in the result
		result["__stop__"] = true
	}

	return result, nil
}

// isTruthy evaluates if a value is truthy
func isTruthy(value interface{}) bool {
	if value == nil {
		return false
	}

	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v != "" && v != "false" && v != "0"
	case int, int64, float64:
		return fmt.Sprintf("%v", v) != "0"
	case map[string]interface{}:
		return len(v) > 0
	case []interface{}:
		return len(v) > 0
	default:
		return true
	}
}
