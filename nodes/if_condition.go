package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"fmt"
	"strings"
)

type IfConditionNode struct{}

type Condition struct {
	Field    string      `json:"field"`
	Left     string      `json:"left"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
	Right    interface{} `json:"right"`
}

func (n *IfConditionNode) Execute(
	node *models.Node,
	prev map[string]map[string]interface{},
) (map[string]interface{}, error) {
	var cond Condition
	if err := json.Unmarshal([]byte(node.Parameters), &cond); err != nil {
		return nil, fmt.Errorf("Invalid condition parameters")
	}

	if cond.Field == "" && cond.Left != "" {
		cond.Field = cond.Left
	}
	if cond.Value == nil && cond.Right != nil {
		cond.Value = cond.Right
	}
	if cond.Operator == "" {
		cond.Operator = "=="
	}

	var leftValue interface{}
	found := false

	// Support explicit references like {{node-2.output.status}}
	if cond.Field != "" {
		if value, ok := resolveConditionField(cond.Field, prev); ok {
			leftValue = value
			found = true
		}
	}

	if !found {
		if cond.Operator == "exists" {
			return map[string]interface{}{
				"condition": false,
			}, nil
		}
		return nil, fmt.Errorf("Condition key not found: %s", cond.Field)
	}

	result, err := evaluateCondition(leftValue, cond.Operator, cond.Value)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"condition": result,
	}, nil
}

func resolveConditionField(field string, prev map[string]map[string]interface{}) (interface{}, bool) {
	ref := field
	if len(ref) >= 4 && ref[:2] == "{{" && ref[len(ref)-2:] == "}}" {
		ref = ref[2 : len(ref)-2]
	}

	// node-X.output.some.path
	parts := splitPath(ref)
	if len(parts) >= 3 && parts[1] == "output" {
		nodeID := parts[0]
		if output, ok := prev[nodeID]; ok {
			return getNestedValue(output, joinPath(parts[2:])), true
		}
	}

	// Fallback: search key in previous outputs
	for _, output := range prev {
		if val, ok := output[field]; ok {
			return val, true
		}
		if val := getNestedValue(output, field); val != nil {
			return val, true
		}
	}

	return nil, false
}

func splitPath(path string) []string {
	if path == "" {
		return []string{}
	}
	return strings.Split(path, ".")
}

func joinPath(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	res := parts[0]
	for i := 1; i < len(parts); i++ {
		res += "." + parts[i]
	}
	return res
}
