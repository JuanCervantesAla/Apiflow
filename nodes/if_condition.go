package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"fmt"
)

type IfConditionNode struct{}

type Condition struct {
	Left     string      `json:"left"`
	Operator string      `json:"operator"`
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

	var leftValue interface{}
	found := false
	for _, output := range prev {
		if val, ok := output[cond.Left]; ok {
			leftValue = val
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("Condition key not found: %s", cond.Left)
	}

	result, err := evaluateCondition(leftValue, cond.Operator, cond.Right)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"condition": result,
	}, nil
}
