package nodes

import (
	"encoding/json"
	"fmt"

	"capyflow/api/models"
)

type SplitNode struct{}

type SplitParams struct {
	InputData interface{} `json:"inputData"`
	Mode      string      `json:"mode"`
	BatchSize int         `json:"batchSize"`
	Field     string      `json:"field"`
}

func (n *SplitNode) Execute(
	node *models.Node,
	prev map[string]map[string]interface{},
) (map[string]interface{}, error) {

	var params SplitParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, fmt.Errorf("invalid split parameters: %v", err)
	}

	// Get input data
	inputData := params.InputData
	if inputData == nil {
		return nil, fmt.Errorf("inputData parameter is required")
	}

	// Get split mode
	mode := params.Mode
	if mode == "" {
		mode = "items" // items, batches, or field
	}

	// Get batch size (for batch mode)
	batchSize := params.BatchSize
	if batchSize < 1 {
		batchSize = 1
	}

	// Get field to extract (for field mode)
	field := params.Field

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

	var items []interface{}

	switch mode {
	case "items":
		// Split into individual items
		items = dataArray

	case "batches":
		// Split into batches
		for i := 0; i < len(dataArray); i += batchSize {
			end := i + batchSize
			if end > len(dataArray) {
				end = len(dataArray)
			}
			batch := dataArray[i:end]
			items = append(items, batch)
		}

	case "field":
		// Extract specific field from each item
		if field == "" {
			return nil, fmt.Errorf("field parameter is required for field mode")
		}
		for _, item := range dataArray {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if value, exists := itemMap[field]; exists {
					items = append(items, value)
				}
			}
		}

	default:
		return nil, fmt.Errorf("invalid mode: %s", mode)
	}

	return map[string]interface{}{
		"items":     items,
		"count":     len(items),
		"mode":      mode,
		"batchSize": batchSize,
	}, nil
}
