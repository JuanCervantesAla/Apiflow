package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"errors"
)

type TransformDataNode struct{}

type TransformParams struct {
	Operation string      `json:"operation"`
	Field     string      `json:"field"`
	Multiply  float64     `json:"multiply,omitempty"`
	Equals    interface{} `json:"equals,omitempty"`
}

func (n *TransformDataNode) Execute(
	node *models.Node,
	prev map[string]map[string]interface{},
) (map[string]interface{}, error) {

	var params TransformParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, errors.New("invalid transform-data parameters")
	}

	input, ok := prev[node.ID]["data"]
	if !ok {
		for _, out := range prev {
			if v, exists := out["data"]; exists {
				input = v
				ok = true
				break
			}
		}
	}

	if !ok {
		return nil, errors.New("transform-data requires 'data' input")
	}

	items, ok := input.([]interface{})
	if !ok {
		return nil, errors.New("transform-data input must be an array")
	}

	switch params.Operation {

	case "map":
		return mapOperation(items, params)

	case "filter":
		return filterOperation(items, params)

	default:
		return nil, errors.New("unsupported transform operation")
	}
}

func mapOperation(
	items []interface{},
	params TransformParams,
) (map[string]interface{}, error) {

	result := []interface{}{}

	for _, item := range items {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		val, exists := obj[params.Field]
		if !exists {
			result = append(result, obj)
			continue
		}

		num, ok := toFloat(val)
		if !ok {
			result = append(result, obj)
			continue
		}

		obj[params.Field] = num * params.Multiply
		result = append(result, obj)
	}

	return map[string]interface{}{
		"result": result,
	}, nil
}

func filterOperation(
	items []interface{},
	params TransformParams,
) (map[string]interface{}, error) {

	result := []interface{}{}

	for _, item := range items {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		val, exists := obj[params.Field]
		if !exists {
			continue
		}

		if val == params.Equals {
			result = append(result, obj)
		}
	}

	return map[string]interface{}{
		"result": result,
	}, nil
}
