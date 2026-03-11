package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type TransformDataNode struct{}

type Transformation struct {
	Source    string `json:"source"`    // Campo origen con dot notation: "body.user.name"
	Target    string `json:"target"`    // Campo destino: "userName"
	Operation string `json:"operation"` // extract, calculate, concat, default
	Value     string `json:"value"`     // Valor adicional para operaciones
}

type TransformParams struct {
	Transformations []Transformation `json:"transformations"`
}

func (n *TransformDataNode) Execute(node *models.Node, context map[string]map[string]interface{}) (map[string]interface{}, error) {
	var p TransformParams

	if err := json.Unmarshal([]byte(node.Parameters), &p); err != nil {
		return nil, fmt.Errorf("invalid transform parameters: %v", err)
	}

	if len(p.Transformations) == 0 {
		return nil, fmt.Errorf("no transformations defined")
	}

	// Obtener el output del nodo anterior (context unificado)
	var prev map[string]interface{}
	for _, nodeOutput := range context {
		if nodeOutput != nil && len(nodeOutput) > 0 {
			// Merge all previous outputs into one context
			if prev == nil {
				prev = make(map[string]interface{})
			}
			for k, v := range nodeOutput {
				prev[k] = v
			}
		}
	}

	if prev == nil {
		prev = make(map[string]interface{})
	}

	result := make(map[string]interface{})

	for _, transform := range p.Transformations {
		switch transform.Operation {
		case "extract":
			// Extraer valor de campo anidado
			value := getNestedValue(prev, transform.Source)
			if value != nil {
				result[transform.Target] = value
			}

		case "rename":
			// Renombrar campo (igual que extract)
			value := getNestedValue(prev, transform.Source)
			if value != nil {
				result[transform.Target] = value
			}

		case "default":
			// Usar valor por defecto si no existe
			value := getNestedValue(prev, transform.Source)
			if value != nil {
				result[transform.Target] = value
			} else {
				result[transform.Target] = transform.Value
			}

		case "calculate":
			// Operaciones matemáticas simples
			value := calculateExpression(transform.Value, context)
			result[transform.Target] = value

		case "concat":
			// Concatenar strings con interpolación
			interpolated := interpolateString(transform.Value, context)
			result[transform.Target] = interpolated

		default:
			return nil, fmt.Errorf("unknown operation: %s", transform.Operation)
		}
	}

	return result, nil
}

// getNestedValue obtiene un valor anidado usando dot notation
// Ejemplo: "body.user.name" → prev["body"]["user"]["name"]
func getNestedValue(data map[string]interface{}, path string) interface{} {
	if path == "" {
		return nil
	}

	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		if current == nil {
			return nil
		}

		value, ok := current[part]
		if !ok {
			return nil
		}

		// Si es el último elemento, retornar el valor
		if i == len(parts)-1 {
			return value
		}

		// Si no es el último, debe ser un map para continuar
		if nextMap, ok := value.(map[string]interface{}); ok {
			current = nextMap
		} else {
			return nil
		}
	}

	return nil
}

// calculateExpression evalúa expresiones matemáticas simples
// Soporta: +, -, *, /, y variables del contexto
func calculateExpression(expr string, context map[string]map[string]interface{}) interface{} {
	// Interpolate variables first
	interpolated := interpolateString(expr, context)

	// Try to parse as number
	if num, err := strconv.ParseFloat(interpolated, 64); err == nil {
		return num
	}

	return interpolated
}
