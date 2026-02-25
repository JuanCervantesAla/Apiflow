package nodes

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"capyflow/api/models"
)

type SortNode struct{}

type SortParams struct {
	InputData interface{} `json:"inputData"`
	Field     string      `json:"field"`
	Order     string      `json:"order"` // "asc" or "desc"
	Type      string      `json:"type"`  // "string", "number", "date"
}

func (n *SortNode) Execute(node *models.Node, prev map[string]map[string]interface{}) (map[string]interface{}, error) {
	var params SortParams
	paramsJSON, _ := json.Marshal(node.Parameters)
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, fmt.Errorf("error parsing parameters: %v", err)
	}

	// Get input data
	var inputData []interface{}
	switch v := params.InputData.(type) {
	case []interface{}:
		inputData = v
	case string:
		if err := json.Unmarshal([]byte(v), &inputData); err != nil {
			return nil, fmt.Errorf("input data must be an array: %v", err)
		}
	default:
		return nil, fmt.Errorf("input data must be an array")
	}

	if len(inputData) == 0 {
		return map[string]interface{}{
			"sorted":   []interface{}{},
			"count":    0,
			"field":    params.Field,
			"order":    params.Order,
		}, nil
	}

	// Validate parameters
	if params.Order == "" {
		params.Order = "asc"
	}
	if params.Order != "asc" && params.Order != "desc" {
		return nil, fmt.Errorf("order must be 'asc' or 'desc'")
	}
	if params.Type == "" {
		params.Type = "string"
	}

	// Clone array to avoid modifying original
	sortedData := make([]interface{}, len(inputData))
	copy(sortedData, inputData)

	// Sort the array
	sort.SliceStable(sortedData, func(i, j int) bool {
		var valI, valJ interface{}

		// Extract values
		if params.Field != "" {
			// Sort by field
			objI, okI := sortedData[i].(map[string]interface{})
			objJ, okJ := sortedData[j].(map[string]interface{})
			if !okI || !okJ {
				return false
			}
			valI = objI[params.Field]
			valJ = objJ[params.Field]
		} else {
			// Sort primitive values directly
			valI = sortedData[i]
			valJ = sortedData[j]
		}

		// Compare based on type
		result := compareValues(valI, valJ, params.Type)
		
		if params.Order == "desc" {
			return result > 0
		}
		return result < 0
	})

	return map[string]interface{}{
		"sorted": sortedData,
		"count":  len(sortedData),
		"field":  params.Field,
		"order":  params.Order,
	}, nil
}

// compareValues compares two values based on type
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func compareValues(a, b interface{}, valueType string) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}

	switch valueType {
	case "number":
		numA := toFloat64(a)
		numB := toFloat64(b)
		if numA < numB {
			return -1
		} else if numA > numB {
			return 1
		}
		return 0

	case "string":
		strA := fmt.Sprintf("%v", a)
		strB := fmt.Sprintf("%v", b)
		return strings.Compare(strings.ToLower(strA), strings.ToLower(strB))

	case "date":
		// Try to parse as ISO date string
		strA := fmt.Sprintf("%v", a)
		strB := fmt.Sprintf("%v", b)
		return strings.Compare(strA, strB)

	default:
		// Default to string comparison
		strA := fmt.Sprintf("%v", a)
		strB := fmt.Sprintf("%v", b)
		return strings.Compare(strA, strB)
	}
}

// toFloat64 converts interface{} to float64
func toFloat64(val interface{}) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		// Try to parse string as number
		var num float64
		fmt.Sscanf(v, "%f", &num)
		return num
	default:
		return 0
	}
}
