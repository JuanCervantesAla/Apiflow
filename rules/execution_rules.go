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
	string(models.CategoryTrigger): {string(models.CategoryData), string(models.CategoryLogic), string(models.CategoryIO)},
	string(models.CategoryData):    {string(models.CategoryData), string(models.CategoryLogic), string(models.CategoryIO)},
	string(models.CategoryLogic):   {string(models.CategoryData), string(models.CategoryLogic), string(models.CategoryIO)},
	string(models.CategoryIO):      {string(models.CategoryData), string(models.CategoryLogic), string(models.CategoryIO)},
	"":                             {string(models.CategoryData), string(models.CategoryLogic), string(models.CategoryIO), ""}, // Para custom/sin categoría
}

var AllowedNodeTypes = map[string]bool{
	"manual-trigger":  true,
	"webhook-trigger": true,
	"set-data":        true,
	"transform-data":  true,
	"json-parser":     true,
	"if-condition":    true,
	"http-request":    true,
	"custom":          true,
}

func ValidateFlow(flow *models.Flow) error {
	if len(flow.Nodes) == 0 {
		return errors.New("el flujo no contiene nodos")
	}

	if len(flow.Nodes) > MaxNodes {
		return errors.New("excede el máximo de nodos permitidos")
	}

	triggerCount := 0
	nodeMap := map[string]*models.Node{}

	for i := range flow.Nodes {
		node := &flow.Nodes[i]

		if !AllowedNodeTypes[node.Type] {
			return errors.New("tipo de nodo no permitido: " + node.Type)
		}

		if node.Category == string(models.CategoryTrigger) {
			triggerCount++
		}

		nodeMap[node.ID] = node
	}

	if triggerCount != 1 {
		return errors.New("el flujo debe tener exactamente un trigger")
	}

	for _, edge := range flow.Edges {
		source, ok1 := nodeMap[edge.Source]
		target, ok2 := nodeMap[edge.Target]

		if !ok1 || !ok2 {
			return errors.New("edge apunta a nodos inexistentes")
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
			return errors.New("conexión no permitida entre nodos")
		}
	}

	return nil
}
