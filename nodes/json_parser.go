package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"fmt"
)

type JsonParserNode struct{}

func (n *JsonParserNode) Execute(
	node *models.Node,
	prev map[string]map[string]interface{},
) (map[string]interface{}, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, fmt.Errorf("json-parser: invalid parameters")
	}

	var raw interface{}
	found := false

	if jsonParam, ok := params["json"].(string); ok && jsonParam != "" {
		raw = interpolateString(jsonParam, prev)
		found = true
	}

	if !found {
		if jsonParam, ok := params["jsonString"].(string); ok && jsonParam != "" {
			raw = interpolateString(jsonParam, prev)
			found = true
		}
	}

	if !found {
		for _, output := range prev {
			if v, ok := output["json"]; ok {
				raw = v
				found = true
				break
			}
		}
	}

	if !found {
		return nil, fmt.Errorf("json-parser: input 'json' not found")
	}

	jsonStr, ok := raw.(string)
	if !ok {
		return nil, fmt.Errorf("json-parser: input is not a string")
	}

	var parsed interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return nil, fmt.Errorf("json-parser: invalid JSON")
	}

	return map[string]interface{}{
		"parsed": parsed,
	}, nil
}
