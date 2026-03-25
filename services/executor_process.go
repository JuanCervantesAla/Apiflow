package services

import (
	"capyflow/api/models"
	"capyflow/api/nodes"
	"capyflow/api/validators"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var placeholderNodeRefPattern = regexp.MustCompile(`\{\{([a-zA-Z0-9-]+)\.output(?:\.([a-zA-Z0-9_.-]+))?\}\}`)

func (es *ExecutorService) processNode(
	node *models.Node,
	prev map[string]map[string]interface{},
	userID string,
	executionOrder []string,
) (map[string]interface{}, error) {
	normalizeLegacyNodeAliases(node, prev, executionOrder)

	if err := validators.ApplyDefaults(node); err != nil {
		return nil, fmt.Errorf("failed to apply node defaults: %w", err)
	}

	if err := es.hydrateNodeConnection(node, userID); err != nil {
		return nil, err
	}

	handler, ok := nodes.Registry[node.Type]
	if !ok {
		return nil, fmt.Errorf("Node not implemented: %s", node.Type)
	}

	return handler.Execute(node, prev)
}

func normalizeLegacyNodeAliases(node *models.Node, prev map[string]map[string]interface{}, executionOrder []string) {
	if node == nil || strings.TrimSpace(node.Parameters) == "" || len(executionOrder) == 0 {
		return
	}

	var parsed interface{}
	if err := json.Unmarshal([]byte(node.Parameters), &parsed); err != nil {
		// Keep original payload; validators/executor will report malformed JSON later.
		return
	}

	aliasMap := buildLegacyAliasMap(executionOrder)
	if len(aliasMap) == 0 {
		return
	}

	normalized := replaceLegacyAliasesInAny(parsed, aliasMap, prev, executionOrder)
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return
	}

	node.Parameters = string(encoded)
}

func buildLegacyAliasMap(executionOrder []string) map[string]string {
	aliasMap := make(map[string]string, len(executionOrder))
	for i, nodeID := range executionOrder {
		aliasMap["node-"+strconv.Itoa(i+1)] = nodeID
	}
	return aliasMap
}

func replaceLegacyAliasesInAny(
	value interface{},
	aliasMap map[string]string,
	prev map[string]map[string]interface{},
	executionOrder []string,
) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		updated := make(map[string]interface{}, len(v))
		for k, nested := range v {
			updated[k] = replaceLegacyAliasesInAny(nested, aliasMap, prev, executionOrder)
		}
		return updated
	case []interface{}:
		updated := make([]interface{}, len(v))
		for i, nested := range v {
			updated[i] = replaceLegacyAliasesInAny(nested, aliasMap, prev, executionOrder)
		}
		return updated
	case string:
		result := v
		for alias, realID := range aliasMap {
			result = strings.ReplaceAll(result, "{{"+alias+".", "{{"+realID+".")
			result = strings.ReplaceAll(result, "{{"+alias+"}}", "{{"+realID+"}}")
		}
		return remapUnknownNodeReferences(result, prev, executionOrder)
	default:
		return value
	}
}

func remapUnknownNodeReferences(
	input string,
	prev map[string]map[string]interface{},
	executionOrder []string,
) string {
	if strings.TrimSpace(input) == "" || len(prev) == 0 {
		return input
	}

	matches := placeholderNodeRefPattern.FindAllStringSubmatch(input, -1)
	if len(matches) == 0 {
		return input
	}

	orderedCandidates := orderedExistingNodeIDs(executionOrder, prev)
	if len(orderedCandidates) == 0 {
		return input
	}

	result := input
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		full := match[0]
		refID := match[1]
		path := match[2]

		if _, exists := prev[refID]; exists {
			continue
		}
		if !strings.HasPrefix(refID, "node-") && !looksLikeUUID(refID) {
			continue
		}

		targetID := findBestReferenceTarget(path, orderedCandidates, prev)
		if targetID == "" {
			continue
		}

		replacement := "{{" + targetID + ".output"
		if path != "" {
			replacement += "." + path
		}
		replacement += "}}"

		result = strings.ReplaceAll(result, full, replacement)
	}

	return result
}

func orderedExistingNodeIDs(executionOrder []string, prev map[string]map[string]interface{}) []string {
	ordered := make([]string, 0, len(executionOrder))
	for i := len(executionOrder) - 1; i >= 0; i-- {
		nodeID := executionOrder[i]
		if _, ok := prev[nodeID]; ok {
			ordered = append(ordered, nodeID)
		}
	}
	return ordered
}

func findBestReferenceTarget(
	path string,
	orderedCandidates []string,
	prev map[string]map[string]interface{},
) string {
	if path == "" {
		if len(orderedCandidates) > 0 {
			return orderedCandidates[0]
		}
		return ""
	}

	for _, nodeID := range orderedCandidates {
		if hasOutputPath(prev[nodeID], path) {
			return nodeID
		}
	}

	return ""
}

func hasOutputPath(output map[string]interface{}, path string) bool {
	if len(output) == 0 || strings.TrimSpace(path) == "" {
		return false
	}

	if resolvePathInMap(output, path) {
		return true
	}

	// Compatibility: if placeholder asks for body.x, try resolving x directly
	// because webhook trigger outputs may already be flattened.
	if strings.HasPrefix(path, "body.") {
		trimmed := strings.TrimPrefix(path, "body.")
		if resolvePathInMap(output, trimmed) {
			return true
		}
	}

	return false
}

func resolvePathInMap(data map[string]interface{}, path string) bool {
	parts := strings.Split(path, ".")
	var current interface{} = data

	for _, rawPart := range parts {
		part := strings.TrimSpace(rawPart)
		if part == "" {
			return false
		}

		asMap, ok := current.(map[string]interface{})
		if !ok {
			return false
		}

		next, exists := asMap[part]
		if !exists {
			return false
		}
		current = next
	}

	return true
}

func looksLikeUUID(value string) bool {
	parts := strings.Split(value, "-")
	if len(parts) != 5 {
		return false
	}

	segmentLengths := []int{8, 4, 4, 4, 12}
	for i, part := range parts {
		if len(part) != segmentLengths[i] {
			return false
		}
		for _, r := range part {
			if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
				return false
			}
		}
	}

	return true
}
