package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"fmt"
	"strings"
)

type LoopNode struct{}

type LoopParams struct {
	ArraySource   string `json:"arraySource"`   // Path al array en el contexto: "body.users"
	Operation     string `json:"operation"`     // "map", "filter", "forEach"
	MapExpression string `json:"mapExpression"` // Para map: expresión a aplicar "{{item.name}}"
	FilterExpr    string `json:"filterExpr"`    // Para filter: condición "{{item.age}} > 18"
	ItemVariable  string `json:"itemVariable"`  // Nombre de la variable para cada item (default: "item")
	IndexVariable string `json:"indexVariable"` // Nombre de la variable para el índice (default: "index")
}

func (n *LoopNode) Execute(node *models.Node, context map[string]map[string]interface{}) (map[string]interface{}, error) {
	var params LoopParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, fmt.Errorf("error al decodificar parámetros: %v", err)
	}

	// Defaults
	if params.ItemVariable == "" {
		params.ItemVariable = "item"
	}
	if params.IndexVariable == "" {
		params.IndexVariable = "index"
	}
	if params.Operation == "" {
		params.Operation = "forEach"
	}

	// Merge all previous outputs into a flat map for getNestedValue
	prev := make(map[string]interface{})
	for _, outputs := range context {
		for k, v := range outputs {
			prev[k] = v
		}
	}

	// Obtener el array del contexto
	arrayValue := getNestedValue(prev, params.ArraySource)
	fmt.Printf("[DEBUG] Loop Node - arraySource: %s, arrayValue: %T, %v\n", params.ArraySource, arrayValue, arrayValue)

	if arrayValue == nil {
		fmt.Printf("[DEBUG] Loop Node - arrayValue is nil, returning empty array\n")
		return map[string]interface{}{
			"items": []interface{}{},
			"count": 0,
		}, nil
	}

	// Convertir a slice
	arraySlice, ok := arrayValue.([]interface{})
	if !ok {
		fmt.Printf("[DEBUG] Loop Node - arrayValue is not []interface{}, type: %T\n", arrayValue)
		return nil, fmt.Errorf("'%s' no es un array válido", params.ArraySource)
	}

	fmt.Printf("[DEBUG] Loop Node - arraySlice length: %d, operation: %s\n", len(arraySlice), params.Operation)

	switch params.Operation {
	case "map":
		return n.executeMap(arraySlice, params, context)
	case "filter":
		return n.executeFilter(arraySlice, params, context)
	case "forEach":
		return n.executeForEach(arraySlice, params)
	default:
		return nil, fmt.Errorf("operación no soportada: %s", params.Operation)
	}
}

// executeMap aplica una transformación a cada item
func (n *LoopNode) executeMap(array []interface{}, params LoopParams, context map[string]map[string]interface{}) (map[string]interface{}, error) {
	if params.MapExpression == "" {
		return nil, fmt.Errorf("mapExpression es requerido para la operación 'map'")
	}

	results := make([]interface{}, len(array))

	for i, item := range array {
		// Crear contexto temporal para este item
		// Aplanar el contexto: copiar todas las variables existentes
		itemContext := make(map[string]map[string]interface{})
		for k, v := range context {
			itemContext[k] = v
		}

		// Agregar variables del loop en un nodo temporal "loop"
		// Esto permite acceder a {{item.name}} o {{index}}
		loopVars := map[string]interface{}{
			params.IndexVariable: i,
		}

		// Si el item es un mapa, agregar sus campos directamente con el prefijo item
		if itemMap, ok := item.(map[string]interface{}); ok {
			for k, v := range itemMap {
				loopVars[params.ItemVariable+"."+k] = v
			}
			loopVars[params.ItemVariable] = item // También mantener el item completo
		} else {
			loopVars[params.ItemVariable] = item
		}

		itemContext["loop"] = loopVars

		// Interpolar la expresión
		result := interpolateString(params.MapExpression, itemContext)
		results[i] = result
		if i < 3 {
			fmt.Printf("[DEBUG] Loop Map - item %d: %v -> result: %v\n", i, item, result)
		}
	}

	fmt.Printf("[DEBUG] Loop Map - final results length: %d, first 3: %v\n", len(results), results[:min(3, len(results))])
	return map[string]interface{}{
		"items": results,
		"count": len(results),
	}, nil
}

// executeFilter filtra items que cumplen una condición
func (n *LoopNode) executeFilter(array []interface{}, params LoopParams, context map[string]map[string]interface{}) (map[string]interface{}, error) {
	if params.FilterExpr == "" {
		return nil, fmt.Errorf("filterExpr es requerido para la operación 'filter'")
	}

	results := []interface{}{}

	for i, item := range array {
		// Crear contexto temporal para este item
		itemContext := make(map[string]map[string]interface{})
		for k, v := range context {
			itemContext[k] = v
		}

		// Agregar variables del loop
		loopVars := map[string]interface{}{
			params.IndexVariable: i,
		}

		// Si el item es un mapa, agregar sus campos con el prefijo item
		if itemMap, ok := item.(map[string]interface{}); ok {
			for k, v := range itemMap {
				loopVars[params.ItemVariable+"."+k] = v
			}
			loopVars[params.ItemVariable] = item
		} else {
			loopVars[params.ItemVariable] = item
		}

		itemContext["loop"] = loopVars

		// Evaluar condición
		condition := interpolateString(params.FilterExpr, itemContext)
		// Si la condición evaluada es "true" o contiene el item, incluirlo
		if condition == "true" || strings.Contains(strings.ToLower(condition), "true") {
			results = append(results, item)
		}
	}

	return map[string]interface{}{
		"items": results,
		"count": len(results),
	}, nil
}

// executeForEach simplemente pasa los items al siguiente nodo
// Útil para combinar con otros nodos que procesen arrays
func (n *LoopNode) executeForEach(array []interface{}, params LoopParams) (map[string]interface{}, error) {
	// Crear array de items con metadata
	items := make([]interface{}, len(array))
	for i, item := range array {
		items[i] = map[string]interface{}{
			params.ItemVariable:  item,
			params.IndexVariable: i,
		}
	}

	return map[string]interface{}{
		"items": items,
		"count": len(array),
	}, nil
}
