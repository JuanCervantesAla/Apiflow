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
	string(models.CategoryTrigger): {string(models.CategoryData), string(models.CategoryLogic), string(models.CategoryIO), string(models.CategoryControl)},
	string(models.CategoryData):    {string(models.CategoryData), string(models.CategoryLogic), string(models.CategoryIO), string(models.CategoryControl)},
	string(models.CategoryLogic):   {string(models.CategoryData), string(models.CategoryLogic), string(models.CategoryIO), string(models.CategoryControl)},
	string(models.CategoryIO):      {string(models.CategoryData), string(models.CategoryLogic), string(models.CategoryIO), string(models.CategoryControl)},
	string(models.CategoryControl): {string(models.CategoryData), string(models.CategoryLogic), string(models.CategoryIO), string(models.CategoryControl)},
	"":                             {string(models.CategoryData), string(models.CategoryLogic), string(models.CategoryIO), string(models.CategoryControl), ""}, // Para custom/sin categoría
}

var AllowedNodeTypes = map[string]bool{
	"manual-trigger":  true,
	"webhook-trigger": true,
	"set-data":        true,
	"transform-data":  true,
	"json-parser":     true,
	"if-condition":    true,
	"http-request":    true,
	"log":             true,
	"loop":            true,
	"delay":           true,
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

		// Los triggers NO pueden recibir conexiones de entrada
		if target.Category == string(models.CategoryTrigger) {
			println("ERROR: No puedes conectar hacia un nodo trigger")
			println("  Intentaste conectar:", source.Label, "→", target.Label)
			println("  Los triggers deben estar al INICIO del flujo, no pueden recibir datos")
			return errors.New("los triggers no pueden recibir conexiones de entrada. Conecta desde el trigger hacia otros nodos")
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
			// Debug: imprimir qué conexión falló
			println("DEBUG: Conexión rechazada")
			println("  Source Node:", source.Label, "Type:", source.Type, "Category:", source.Category)
			println("  Target Node:", target.Label, "Type:", target.Type, "Category:", target.Category)
			println("  Allowed categories from source:", allowed)
			return errors.New("conexión no permitida entre nodos")
		}
	}

	return nil
}
