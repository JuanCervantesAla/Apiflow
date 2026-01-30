package nodes

import "fmt"

func evaluateCondition(left interface{}, operator string, right interface{}) (bool, error) {

	switch operator {

	case "==":
		return left == right, nil

	case "!=":
		return left != right, nil

	case ">":
		l, lok := left.(float64)
		r, rok := right.(float64)
		if !lok || !rok {
			return false, fmt.Errorf("operator > requires numbers")
		}
		return l > r, nil

	case "<":
		l, lok := left.(float64)
		r, rok := right.(float64)
		if !lok || !rok {
			return false, fmt.Errorf("operator < requires numbers")
		}
		return l < r, nil

	default:
		return false, fmt.Errorf("unsupported operator: %s", operator)
	}
}
