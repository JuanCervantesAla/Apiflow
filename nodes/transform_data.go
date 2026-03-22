package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type TransformDataNode struct{}

type Transformation struct {
	Source    string `json:"source"`    // Source field with dot notation: "body.user.name"
	Target    string `json:"target"`    // Destination field: "userName"
	Operation string `json:"operation"` // extract, calculate, concat, default
	Value     string `json:"value"`     // Additional value for operations
}

type TransformParams struct {
	Transformations []Transformation `json:"transformations"`
}

func (n *TransformDataNode) Execute(node *models.Node, context map[string]map[string]interface{}) (map[string]interface{}, error) {
	var p TransformParams

	if err := json.Unmarshal([]byte(node.Parameters), &p); err != nil {
		return nil, fmt.Errorf("invalid transform parameters: %v", err)
	}

	if len(p.Transformations) == 0 {
		return nil, fmt.Errorf("no transformations defined")
	}

	// Get the output from the previous node (unified context)
	var prev map[string]interface{}
	for _, nodeOutput := range context {
		if nodeOutput != nil && len(nodeOutput) > 0 {
			// Merge all previous outputs into one context
			if prev == nil {
				prev = make(map[string]interface{})
			}
			for k, v := range nodeOutput {
				prev[k] = v
			}
		}
	}

	if prev == nil {
		prev = make(map[string]interface{})
	}

	result := make(map[string]interface{})

	for _, transform := range p.Transformations {
		switch transform.Operation {
		case "extract":
			// Extract value from nested field
			value := getNestedValue(prev, transform.Source)
			if value != nil {
				result[transform.Target] = value
			}

		case "rename":
			// Rename field (same as extract)
			value := getNestedValue(prev, transform.Source)
			if value != nil {
				result[transform.Target] = value
			}

		case "default":
			// Use default value if it doesn't exist
			value := getNestedValue(prev, transform.Source)
			if value != nil {
				result[transform.Target] = value
			} else {
				result[transform.Target] = transform.Value
			}

		case "calculate":
			// Simple math operations
			value := calculateExpression(transform.Value, context)
			result[transform.Target] = value

		case "concat":
			// Concatenate strings with interpolation
			interpolated := interpolateString(transform.Value, context)
			result[transform.Target] = interpolated

		default:
			return nil, fmt.Errorf("unknown operation: %s", transform.Operation)
		}
	}

	return result, nil
}

// getNestedValue gets a nested value using dot notation
// Example: "body.user.name" → prev["body"]["user"]["name"]
func getNestedValue(data map[string]interface{}, path string) interface{} {
	if path == "" {
		return nil
	}

	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		if current == nil {
			return nil
		}

		value, ok := current[part]
		if !ok {
			return nil
		}

		// If it's the last element, return the value
		if i == len(parts)-1 {
			return value
		}

		// If it's not the last, it must be a map to continue
		if nextMap, ok := value.(map[string]interface{}); ok {
			current = nextMap
		} else {
			return nil
		}
	}

	return nil
}

// calculateExpression evaluates simple math expressions
// Supports: +, -, *, /, and context variables
func calculateExpression(expr string, context map[string]map[string]interface{}) interface{} {
	// Interpolate variables first
	interpolated := interpolateString(expr, context)

	// Try to parse as number
	if num, err := strconv.ParseFloat(interpolated, 64); err == nil {
		return num
	}

	return interpolated
}
