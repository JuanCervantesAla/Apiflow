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

	// First, support explicit node references like {{node-1.output.field}}
	for nodeID, output := range prev {
		// Replace {{nodeID.output.field}}
		for key, val := range output {
			placeholder := fmt.Sprintf("{{%s.output.%s}}", nodeID, key)
			if strings.Contains(result, placeholder) {
				result = strings.ReplaceAll(result, placeholder, fmt.Sprint(val))
			}
		}

		// Replace {{nodeID.output}} with the full output map
		basePlaceholder := fmt.Sprintf("{{%s.output}}", nodeID)
		if strings.Contains(result, basePlaceholder) {
			result = strings.ReplaceAll(result, basePlaceholder, fmt.Sprint(output))
		}
	}

	// Backwards compatibility: simple placeholders like {{field}} that match any
	// key in any previous node output
	for _, output := range prev {
		for key, val := range output {
			placeholder := fmt.Sprintf("{{%s}}", key)
			if strings.Contains(result, placeholder) {
				result = strings.ReplaceAll(result, placeholder, fmt.Sprint(val))
			}
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
