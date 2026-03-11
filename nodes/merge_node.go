package nodes

import (
	"encoding/json"
	"fmt"

	"capyflow/api/models"
)

type MergeNode struct{}

type MergeParams struct {
	Mode   string      `json:"mode"`
	Input1 interface{} `json:"input1"`
	Input2 interface{} `json:"input2"`
	Input3 interface{} `json:"input3"`
	Input4 interface{} `json:"input4"`
}

func (n *MergeNode) Execute(
	node *models.Node,
	prev map[string]map[string]interface{},
) (map[string]interface{}, error) {

	var params MergeParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, fmt.Errorf("invalid merge parameters: %v", err)
	}

	// Get merge mode
	mode := params.Mode
	if mode == "" {
		mode = "append" // append, combine, or merge
	}

	// Collect all inputs
	var inputs []interface{}
	
	if params.Input1 != nil && params.Input1 != "" {
		inputs = append(inputs, params.Input1)
	}
	if params.Input2 != nil && params.Input2 != "" {
		inputs = append(inputs, params.Input2)
	}
	if params.Input3 != nil && params.Input3 != "" {
		inputs = append(inputs, params.Input3)
	}
	if params.Input4 != nil && params.Input4 != "" {
		inputs = append(inputs, params.Input4)
	}

	if len(inputs) == 0 {
		return nil, fmt.Errorf("at least one input is required")
	}

	var result interface{}

	switch mode {
	case "append":
		// Append all arrays together
		var merged []interface{}
		for _, input := range inputs {
			arr := toArray(input)
			merged = append(merged, arr...)
		}
		result = merged

	case "combine":
		// Keep arrays separate in object
		combined := make(map[string]interface{})
		for i, input := range inputs {
			key := fmt.Sprintf("input%d", i+1)
			combined[key] = input
		}
		result = combined

	case "merge":
		// Merge objects together (last wins for duplicate keys)
		merged := make(map[string]interface{})
		for _, input := range inputs {
			obj := toObject(input)
			for k, v := range obj {
				merged[k] = v
			}
		}
		result = merged

	default:
		return nil, fmt.Errorf("invalid mode: %s", mode)
	}

	return map[string]interface{}{
		"merged": result,
		"count":  len(inputs),
		"mode":   mode,
	}, nil
}

// toArray converts various types to array
func toArray(input interface{}) []interface{} {
	switch v := input.(type) {
	case []interface{}:
		return v
	case string:
		// Try to parse as JSON array
		var parsed interface{}
		if err := json.Unmarshal([]byte(v), &parsed); err == nil {
			if arr, ok := parsed.([]interface{}); ok {
				return arr
			}
		}
		// If not JSON, return string as single element
		return []interface{}{v}
	default:
		// Wrap in array
		return []interface{}{v}
	}
}

// toObject converts various types to object
func toObject(input interface{}) map[string]interface{} {
	switch v := input.(type) {
	case map[string]interface{}:
		return v
	case string:
		// Try to parse as JSON object
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(v), &parsed); err == nil {
			return parsed
		}
		// If not JSON, create object with value
		return map[string]interface{}{"value": v}
	default:
		// Create object with value
		return map[string]interface{}{"value": v}
	}
}
