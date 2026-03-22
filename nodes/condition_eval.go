package nodes

import (
	"fmt"
	"strings"
)

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

	case ">=":
		l, lok := toFloat(left)
		r, rok := toFloat(right)
		if !lok || !rok {
			return false, fmt.Errorf("operator >= requires numbers")
		}
		return l >= r, nil

	case "<=":
		l, lok := toFloat(left)
		r, rok := toFloat(right)
		if !lok || !rok {
			return false, fmt.Errorf("operator <= requires numbers")
		}
		return l <= r, nil

	case "contains":
		leftStr := strings.ToLower(fmt.Sprint(left))
		rightStr := strings.ToLower(fmt.Sprint(right))
		return strings.Contains(leftStr, rightStr), nil

	case "exists":
		return left != nil, nil

	default:
		return false, fmt.Errorf("unsupported operator: %s", operator)
	}
}
