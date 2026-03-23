package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"fmt"
	"strings"
)

// Struct of the type of the node
type SetDataNode struct{}

// ACTION: Sets the node output by mapping the specific 'values' object from the provided parameters.
func (n *SetDataNode) Execute(
	node *models.Node, //Takes the node model meaning the class
	prev map[string]map[string]interface{}, //Creates a Map of string,object
) (map[string]interface{}, error) {
	var params map[string]interface{}                                        //Params
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil { //If params are note correct or is null
		return nil, fmt.Errorf("Invalid parameters in set-data") //Error
	}

	values, ok := params["values"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("Set-data Requieres 'values' object")
	}

	output := map[string]interface{}{}

	for k, v := range values { //Search the values in the range
		output[k] = resolveSetDataValue(v, prev)
	}

	return output, nil //Return the setted output
}

func resolveSetDataValue(v interface{}, prev map[string]map[string]interface{}) interface{} {
	switch val := v.(type) {
	case string:
		interpolated := interpolateString(val, prev)
		if strings.Contains(interpolated, "?") && strings.Contains(interpolated, ":") {
			if evaluated, ok := evaluateSimpleTernary(interpolated); ok {
				return evaluated
			}
		}
		return interpolated
	case map[string]interface{}:
		resolved := map[string]interface{}{}
		for k, nested := range val {
			resolved[k] = resolveSetDataValue(nested, prev)
		}
		return resolved
	case []interface{}:
		resolved := make([]interface{}, len(val))
		for i, nested := range val {
			resolved[i] = resolveSetDataValue(nested, prev)
		}
		return resolved
	default:
		return v
	}
}

// Supports simple expressions like:
// category == 'billing' ? 'finance' : category == 'technical' ? 'engineering' : 'customer-success'
func evaluateSimpleTernary(expr string) (string, bool) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return "", false
	}

	q := strings.Index(expr, "?")
	if q < 0 {
		return stripQuoted(expr), true
	}

	cond := strings.TrimSpace(expr[:q])
	rest := strings.TrimSpace(expr[q+1:])
	truePart, falsePart, ok := splitTernaryBranches(rest)
	if !ok {
		return "", false
	}

	if evalSimpleCondition(cond) {
		return evaluateSimpleTernary(truePart)
	}
	return evaluateSimpleTernary(falsePart)
}

func splitTernaryBranches(rest string) (string, string, bool) {
	depth := 0
	for i, r := range rest {
		switch r {
		case '?':
			depth++
		case ':':
			if depth == 0 {
				left := strings.TrimSpace(rest[:i])
				right := strings.TrimSpace(rest[i+1:])
				return left, right, true
			}
			depth--
		}
	}
	return "", "", false
}

func evalSimpleCondition(cond string) bool {
	cond = strings.TrimSpace(cond)
	if strings.Contains(cond, "==") {
		parts := strings.SplitN(cond, "==", 2)
		left := stripQuoted(strings.TrimSpace(parts[0]))
		right := stripQuoted(strings.TrimSpace(parts[1]))
		return left == right
	}
	if strings.Contains(cond, "!=") {
		parts := strings.SplitN(cond, "!=", 2)
		left := stripQuoted(strings.TrimSpace(parts[0]))
		right := stripQuoted(strings.TrimSpace(parts[1]))
		return left != right
	}
	return false
}

func stripQuoted(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
