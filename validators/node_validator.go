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
	Type        string        `json:"type"`
	Required    bool          `json:"required"`
	Default     interface{}   `json:"default,omitempty"`
	Enum        []interface{} `json:"enum,omitempty"`
	Min         *float64      `json:"min,omitempty"`
	Max         *float64      `json:"max,omitempty"`
	Pattern     string        `json:"pattern,omitempty"`
	Description string        `json:"description,omitempty"`
	Example     string        `json:"example,omitempty"`
	Advanced    bool          `json:"advanced,omitempty"` // Si true, solo mostrar en modo avanzado
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
				Type:        "string",
				Required:    true,
				Description: "URL endpoint to request",
				Example:     "https://api.example.com/data",
				Advanced:    false, // BÁSICO
			},
			"method": {
				Type:        "string",
				Required:    true,
				Enum:        []interface{}{"GET", "POST", "PUT", "DELETE", "PATCH"},
				Default:     "GET",
				Description: "HTTP method",
				Advanced:    false, // BÁSICO
			},
			"headers": {
				Type:        "object",
				Required:    false,
				Description: "Custom HTTP headers",
				Example:     `{"Authorization": "Bearer token123"}`,
				Advanced:    true, // AVANZADO
			},
			"body": {
				Type:        "string",
				Required:    false,
				Description: "Request body (for POST/PUT/PATCH)",
				Advanced:    true, // AVANZADO
			},
		},
	},
	// JSON Parser
	"json-parser": {
		Type:     "json-parser",
		Required: []string{"json"},
		Parameters: map[string]ParameterSchema{
			"json": {
				Type:        "string",
				Required:    true,
				Description: "JSON string to parse",
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
				Default:  "Log",
			},
		},
	},
	// Nodos AI
	"gpt": {
		Type:     "gpt",
		Required: []string{"prompt"},
		Parameters: map[string]ParameterSchema{
			"prompt": {
				Type:        "string",
				Required:    true,
				Description: "The prompt to send to GPT",
				Example:     "Summarize this text: {{input}}",
				Advanced:    false, // BÁSICO: Campo principal
			},
			"model": {
				Type:        "string",
				Required:    false,
				Default:     "gpt-4",
				Enum:        []interface{}{"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo"},
				Description: "GPT model to use",
				Advanced:    true, // AVANZADO: Auto-usa default en básico
			},
			"temperature": {
				Type:        "number",
				Required:    false,
				Default:     0.7,
				Min:         floatPtr(0.0),
				Max:         floatPtr(2.0),
				Description: "Randomness of the output (0-2)",
				Advanced:    true, // AVANZADO: Parámetro técnico
			},
			"max_tokens": {
				Type:        "number",
				Required:    false,
				Default:     1000,
				Min:         floatPtr(1),
				Max:         floatPtr(4096),
				Description: "Maximum tokens in response",
				Advanced:    true, // AVANZADO: Parámetro técnico
			},
		},
	},
	"claude": {
		Type:     "claude",
		Required: []string{"prompt"},
		Parameters: map[string]ParameterSchema{
			"prompt": {
				Type:        "string",
				Required:    true,
				Description: "The prompt to send to Claude",
				Example:     "Analyze this data: {{input}}",
				Advanced:    false, // BÁSICO
			},
			"model": {
				Type:        "string",
				Required:    false,
				Default:     "claude-3-5-sonnet-20241022",
				Enum:        []interface{}{"claude-3-5-sonnet-20241022", "claude-3-opus-20240229", "claude-3-haiku-20240307"},
				Description: "Claude model to use",
				Advanced:    true, // AVANZADO
			},
			"temperature": {
				Type:        "number",
				Required:    false,
				Default:     0.7,
				Min:         floatPtr(0.0),
				Max:         floatPtr(1.0),
				Description: "Randomness of the output (0-1)",
				Advanced:    true, // AVANZADO
			},
			"max_tokens": {
				Type:        "number",
				Required:    false,
				Default:     1000,
				Min:         floatPtr(1),
				Max:         floatPtr(4096),
				Description: "Maximum tokens in response",
				Advanced:    true, // AVANZADO
			},
		},
	},
	"groq": {
		Type:     "groq",
		Required: []string{"prompt"},
		Parameters: map[string]ParameterSchema{
			"prompt": {
				Type:        "string",
				Required:    true,
				Description: "The prompt to send to Groq",
				Example:     "Analyze this data: {{input}}",
				Advanced:    false, // BÁSICO
			},
			"model": {
				Type:        "string",
				Required:    false,
				Default:     "llama-3.3-70b-versatile",
				Enum:        []interface{}{"llama-3.3-70b-versatile", "llama-3.1-8b-instant", "mixtral-8x7b-32768", "gemma2-9b-it"},
				Description: "Groq model to use (API key configured in .env)",
				Advanced:    true, // AVANZADO
			},
			"temperature": {
				Type:        "number",
				Required:    false,
				Default:     0.7,
				Min:         floatPtr(0.0),
				Max:         floatPtr(2.0),
				Description: "Randomness of the output (0-2)",
				Advanced:    true, // AVANZADO
			},
			"maxTokens": {
				Type:        "number",
				Required:    false,
				Default:     1024,
				Min:         floatPtr(1),
				Max:         floatPtr(32768),
				Description: "Maximum tokens in response",
				Advanced:    true, // AVANZADO
			},
			"systemPrompt": {
				Type:        "string",
				Required:    false,
				Description: "System prompt to set context/instructions",
				Example:     "You are a helpful assistant that analyzes data.",
				Advanced:    true, // AVANZADO
			},
		},
	},
	// Email
	"email": {
		Type:     "email",
		Required: []string{"to", "subject"},
		Parameters: map[string]ParameterSchema{
			"to": {
				Type:        "string",
				Required:    true,
				Description: "Recipient email address",
				Example:     "user@example.com",
				Pattern:     "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$",
				Advanced:    false, // BÁSICO
			},
			"subject": {
				Type:        "string",
				Required:    true,
				Description: "Email subject line",
				Advanced:    false, // BÁSICO
			},
			"body": {
				Type:        "string",
				Required:    false,
				Default:     "",
				Description: "Email body content",
				Advanced:    false, // BÁSICO (cuerpo es importante)
			},
			"from": {
				Type:        "string",
				Required:    false,
				Description: "Sender email (if different from default)",
				Advanced:    true, // AVANZADO (usa default del sistema)
			},
		},
	},
	// Telegram
	"telegram": {
		Type:     "telegram",
		Required: []string{"message"},
		Parameters: map[string]ParameterSchema{
			"message": {
				Type:        "string",
				Required:    true,
				Description: "Message to send",
			},
			"chat_id": {
				Type:        "string",
				Required:    false,
				Description: "Telegram chat ID (optional if configured globally)",
			},
		},
	},
	// Loop
	"loop": {
		Type:     "loop",
		Required: []string{"items"},
		Parameters: map[string]ParameterSchema{
			"items": {
				Type:        "array",
				Required:    true,
				Description: "Array of items to iterate over",
			},
			"max_iterations": {
				Type:        "number",
				Required:    false,
				Default:     100,
				Min:         floatPtr(1),
				Max:         floatPtr(1000),
				Description: "Maximum iterations to prevent infinite loops",
			},
		},
	},
	// Delay
	"delay": {
		Type:     "delay",
		Required: []string{"duration"},
		Parameters: map[string]ParameterSchema{
			"duration": {
				Type:        "number",
				Required:    true,
				Min:         floatPtr(0),
				Max:         floatPtr(300),
				Description: "Delay duration in seconds (max 5 minutes)",
				Example:     "5",
			},
		},
	},
	// Filter
	"filter": {
		Type:     "filter",
		Required: []string{"field", "operator", "value"},
		Parameters: map[string]ParameterSchema{
			"field": {
				Type:        "string",
				Required:    true,
				Description: "Field name to filter on",
			},
			"operator": {
				Type:        "string",
				Required:    true,
				Enum:        []interface{}{"==", "!=", ">", "<", ">=", "<=", "contains"},
				Description: "Comparison operator",
			},
			"value": {
				Type:        "string",
				Required:    true,
				Description: "Value to compare against",
			},
		},
	},
	// Database
	"database": {
		Type:     "database",
		Required: []string{"driver", "connectionUrl", "query"},
		Parameters: map[string]ParameterSchema{
			"driver": {
				Type:        "string",
				Required:    true,
				Enum:        []interface{}{"postgres", "mysql", "sqlite3"},
				Description: "Database driver to use",
				Example:     "postgres",
				Advanced:    false, // BÁSICO
			},
			"connectionUrl": {
				Type:        "string",
				Required:    true,
				Description: "Database connection URL/DSN",
				Example:     "postgres://user:pass@localhost:5432/dbname?sslmode=disable",
				Advanced:    false, // BÁSICO
			},
			"query": {
				Type:        "string",
				Required:    true,
				Description: "SQL query to execute (use {{variable}} for interpolation)",
				Example:     "SELECT * FROM users WHERE id = {{userId}}",
				Advanced:    false, // BÁSICO
			},
			"timeout": {
				Type:        "number",
				Required:    false,
				Default:     30000,
				Min:         floatPtr(1000),
				Max:         floatPtr(300000),
				Description: "Query timeout in milliseconds",
				Advanced:    true, // AVANZADO
			},
			"maxRetries": {
				Type:        "number",
				Required:    false,
				Default:     0,
				Min:         floatPtr(0),
				Max:         floatPtr(5),
				Description: "Maximum number of retries on failure",
				Advanced:    true, // AVANZADO
			},
			"retryDelay": {
				Type:        "number",
				Required:    false,
				Default:     1000,
				Min:         floatPtr(100),
				Max:         floatPtr(10000),
				Description: "Delay between retries in milliseconds",
				Advanced:    true, // AVANZADO
			},
			"queryParams": {
				Type:        "object",
				Required:    false,
				Description: "Named parameters for parameterized queries",
				Example:     `{"userId": 123, "status": "active"}`,
				Advanced:    true, // AVANZADO
			},
			"returnMetadata": {
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Include query metadata in response",
				Advanced:    true, // AVANZADO
			},
		},
	},
	// Split
	"split": {
		Type:     "split",
		Required: []string{"array", "chunkSize"},
		Parameters: map[string]ParameterSchema{
			"array": {
				Type:        "array",
				Required:    true,
				Description: "Array to split into chunks",
			},
			"chunkSize": {
				Type:        "number",
				Required:    true,
				Min:         floatPtr(1),
				Description: "Size of each chunk",
			},
		},
	},
	// Merge
	"merge": {
		Type:     "merge",
		Required: []string{"arrays"},
		Parameters: map[string]ParameterSchema{
			"arrays": {
				Type:        "array",
				Required:    true,
				Description: "Arrays to merge",
			},
		},
	},
	// Function (JavaScript execution)
	"function": {
		Type:     "function",
		Required: []string{"code"},
		Parameters: map[string]ParameterSchema{
			"code": {
				Type:        "string",
				Required:    true,
				Description: "JavaScript code to execute",
				Example:     "return input * 2;",
			},
		},
	},
	// Sort
	"sort": {
		Type:     "sort",
		Required: []string{"array", "field"},
		Parameters: map[string]ParameterSchema{
			"array": {
				Type:        "array",
				Required:    true,
				Description: "Array to sort",
			},
			"field": {
				Type:        "string",
				Required:    true,
				Description: "Field to sort by",
			},
			"order": {
				Type:        "string",
				Required:    false,
				Default:     "asc",
				Enum:        []interface{}{"asc", "desc"},
				Description: "Sort order",
			},
		},
	},
	// CSV Parser
	"csv-parser": {
		Type:     "csv-parser",
		Required: []string{"input"},
		Parameters: map[string]ParameterSchema{
			"input": {
				Type:        "string",
				Required:    true,
				Description: "CSV string to parse",
			},
			"delimiter": {
				Type:        "string",
				Required:    false,
				Default:     ",",
				Description: "CSV delimiter character",
			},
			"hasHeader": {
				Type:        "boolean",
				Required:    false,
				Default:     true,
				Description: "First row contains headers",
			},
		},
	},
	// Regex Extract
	"regex-extract": {
		Type:     "regex-extract",
		Required: []string{"input", "pattern"},
		Parameters: map[string]ParameterSchema{
			"input": {
				Type:        "string",
				Required:    true,
				Description: "Input string to extract from",
			},
			"pattern": {
				Type:        "string",
				Required:    true,
				Description: "Regular expression pattern",
				Example:     "[0-9]+",
			},
			"group": {
				Type:        "number",
				Required:    false,
				Default:     0,
				Description: "Capture group to extract (0 for full match)",
			},
		},
	},
	// Switch (multiple conditions)
	"switch": {
		Type:     "switch",
		Required: []string{"value", "cases"},
		Parameters: map[string]ParameterSchema{
			"value": {
				Type:        "string",
				Required:    true,
				Description: "Value to match",
			},
			"cases": {
				Type:        "array",
				Required:    true,
				Description: "Array of {value, output} cases",
			},
			"default": {
				Type:        "string",
				Required:    false,
				Description: "Default output if no match",
			},
		},
	},
	// Error Handler
	"error-handler": {
		Type:     "error-handler",
		Required: []string{},
		Parameters: map[string]ParameterSchema{
			"retry": {
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Retry on error",
			},
			"maxRetries": {
				Type:        "number",
				Required:    false,
				Default:     3,
				Min:         floatPtr(1),
				Max:         floatPtr(10),
				Description: "Maximum retry attempts",
			},
			"fallbackValue": {
				Type:        "string",
				Required:    false,
				Description: "Fallback value on error",
			},
		},
	},
	// Stop
	"stop": {
		Type:     "stop",
		Required: []string{},
		Parameters: map[string]ParameterSchema{
			"message": {
				Type:        "string",
				Required:    false,
				Description: "Stop message",
			},
		},
	},
	// Manual Trigger
	"manual-trigger": {
		Type:       "manual-trigger",
		Required:   []string{},
		Parameters: map[string]ParameterSchema{},
	},
	// Webhook Trigger
	"webhook-trigger": {
		Type:     "webhook-trigger",
		Required: []string{},
		Parameters: map[string]ParameterSchema{
			"method": {
				Type:        "string",
				Required:    false,
				Default:     "POST",
				Enum:        []interface{}{"GET", "POST", "PUT", "DELETE"},
				Description: "HTTP method",
			},
			"path": {
				Type:        "string",
				Required:    false,
				Description: "Webhook path",
			},
		},
	},
}

// Helper function
func floatPtr(f float64) *float64 {
	return &f
}

// getRegisteredNodeTypes retorna la lista de tipos de nodos válidos
func getRegisteredNodeTypes() []string {
	types := make([]string, 0, len(NodeSchemas))
	for nodeType := range NodeSchemas {
		types = append(types, nodeType)
	}
	return types
}

// GetNodeSchema devuelve el schema de un tipo de nodo (para el frontend)
func GetNodeSchema(nodeType string) (NodeSchema, bool) {
	schema, exists := NodeSchemas[nodeType]
	return schema, exists
}

// GetAllNodeSchemas devuelve todos los schemas de nodos disponibles
func GetAllNodeSchemas() map[string]NodeSchema {
	return NodeSchemas
}

// GetNodeSchemaFiltered devuelve el schema filtrado según el modo (basic o advanced)
func GetNodeSchemaFiltered(nodeType string, mode string) (NodeSchema, bool) {
	schema, exists := NodeSchemas[nodeType]
	if !exists {
		return NodeSchema{}, false
	}

	// Si el modo es "basic", filtrar solo parámetros no-avanzados
	if mode == "basic" {
		filteredParams := make(map[string]ParameterSchema)
		for key, param := range schema.Parameters {
			if !param.Advanced {
				filteredParams[key] = param
			}
		}
		schema.Parameters = filteredParams
	}

	return schema, true
}

// GetAllNodeSchemasFiltered devuelve todos los schemas filtrados según el modo
func GetAllNodeSchemasFiltered(mode string) map[string]NodeSchema {
	if mode != "basic" {
		return NodeSchemas
	}

	// Filtrar todos los schemas para modo básico
	filtered := make(map[string]NodeSchema)
	for nodeType, schema := range NodeSchemas {
		filteredParams := make(map[string]ParameterSchema)
		for key, param := range schema.Parameters {
			if !param.Advanced {
				filteredParams[key] = param
			}
		}
		filteredSchema := schema
		filteredSchema.Parameters = filteredParams
		filtered[nodeType] = filteredSchema
	}
	return filtered
}

// ApplyDefaults completa automáticamente los parámetros con sus valores por defecto
func ApplyDefaults(node *models.Node) error {
	schema, exists := NodeSchemas[node.Type]
	if !exists {
		// Si no hay schema, no aplicar defaults
		return nil
	}

	var params map[string]interface{}
	if node.Parameters == "" {
		params = make(map[string]interface{})
	} else {
		if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
			return fmt.Errorf("invalid JSON parameters for node %s", node.Label)
		}
	}

	// Aplicar defaults para parámetros no provistos
	modified := false
	for paramName, paramSchema := range schema.Parameters {
		if _, exists := params[paramName]; !exists && paramSchema.Default != nil {
			params[paramName] = paramSchema.Default
			modified = true
		}
	}

	// Si se modificó algo, actualizar el JSON
	if modified {
		updatedJSON, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal updated parameters: %v", err)
		}
		node.Parameters = string(updatedJSON)
	}

	return nil
}

// ValidateNode valida los parámetros de un nodo según su schema
func ValidateNode(node *models.Node) error {
	schema, exists := NodeSchemas[node.Type]
	if !exists {
		// RECHAZAR nodos con tipos no registrados
		return fmt.Errorf("node type '%s' is not registered in NodeSchemas. Valid types are: %v",
			node.Type, getRegisteredNodeTypes())
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
