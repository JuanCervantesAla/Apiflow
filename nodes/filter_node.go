package nodes

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"capyflow/api/models"
)

type FilterNode struct{}

type FilterParams struct {
	InputData interface{} `json:"inputData"`
	Mode      string      `json:"mode"`
	Field     string      `json:"field"`
	Operator  string      `json:"operator"`
	Value     interface{} `json:"value"`
}

func (n *FilterNode) Execute(
	node *models.Node,
	prev map[string]map[string]interface{},
) (map[string]interface{}, error) {

	var params FilterParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, fmt.Errorf("invalid filter parameters: %v", err)
	}

	// Get input data
	inputData := params.InputData
	if inputData == nil {
		return nil, fmt.Errorf("inputData parameter is required")
	}

	// Get filter mode
	mode := params.Mode
	if mode == "" {
		mode = "keep" // keep or remove
	}

	// Get condition field
	field := params.Field
	
	// Get operator
	operator := params.Operator
	if operator == "" {
		operator = "equals"
	}

	// Get value to compare
	compareValue := params.Value

	// Handle input data - could be array or object
	var dataArray []interface{}
	
	switch v := inputData.(type) {
	case []interface{}:
		dataArray = v
	case string:
		// Try to parse as JSON
		var parsed interface{}
		if err := json.Unmarshal([]byte(v), &parsed); err == nil {
			if arr, ok := parsed.([]interface{}); ok {
				dataArray = arr
			} else {
				return nil, fmt.Errorf("input data must be an array")
			}
		} else {
			return nil, fmt.Errorf("invalid input data: %v", err)
		}
	default:
		return nil, fmt.Errorf("input data must be an array")
	}

	// Filter the array
	var filtered []interface{}
	for _, item := range dataArray {
		matches := evaluateFilterCondition(item, field, operator, compareValue)
		
		if (mode == "keep" && matches) || (mode == "remove" && !matches) {
			filtered = append(filtered, item)
		}
	}

	return map[string]interface{}{
		"filtered": filtered,
		"count":    len(filtered),
		"original": len(dataArray),
	}, nil
}

// evaluateFilterCondition checks if an item matches the filter condition
func evaluateFilterCondition(item interface{}, field, operator string, compareValue interface{}) bool {
	var itemValue interface{}

	// Extract field value if specified
	if field != "" {
		if itemMap, ok := item.(map[string]interface{}); ok {
			itemValue = itemMap[field]
		} else {
			return false
		}
	} else {
		itemValue = item
	}

	// Convert to comparable types
	itemStr := fmt.Sprintf("%v", itemValue)
	compareStr := fmt.Sprintf("%v", compareValue)

	switch operator {
	case "equals":
		return itemStr == compareStr
	
	case "notEquals":
		return itemStr != compareStr
	
	case "contains":
		return strings.Contains(strings.ToLower(itemStr), strings.ToLower(compareStr))
	
	case "startsWith":
		return strings.HasPrefix(strings.ToLower(itemStr), strings.ToLower(compareStr))
	
	case "endsWith":
		return strings.HasSuffix(strings.ToLower(itemStr), strings.ToLower(compareStr))
	
	case "greaterThan":
		itemNum, err1 := strconv.ParseFloat(itemStr, 64)
		compareNum, err2 := strconv.ParseFloat(compareStr, 64)
		if err1 == nil && err2 == nil {
			return itemNum > compareNum
		}
		return false
	
	case "lessThan":
		itemNum, err1 := strconv.ParseFloat(itemStr, 64)
		compareNum, err2 := strconv.ParseFloat(compareStr, 64)
		if err1 == nil && err2 == nil {
			return itemNum < compareNum
		}
		return false
	
	case "greaterOrEqual":
		itemNum, err1 := strconv.ParseFloat(itemStr, 64)
		compareNum, err2 := strconv.ParseFloat(compareStr, 64)
		if err1 == nil && err2 == nil {
			return itemNum >= compareNum
		}
		return false
	
	case "lessOrEqual":
		itemNum, err1 := strconv.ParseFloat(itemStr, 64)
		compareNum, err2 := strconv.ParseFloat(compareStr, 64)
		if err1 == nil && err2 == nil {
			return itemNum <= compareNum
		}
		return false
	
	case "isEmpty":
		return itemStr == "" || itemStr == "null" || itemStr == "<nil>"
	
	case "isNotEmpty":
		return itemStr != "" && itemStr != "null" && itemStr != "<nil>"
	
	default:
		return false
	}
}
