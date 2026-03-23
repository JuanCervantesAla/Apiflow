package nodes

import (
	"fmt"
	"sort"
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

	// Keep interpolation deterministic across executions.
	nodeIDs := make([]string, 0, len(prev))
	for nodeID := range prev {
		nodeIDs = append(nodeIDs, nodeID)
	}
	sort.Strings(nodeIDs)

	// First, support explicit node references like {{node-1.output.field}}
	for _, nodeID := range nodeIDs {
		output := prev[nodeID]
		keys := make([]string, 0, len(output))
		for key := range output {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		// Replace {{nodeID.output.field}}
		for _, key := range keys {
			val := output[key]
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
	// key in any previous node output. We only replace them when the key is
	// unique to avoid non-deterministic collisions (e.g. multiple "response").
	type keyMatch struct {
		val   interface{}
		count int
	}
	keyMatches := map[string]keyMatch{}
	for _, nodeID := range nodeIDs {
		for key, val := range prev[nodeID] {
			current := keyMatches[key]
			if current.count == 0 {
				keyMatches[key] = keyMatch{val: val, count: 1}
			} else {
				current.count++
				keyMatches[key] = current
			}
		}
	}

	for key, match := range keyMatches {
		if match.count != 1 {
			continue
		}
		placeholder := fmt.Sprintf("{{%s}}", key)
		if strings.Contains(result, placeholder) {
			result = strings.ReplaceAll(result, placeholder, fmt.Sprint(match.val))
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
