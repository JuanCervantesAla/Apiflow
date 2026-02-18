package nodes

import (
	"fmt"
	"strconv"
	"strings"
)

func toFloat(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case int:
		return float64(t), true
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(t, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func interpolateString(str string, prev map[string]map[string]interface{}) string {
	result := str
	for _, output := range prev {
		for key, val := range output {
			placeholder := fmt.Sprintf("{{%s}}", key)
			result = strings.ReplaceAll(result, placeholder, fmt.Sprint(val))
		}
	}
	return result
}

func interpolateMap(m map[string]interface{}, prev map[string]map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		switch val := v.(type) {
		case string:
			result[k] = interpolateString(val, prev)
		case map[string]interface{}:
			result[k] = interpolateMap(val, prev)
		default:
			result[k] = v
		}
	}
	return result
}
