package nodes

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var explicitNodePlaceholderRe = regexp.MustCompile(`\{\{([a-zA-Z0-9-]+)\.output(?:\.([a-zA-Z0-9_.-]+))?\}\}`)

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

	// Resolve explicit placeholders with nested paths (e.g. {{id.output.body.title}})
	// and compatibility fallback for flattened webhook fields (body.title -> title).
	result = explicitNodePlaceholderRe.ReplaceAllStringFunc(result, func(placeholder string) string {
		matches := explicitNodePlaceholderRe.FindStringSubmatch(placeholder)
		if len(matches) < 3 {
			return placeholder
		}

		nodeID := matches[1]
		path := matches[2]
		output, ok := prev[nodeID]
		if !ok {
			return placeholder
		}

		if path == "" {
			return fmt.Sprint(output)
		}

		if value, found := resolveOutputPath(output, path); found {
			return fmt.Sprint(value)
		}

		return placeholder
	})

	// Backwards compatibility: simple placeholders like {{field}} that match any
	// key in any previous node output. We only replace them when the key is
	// unique to avoid non-deterministic collisions (e.g. multiple "response").
	// If a key appears in multiple nodes but all values are equal, we still
	// replace it because the result remains deterministic.
	type keyMatch struct {
		val       interface{}
		count     int
		allEqual  bool
		firstRepr string
	}
	keyMatches := map[string]keyMatch{}
	for _, nodeID := range nodeIDs {
		for key, val := range prev[nodeID] {
			current := keyMatches[key]
			if current.count == 0 {
				repr := fmt.Sprint(val)
				keyMatches[key] = keyMatch{val: val, count: 1, allEqual: true, firstRepr: repr}
			} else {
				current.count++
				if current.allEqual && current.firstRepr != fmt.Sprint(val) {
					current.allEqual = false
				}
				keyMatches[key] = current
			}
		}
	}

	for key, match := range keyMatches {
		if match.count != 1 && !match.allEqual {
			continue
		}
		placeholder := fmt.Sprintf("{{%s}}", key)
		if strings.Contains(result, placeholder) {
			result = strings.ReplaceAll(result, placeholder, fmt.Sprint(match.val))
		}
	}

	return result
}

func resolveOutputPath(output map[string]interface{}, path string) (interface{}, bool) {
	if len(output) == 0 || strings.TrimSpace(path) == "" {
		return nil, false
	}

	if value, ok := resolveDotPath(output, path); ok {
		return value, true
	}

	if strings.HasPrefix(path, "body.") {
		trimmed := strings.TrimPrefix(path, "body.")
		if value, ok := resolveDotPath(output, trimmed); ok {
			return value, true
		}
	}

	return nil, false
}

func resolveDotPath(data map[string]interface{}, path string) (interface{}, bool) {
	parts := strings.Split(path, ".")
	var current interface{} = data

	for _, raw := range parts {
		part := strings.TrimSpace(raw)
		if part == "" {
			return nil, false
		}

		asMap, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}

		next, exists := asMap[part]
		if !exists {
			return nil, false
		}
		current = next
	}

	return current, true
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
