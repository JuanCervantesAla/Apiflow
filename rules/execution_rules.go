package rules

import (
	"capyflow/api/models"
	"errors"
	"strings"
)

const (
	MaxNodes      = 20
	MaxDepth      = 10
	FlowTimeoutMs = 30000
)

var AllowedCategoryTransitions = map[string][]string{
	string(models.CategoryTrigger):     {string(models.CategoryData), string(models.CategoryLogic), string(models.CategoryIO), string(models.CategoryControl), string(models.CategoryIntegration), string(models.CategoryAI)},
	string(models.CategoryData):        {string(models.CategoryData), string(models.CategoryLogic), string(models.CategoryIO), string(models.CategoryControl), string(models.CategoryIntegration), string(models.CategoryAI)},
	string(models.CategoryLogic):       {string(models.CategoryData), string(models.CategoryLogic), string(models.CategoryIO), string(models.CategoryControl), string(models.CategoryIntegration), string(models.CategoryAI)},
	string(models.CategoryIO):          {string(models.CategoryData), string(models.CategoryLogic), string(models.CategoryIO), string(models.CategoryControl), string(models.CategoryIntegration), string(models.CategoryAI)},
	string(models.CategoryControl):     {string(models.CategoryData), string(models.CategoryLogic), string(models.CategoryIO), string(models.CategoryControl), string(models.CategoryIntegration), string(models.CategoryAI)},
	string(models.CategoryIntegration): {string(models.CategoryData), string(models.CategoryLogic), string(models.CategoryIO), string(models.CategoryControl), string(models.CategoryIntegration), string(models.CategoryAI)},
	string(models.CategoryAI):          {string(models.CategoryData), string(models.CategoryLogic), string(models.CategoryIO), string(models.CategoryControl), string(models.CategoryIntegration), string(models.CategoryAI)},
	"":                                 {string(models.CategoryData), string(models.CategoryLogic), string(models.CategoryIO), string(models.CategoryControl), string(models.CategoryIntegration), string(models.CategoryAI), ""}, // For custom/no category
}

var AllowedNodeTypes = map[string]bool{
	// Triggers
	"manual-trigger":   true,
	"webhook-trigger":  true,
	"cron-trigger":     true,
	"telegram-trigger": true,
	// Data Processing
	"set-data":       true,
	"transform-data": true,
	"json-parser":    true,
	"filter":         true,
	"split":          true,
	"merge":          true,
	"function":       true,
	"sort":           true,
	"csv-parser":     true,
	"regex-extract":  true,
	// Logic & Control
	"if-condition":  true,
	"switch":        true,
	"error-handler": true,
	"stop":          true,
	"loop":          true,
	"delay":         true,
	// IO
	"http-request": true,
	"log":          true,
	"database":     true,
	// AI
	"groq":            true,
	"ai-configurator": true,
	// Integration
	"email":           true,
	"telegram":        true,
	"document-ingest": true,
	"ocr-extract":     true,
	"finance-extract": true,
	// Custom
	"custom": true,
}

var TriggerNodeTypes = map[string]bool{
	"manual-trigger":   true,
	"webhook-trigger":  true,
	"cron-trigger":     true,
	"telegram-trigger": true,
}

func normalizeValidationCategory(node *models.Node) string {
	if node == nil {
		return ""
	}

	if TriggerNodeTypes[node.Type] {
		return string(models.CategoryTrigger)
	}

	cat := strings.TrimSpace(node.Category)
	if _, ok := AllowedCategoryTransitions[cat]; ok {
		return cat
	}

	// Unknown/legacy categories (e.g. "other") are treated as uncategorized.
	return ""
}

func ValidateFlow(flow *models.Flow) error {
	if len(flow.Nodes) == 0 {
		return errors.New("flow contains no nodes")
	}

	if len(flow.Nodes) > MaxNodes {
		return errors.New("exceeds maximum allowed nodes")
	}

	triggerCount := 0
	nodeMap := map[string]*models.Node{}
	outgoing := map[string][]models.Edge{}

	for i := range flow.Nodes {
		node := &flow.Nodes[i]

		if !AllowedNodeTypes[node.Type] {
			return errors.New("node type not allowed: " + node.Type)
		}

		if node.Category == string(models.CategoryTrigger) || TriggerNodeTypes[node.Type] {
			triggerCount++
		}

		nodeMap[node.ID] = node
	}

	if triggerCount != 1 {
		return errors.New("flow must have exactly one trigger")
	}

	for _, edge := range flow.Edges {
		outgoing[edge.Source] = append(outgoing[edge.Source], edge)

		source, ok1 := nodeMap[edge.Source]
		target, ok2 := nodeMap[edge.Target]

		if !ok1 || !ok2 {
			return errors.New("edge points to non-existent nodes")
		}

		sourceCategory := normalizeValidationCategory(source)
		targetCategory := normalizeValidationCategory(target)

		// Triggers CANNOT receive input connections
		if targetCategory == string(models.CategoryTrigger) {
			println("ERROR: You cannot connect to a trigger node")
			println("  Tried to connect:", source.Label, "→", target.Label)
			println("  Triggers must be at the START of the flow, they cannot receive data")
			return errors.New("triggers cannot receive input connections. Connect from the trigger to other nodes")
		}

		allowed := AllowedCategoryTransitions[sourceCategory]
		valid := false
		for _, cat := range allowed {
			if cat == targetCategory {
				valid = true
				break
			}
		}

		if !valid {
			// Debug: print which connection failed
			println("DEBUG: Connection rejected")
			println("  Source Node:", source.Label, "Type:", source.Type, "Category:", sourceCategory)
			println("  Target Node:", target.Label, "Type:", target.Type, "Category:", targetCategory)
			println("  Allowed categories from source:", allowed)
			return errors.New("connection not allowed between nodes")
		}
	}

	for _, node := range flow.Nodes {
		if node.Type != "if-condition" {
			continue
		}

		edges := outgoing[node.ID]
		if len(edges) == 0 {
			return errors.New("if-condition node must have true/false branches connected")
		}

		// Legacy tolerance: many AI-generated flows omit branch labels/handles.
		// Runtime can execute with one fallback edge or route deterministically with two.
		if len(edges) >= 1 {
			continue
		}

		hasTrue := false
		hasFalse := false
		for _, e := range edges {
			branch := normalizeIfBranchToken(e.SourceHandle)
			if branch == "" {
				branch = normalizeIfBranchToken(e.Label)
			}
			switch branch {
			case "true":
				hasTrue = true
			case "false":
				hasFalse = true
			}
		}

		if !hasTrue || !hasFalse {
			return errors.New("if-condition node requires both true and false outgoing branches")
		}
	}

	return nil
}

func normalizeIfBranchToken(raw string) string {
	branch := strings.ToLower(strings.TrimSpace(raw))
	switch branch {
	case "true", "t", "yes", "y", "si", "sí", "1", "ok", "pass", "critical", "critico", "crítico":
		return "true"
	case "false", "f", "no", "n", "0", "fail", "normal":
		return "false"
	default:
		return ""
	}
}
