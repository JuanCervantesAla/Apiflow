package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"fmt"
	"strings"
)

type LoopNode struct{}

type LoopParams struct {
	ArraySource   string `json:"arraySource"`   // Path to the array in context: "body.users"
	Operation     string `json:"operation"`     // "map", "filter", "forEach"
	MapExpression string `json:"mapExpression"` // For map: expression to apply "{{item.name}}"
	FilterExpr    string `json:"filterExpr"`    // For filter: condition "{{item.age}} > 18"
	ItemVariable  string `json:"itemVariable"`  // Variable name for each item (default: "item")
	IndexVariable string `json:"indexVariable"` // Variable name for the index (default: "index")
}

func (n *LoopNode) Execute(node *models.Node, context map[string]map[string]interface{}) (map[string]interface{}, error) {
	var params LoopParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, fmt.Errorf("error decoding parameters: %v", err)
	}

	// Defaults
	if params.ItemVariable == "" {
		params.ItemVariable = "item"
	}
	if params.IndexVariable == "" {
		params.IndexVariable = "index"
	}
	if params.Operation == "" {
		params.Operation = "forEach"
	}

	// Merge all previous outputs into a flat map for getNestedValue
	prev := make(map[string]interface{})
	for _, outputs := range context {
		for k, v := range outputs {
			prev[k] = v
		}
	}

	// Get the array from context
	arrayValue := getNestedValue(prev, params.ArraySource)
	fmt.Printf("[DEBUG] Loop Node - arraySource: %s, arrayValue: %T, %v\n", params.ArraySource, arrayValue, arrayValue)

	if arrayValue == nil {
		fmt.Printf("[DEBUG] Loop Node - arrayValue is nil, returning empty array\n")
		return map[string]interface{}{
			"items": []interface{}{},
			"count": 0,
		}, nil
	}

	// Convert to slice
	arraySlice, ok := arrayValue.([]interface{})
	if !ok {
		fmt.Printf("[DEBUG] Loop Node - arrayValue is not []interface{}, type: %T\n", arrayValue)
		return nil, fmt.Errorf("'%s' is not a valid array", params.ArraySource)
	}

	fmt.Printf("[DEBUG] Loop Node - arraySlice length: %d, operation: %s\n", len(arraySlice), params.Operation)

	switch params.Operation {
	case "map":
		return n.executeMap(arraySlice, params, context)
	case "filter":
		return n.executeFilter(arraySlice, params, context)
	case "forEach":
		return n.executeForEach(arraySlice, params)
	default:
		return nil, fmt.Errorf("operation not supported: %s", params.Operation)
	}
}

// executeMap applies a transformation to each item
func (n *LoopNode) executeMap(array []interface{}, params LoopParams, context map[string]map[string]interface{}) (map[string]interface{}, error) {
	if params.MapExpression == "" {
		return nil, fmt.Errorf("mapExpression is required for 'map' operation")
	}

	results := make([]interface{}, len(array))

	for i, item := range array {
		// Create temporary context for this item
		// Flatten the context: copy all existing variables
		itemContext := make(map[string]map[string]interface{})
		for k, v := range context {
			itemContext[k] = v
		}

		// Add loop variables in a temporary "loop" node
		// This allows accessing {{item.name}} or {{index}}
		loopVars := map[string]interface{}{
			params.IndexVariable: i,
		}

		// If the item is a map, add its fields directly with the item prefix
		if itemMap, ok := item.(map[string]interface{}); ok {
			for k, v := range itemMap {
				loopVars[params.ItemVariable+"."+k] = v
			}
			loopVars[params.ItemVariable] = item // Also keep the complete item
		} else {
			loopVars[params.ItemVariable] = item
		}

		itemContext["loop"] = loopVars

		// Interpolate the expression
		result := interpolateString(params.MapExpression, itemContext)
		results[i] = result
		if i < 3 {
			fmt.Printf("[DEBUG] Loop Map - item %d: %v -> result: %v\n", i, item, result)
		}
	}

	fmt.Printf("[DEBUG] Loop Map - final results length: %d, first 3: %v\n", len(results), results[:min(3, len(results))])
	return map[string]interface{}{
		"items": results,
		"count": len(results),
	}, nil
}

// executeFilter filters items that meet a condition
func (n *LoopNode) executeFilter(array []interface{}, params LoopParams, context map[string]map[string]interface{}) (map[string]interface{}, error) {
	if params.FilterExpr == "" {
		return nil, fmt.Errorf("filterExpr is required for 'filter' operation")
	}

	results := []interface{}{}

	for i, item := range array {
		// Create temporary context for this item
		itemContext := make(map[string]map[string]interface{})
		for k, v := range context {
			itemContext[k] = v
		}

		// Add loop variables
		loopVars := map[string]interface{}{
			params.IndexVariable: i,
		}

		// If the item is a map, add its fields with the item prefix
		if itemMap, ok := item.(map[string]interface{}); ok {
			for k, v := range itemMap {
				loopVars[params.ItemVariable+"."+k] = v
			}
			loopVars[params.ItemVariable] = item
		} else {
			loopVars[params.ItemVariable] = item
		}

		itemContext["loop"] = loopVars

		// Evaluate condition
		condition := interpolateString(params.FilterExpr, itemContext)
		// If the evaluated condition is "true" or contains the item, include it
		if condition == "true" || strings.Contains(strings.ToLower(condition), "true") {
			results = append(results, item)
		}
	}

	return map[string]interface{}{
		"items": results,
		"count": len(results),
	}, nil
}

// executeForEach simply passes the items to the next node
// Useful for combining with other nodes that process arrays
func (n *LoopNode) executeForEach(array []interface{}, params LoopParams) (map[string]interface{}, error) {
	// Create array of items with metadata
	items := make([]interface{}, len(array))
	for i, item := range array {
		items[i] = map[string]interface{}{
			params.ItemVariable:  item,
			params.IndexVariable: i,
		}
	}

	return map[string]interface{}{
		"items": items,
		"count": len(array),
	}, nil
}
