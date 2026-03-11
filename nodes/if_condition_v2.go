package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type IfConditionNodeV2 struct{}

type IfParams struct {
	Expression string `json:"expression"`
}

func (n *IfConditionNodeV2) Execute(
	node *models.Node,
	prev map[string]map[string]interface{},
) (map[string]interface{}, error) {

	// 1. Parse params
	var params IfParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, errors.New("invalid if-condition parameters")
	}

	if params.Expression == "" {
		return nil, errors.New("expression is required")
	}

	// 2. Parse expression
	leftPath, operator, rightRaw, err := parseExpression(params.Expression)
	if err != nil {
		return nil, err
	}

	// 3. Resolve left value from context
	leftValue, err := resolvePath(prev, leftPath)
	if err != nil {
		return nil, err
	}

	// 4. Cast right value
	rightValue := parseLiteral(rightRaw)

	// 5. Evaluate
	result, err := compare(leftValue, operator, rightValue)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"condition": result,
	}, nil
}

func parseExpression(expr string) (left string, operator string, right string, err error) {
	operators := []string{"==", "!=", ">=", "<=", ">", "<"}

	for _, op := range operators {
		if strings.Contains(expr, op) {
			parts := strings.SplitN(expr, op, 2)
			if len(parts) != 2 {
				return "", "", "", fmt.Errorf("invalid expression: %s", expr)
			}
			return strings.TrimSpace(parts[0]), op, strings.TrimSpace(parts[1]), nil
		}
	}

	return "", "", "", fmt.Errorf("unsupported operator in expression")
}

func resolvePath(
	ctx map[string]map[string]interface{},
	path string,
) (interface{}, error) {

	if !strings.HasPrefix(path, "$.") {
		return nil, fmt.Errorf("invalid path: %s", path)
	}

	parts := strings.Split(path[2:], ".")

	var current interface{} = ctx["__webhook__"]
	for _, p := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("path not found: %s", path)
		}
		current, ok = m[p]
		if !ok {
			return nil, fmt.Errorf("path not found: %s", path)
		}
	}

	return current, nil
}

func compare(left interface{}, op string, right interface{}) (bool, error) {

	// Number comparison
	lf, lok := toFloat(left)
	rf, rok := toFloat(right)

	if lok && rok {
		switch op {
		case "==":
			return lf == rf, nil
		case "!=":
			return lf != rf, nil
		case ">":
			return lf > rf, nil
		case "<":
			return lf < rf, nil
		case ">=":
			return lf >= rf, nil
		case "<=":
			return lf <= rf, nil
		}
	}

	// String comparison
	ls, lok := left.(string)
	rs, rok := right.(string)

	if lok && rok {
		switch op {
		case "==":
			return ls == rs, nil
		case "!=":
			return ls != rs, nil
		}
	}

	return false, fmt.Errorf("cannot compare values")
}

func parseLiteral(v string) interface{} {
	v = strings.Trim(v, `"`)

	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}

	if v == "true" {
		return true
	}
	if v == "false" {
		return false
	}

	if v == "null" {
		return nil
	}

	return v
}
