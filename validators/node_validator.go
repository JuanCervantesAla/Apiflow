package validators

import (
	"capyflow/api/models"
	"encoding/json"
	"errors"
	"fmt"
)

// NodeSchema define el schema de validación para un tipo de nodo
type NodeSchema struct {
	Type       string                     `json:"type"`
	Required   []string                   `json:"required"`
	Parameters map[string]ParameterSchema `json:"parameters"`
}

type ParameterSchema struct {
	Type     string        `json:"type"`
	Required bool          `json:"required"`
	Enum     []interface{} `json:"enum,omitempty"`
	Min      *float64      `json:"min,omitempty"`
	Max      *float64      `json:"max,omitempty"`
	Pattern  string        `json:"pattern,omitempty"`
}

// Schemas de validación para cada tipo de nodo
var NodeSchemas = map[string]NodeSchema{
	"set-data": {
		Type:     "set-data",
		Required: []string{"data"},
		Parameters: map[string]ParameterSchema{
			"data": {
				Type:     "object",
				Required: true,
			},
		},
	},
	"http-request": {
		Type:     "http-request",
		Required: []string{"url", "method"},
		Parameters: map[string]ParameterSchema{
			"url": {
				Type:     "string",
				Required: true,
			},
			"method": {
				Type:     "string",
				Required: true,
				Enum:     []interface{}{"GET", "POST", "PUT", "DELETE", "PATCH"},
			},
			"headers": {
				Type:     "object",
				Required: false,
			},
			"body": {
				Type:     "string",
				Required: false,
			},
		},
	},
	"if-condition": {
		Type:     "if-condition",
		Required: []string{"left", "operator", "right"},
		Parameters: map[string]ParameterSchema{
			"left": {
				Type:     "string",
				Required: true,
			},
			"operator": {
				Type:     "string",
				Required: true,
				Enum:     []interface{}{"==", "!=", ">", "<", ">=", "<=", "contains"},
			},
			"right": {
				Type:     "string",
				Required: true,
			},
		},
	},
	"transform-data": {
		Type:     "transform-data",
		Required: []string{"operation", "field"},
		Parameters: map[string]ParameterSchema{
			"operation": {
				Type:     "string",
				Required: true,
				Enum:     []interface{}{"map", "filter"},
			},
			"field": {
				Type:     "string",
				Required: true,
			},
			"multiply": {
				Type:     "number",
				Required: false,
			},
			"equals": {
				Type:     "string",
				Required: false,
			},
		},
	},
	"log": {
		Type:     "log",
		Required: []string{},
		Parameters: map[string]ParameterSchema{
			"label": {
				Type:     "string",
				Required: false,
			},
		},
	},
}

// ValidateNode valida los parámetros de un nodo según su schema
func ValidateNode(node *models.Node) error {
	schema, exists := NodeSchemas[node.Type]
	if !exists {
		// Si no hay schema, permitir (nodos sin validación estricta)
		return nil
	}

	if node.Parameters == "" {
		if len(schema.Required) > 0 {
			return fmt.Errorf("node %s (%s) requires parameters: %v", node.Label, node.Type, schema.Required)
		}
		return nil
	}

	var params map[string]interface{}
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return fmt.Errorf("invalid JSON parameters for node %s", node.Label)
	}

	// Validar campos requeridos
	for _, required := range schema.Required {
		if _, ok := params[required]; !ok {
			return fmt.Errorf("node %s (%s) missing required parameter: %s", node.Label, node.Type, required)
		}
	}

	// Validar cada parámetro
	for key, value := range params {
		paramSchema, ok := schema.Parameters[key]
		if !ok {
			continue // Permitir parámetros extra
		}

		if err := validateParameter(key, value, paramSchema); err != nil {
			return fmt.Errorf("node %s (%s) parameter '%s': %v", node.Label, node.Type, key, err)
		}
	}

	return nil
}

func validateParameter(name string, value interface{}, schema ParameterSchema) error {
	// Validar tipo
	switch schema.Type {
	case "string":
		if _, ok := value.(string); !ok {
			return errors.New("must be a string")
		}
	case "number":
		switch value.(type) {
		case float64, int, int64:
			// OK
		default:
			return errors.New("must be a number")
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return errors.New("must be a boolean")
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return errors.New("must be an array")
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return errors.New("must be an object")
		}
	}

	// Validar enum
	if len(schema.Enum) > 0 {
		valid := false
		for _, enumVal := range schema.Enum {
			if value == enumVal {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("must be one of: %v", schema.Enum)
		}
	}

	// Validar min/max para números
	if schema.Type == "number" {
		var num float64
		switch v := value.(type) {
		case float64:
			num = v
		case int:
			num = float64(v)
		case int64:
			num = float64(v)
		}

		if schema.Min != nil && num < *schema.Min {
			return fmt.Errorf("must be >= %v", *schema.Min)
		}
		if schema.Max != nil && num > *schema.Max {
			return fmt.Errorf("must be <= %v", *schema.Max)
		}
	}

	return nil
}
