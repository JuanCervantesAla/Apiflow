package services

import (
	"capyflow/api/models"
	"capyflow/api/nodes"
	"capyflow/api/validators"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func (es *ExecutorService) processNode(
	node *models.Node,
	prev map[string]map[string]interface{},
	userID string,
	executionOrder []string,
) (map[string]interface{}, error) {
	normalizeLegacyNodeAliases(node, executionOrder)

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

func normalizeLegacyNodeAliases(node *models.Node, executionOrder []string) {
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

	normalized := replaceLegacyAliasesInAny(parsed, aliasMap)
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

func replaceLegacyAliasesInAny(value interface{}, aliasMap map[string]string) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		updated := make(map[string]interface{}, len(v))
		for k, nested := range v {
			updated[k] = replaceLegacyAliasesInAny(nested, aliasMap)
		}
		return updated
	case []interface{}:
		updated := make([]interface{}, len(v))
		for i, nested := range v {
			updated[i] = replaceLegacyAliasesInAny(nested, aliasMap)
		}
		return updated
	case string:
		result := v
		for alias, realID := range aliasMap {
			result = strings.ReplaceAll(result, "{{"+alias+".", "{{"+realID+".")
			result = strings.ReplaceAll(result, "{{"+alias+"}}", "{{"+realID+"}}")
		}
		return result
	default:
		return value
	}
}
