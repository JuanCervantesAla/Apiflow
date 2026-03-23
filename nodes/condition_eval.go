package nodes

import (
	"fmt"
	"strconv"
	"strings"
)

func evaluateCondition(left interface{}, operator string, right interface{}) (bool, error) {

	switch operator {

	case "==":
		if lNum, lok := toFloat(left); lok {
			if rNum, rok := toFloat(right); rok {
				return lNum == rNum, nil
			}
		}

		if lBool, lok := toBool(left); lok {
			if rBool, rok := toBool(right); rok {
				return lBool == rBool, nil
			}
		}

		return strings.EqualFold(strings.TrimSpace(fmt.Sprint(left)), strings.TrimSpace(fmt.Sprint(right))), nil

	case "!=":
		if lNum, lok := toFloat(left); lok {
			if rNum, rok := toFloat(right); rok {
				return lNum != rNum, nil
			}
		}

		if lBool, lok := toBool(left); lok {
			if rBool, rok := toBool(right); rok {
				return lBool != rBool, nil
			}
		}

		return !strings.EqualFold(strings.TrimSpace(fmt.Sprint(left)), strings.TrimSpace(fmt.Sprint(right))), nil

	case ">":
		l, lok := toFloat(left)
		r, rok := toFloat(right)
		if !lok || !rok {
			return false, fmt.Errorf("operator > requires numbers")
		}
		return l > r, nil

	case "<":
		l, lok := toFloat(left)
		r, rok := toFloat(right)
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

func toBool(v interface{}) (bool, bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(strings.ToLower(t)))
		if err != nil {
			return false, false
		}
		return parsed, true
	default:
		return false, false
	}
}
