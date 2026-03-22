package rules

import (
	"capyflow/api/models"
	"errors"
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
	"groq": true,
	// Integration
	"email":    true,
	"telegram": true,
	// Custom
	"custom": true,
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

	for i := range flow.Nodes {
		node := &flow.Nodes[i]

		if !AllowedNodeTypes[node.Type] {
			return errors.New("node type not allowed: " + node.Type)
		}

		if node.Category == string(models.CategoryTrigger) {
			triggerCount++
		}

		nodeMap[node.ID] = node
	}

	if triggerCount != 1 {
		return errors.New("flow must have exactly one trigger")
	}

	for _, edge := range flow.Edges {
		source, ok1 := nodeMap[edge.Source]
		target, ok2 := nodeMap[edge.Target]

		if !ok1 || !ok2 {
			return errors.New("edge points to non-existent nodes")
		}

		// Triggers CANNOT receive input connections
		if target.Category == string(models.CategoryTrigger) {
			println("ERROR: You cannot connect to a trigger node")
			println("  Tried to connect:", source.Label, "→", target.Label)
			println("  Triggers must be at the START of the flow, they cannot receive data")
			return errors.New("triggers cannot receive input connections. Connect from the trigger to other nodes")
		}

		allowed := AllowedCategoryTransitions[source.Category]
		valid := false
		for _, cat := range allowed {
			if cat == target.Category {
				valid = true
				break
			}
		}

		if !valid {
			// Debug: print which connection failed
			println("DEBUG: Connection rejected")
			println("  Source Node:", source.Label, "Type:", source.Type, "Category:", source.Category)
			println("  Target Node:", target.Label, "Type:", target.Type, "Category:", target.Category)
			println("  Allowed categories from source:", allowed)
			return errors.New("connection not allowed between nodes")
		}
	}

	return nil
}
