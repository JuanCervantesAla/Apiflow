package validators

import (
	"capyflow/api/models"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// NodeSchema defines the validation schema for a node type
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
	Advanced    bool          `json:"advanced,omitempty"` // If true, only show in advanced mode
}

// Validation schemas for each node type
var NodeSchemas = map[string]NodeSchema{
	"set-data": {
		Type: "set-data",
		// Frontend y nodo SetDataNode usan la clave "values"
		// para el objeto de pares clave-valor. Alineamos el
		// validador para evitar que el backend rechace nodos
		// válidos que envían { "values": { ... } }.
		Required: []string{"values"},
		Parameters: map[string]ParameterSchema{
			"values": {
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
				Advanced:    false, // BASIC
			},
			"method": {
				Type:        "string",
				Required:    true,
				Enum:        []interface{}{"GET", "POST", "PUT", "DELETE", "PATCH"},
				Default:     "GET",
				Description: "HTTP method",
				Advanced:    false, // BASIC
			},
			"headers": {
				Type:        "object",
				Required:    false,
				Description: "Custom HTTP headers",
				Example:     `{"Authorization": "Bearer token123"}`,
				Advanced:    true, // ADVANCED
			},
			"body": {
				Type:        "object",
				Required:    false,
				Description: "Request body (for POST/PUT/PATCH)",
				Advanced:    true, // ADVANCED
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
			"jsonString": {
				Type:        "string",
				Required:    false,
				Description: "Alias used by frontend for JSON input",
			},
		},
	},
	"if-condition": {
		Type:     "if-condition",
		Required: []string{"field", "operator"},
		Parameters: map[string]ParameterSchema{
			"field": {
				Type:     "string",
				Required: true,
			},
			"operator": {
				Type:     "string",
				Required: true,
				Enum:     []interface{}{"==", "!=", ">", "<", ">=", "<=", "contains", "exists"},
			},
			"value": {
				Type:     "string",
				Required: false,
			},
			"left": {
				Type:     "string",
				Required: false,
			},
			"right": {
				Type:     "",
				Required: false,
			},
		},
	},
	"transform-data": {
		Type:     "transform-data",
		Required: []string{"transformations"},
		Parameters: map[string]ParameterSchema{
			"transformations": {
				Type:        "array",
				Required:    true,
				Description: "List of transformations to apply",
			},
		},
	},
	"log": {
		Type:     "log",
		Required: []string{"message"},
		Parameters: map[string]ParameterSchema{
			"message": {
				Type:     "string",
				Required: true,
			},
			"level": {
				Type:     "string",
				Required: false,
				Default:  "info",
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
				Advanced:    false, // BASIC
			},
			"model": {
				Type:        "string",
				Required:    false,
				Default:     "llama-3.3-70b-versatile",
				Enum:        []interface{}{"llama-3.3-70b-versatile", "llama-3.1-8b-instant", "mixtral-8x7b-32768", "gemma2-9b-it"},
				Description: "Groq model to use (API key configured in .env)",
				Advanced:    true, // ADVANCED
			},
			"temperature": {
				Type:        "number",
				Required:    false,
				Default:     0.7,
				Min:         floatPtr(0.0),
				Max:         floatPtr(2.0),
				Description: "Randomness of the output (0-2)",
				Advanced:    true, // ADVANCED
			},
			"maxTokens": {
				Type:        "number",
				Required:    false,
				Default:     1024,
				Min:         floatPtr(1),
				Max:         floatPtr(32768),
				Description: "Maximum tokens in response",
				Advanced:    true, // ADVANCED
			},
			"systemPrompt": {
				Type:        "string",
				Required:    false,
				Description: "System prompt to set context/instructions",
				Example:     "You are a helpful assistant that analyzes data.",
				Advanced:    true, // ADVANCED
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
				Advanced:    false, // BASIC
			},
			"subject": {
				Type:        "string",
				Required:    true,
				Description: "Email subject line",
				Advanced:    false, // BASIC
			},
			"body": {
				Type:        "string",
				Required:    false,
				Default:     "",
				Description: "Email body content",
				Advanced:    false, // BASIC (body is important)
			},
			"from": {
				Type:        "string",
				Required:    false,
				Description: "Sender email (if different from default)",
				Advanced:    true, // ADVANCED (uses system default)
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
			"chatId": {
				Type:        "string",
				Required:    false,
				Description: "Telegram chat ID (optional if configured globally)",
			},
			"parseMode": {
				Type:        "string",
				Required:    false,
				Description: "Telegram parse mode (Markdown/HTML)",
			},
		},
	},
	// Loop
	"loop": {
		Type:     "loop",
		Required: []string{"arraySource"},
		Parameters: map[string]ParameterSchema{
			"arraySource": {
				Type:        "string",
				Required:    true,
				Description: "Path to the array in context (e.g. data, items)",
			},
			"operation": {
				Type:        "string",
				Required:    false,
				Default:     "forEach",
				Enum:        []interface{}{"forEach", "map", "filter"},
				Description: "Type of loop operation to perform",
			},
			"mapExpression": {
				Type:        "string",
				Required:    false,
				Description: "For 'map': expression to apply to each item (use {{item}} and {{index}})",
			},
			"filterExpr": {
				Type:        "string",
				Required:    false,
				Description: "For 'filter': condition that each item must meet",
			},
			"itemVariable": {
				Type:        "string",
				Required:    false,
				Default:     "item",
				Description: "Variable name to reference each item in expressions",
			},
			"indexVariable": {
				Type:        "string",
				Required:    false,
				Default:     "index",
				Description: "Variable name for the loop index",
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
				Min:         floatPtr(1),
				Max:         floatPtr(300000),
				Description: "Delay duration in milliseconds (max 300000ms = 5 minutes)",
				Example:     "1000",
			},
		},
	},
	// Filter
	"filter": {
		Type:     "filter",
		Required: []string{"inputData", "operator"},
		Parameters: map[string]ParameterSchema{
			"inputData": {
				Type:        "",
				Required:    true,
				Description: "Array input data to filter",
			},
			"mode": {
				Type:        "string",
				Required:    false,
				Enum:        []interface{}{"keep", "remove"},
				Default:     "keep",
				Description: "Keep or remove matching elements",
			},
			"field": {
				Type:        "string",
				Required:    false,
				Description: "Field name to filter on",
			},
			"operator": {
				Type:        "string",
				Required:    true,
				Enum:        []interface{}{"equals", "notEquals", "contains", "startsWith", "endsWith", "greaterThan", "lessThan", "greaterOrEqual", "lessOrEqual", "isEmpty", "isNotEmpty"},
				Description: "Comparison operator",
			},
			"value": {
				Type:        "",
				Required:    false,
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
				Advanced:    false, // BASIC
			},
			"connectionUrl": {
				Type:        "string",
				Required:    true,
				Description: "Database connection URL/DSN",
				Example:     "postgres://user:pass@localhost:5432/dbname?sslmode=disable",
				Advanced:    false, // BASIC
			},
			"query": {
				Type:        "string",
				Required:    true,
				Description: "SQL query to execute (use {{variable}} for interpolation)",
				Example:     "SELECT * FROM users WHERE id = {{userId}}",
				Advanced:    false, // BASIC
			},
			"timeout": {
				Type:        "number",
				Required:    false,
				Default:     30000,
				Min:         floatPtr(1000),
				Max:         floatPtr(300000),
				Description: "Query timeout in milliseconds",
				Advanced:    true, // ADVANCED
			},
			"maxRetries": {
				Type:        "number",
				Required:    false,
				Default:     0,
				Min:         floatPtr(0),
				Max:         floatPtr(5),
				Description: "Maximum number of retries on failure",
				Advanced:    true, // ADVANCED
			},
			"retryDelay": {
				Type:        "number",
				Required:    false,
				Default:     1000,
				Min:         floatPtr(100),
				Max:         floatPtr(10000),
				Description: "Delay between retries in milliseconds",
				Advanced:    true, // ADVANCED
			},
			"queryParams": {
				Type:        "object",
				Required:    false,
				Description: "Named parameters for parameterized queries",
				Example:     `{"userId": 123, "status": "active"}`,
				Advanced:    true, // ADVANCED
			},
			"returnMetadata": {
				Type:        "boolean",
				Required:    false,
				Default:     false,
				Description: "Include query metadata in response",
				Advanced:    true, // ADVANCED
			},
		},
	},
	// Split
	"split": {
		Type:     "split",
		Required: []string{"inputData"},
		Parameters: map[string]ParameterSchema{
			"inputData": {
				Type:        "",
				Required:    true,
				Description: "Array input to split",
			},
			"mode": {
				Type:        "string",
				Required:    false,
				Enum:        []interface{}{"items", "batches", "field"},
				Default:     "items",
				Description: "Split mode",
			},
			"batchSize": {
				Type:        "number",
				Required:    false,
				Min:         floatPtr(1),
				Description: "Size of each batch (when mode=batches)",
			},
			"field": {
				Type:        "string",
				Required:    false,
				Description: "Field to extract (when mode=field)",
			},
		},
	},
	// Merge
	"merge": {
		Type:     "merge",
		Required: []string{},
		Parameters: map[string]ParameterSchema{
			"mode": {
				Type:        "string",
				Required:    false,
				Enum:        []interface{}{"append", "combine", "merge"},
				Default:     "append",
				Description: "Merge mode",
			},
			"input1": {
				Type:     "",
				Required: false,
			},
			"input2": {
				Type:     "",
				Required: false,
			},
			"input3": {
				Type:     "",
				Required: false,
			},
			"input4": {
				Type:     "",
				Required: false,
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
		Required: []string{"inputData"},
		Parameters: map[string]ParameterSchema{
			"inputData": {
				Type:        "",
				Required:    true,
				Description: "Array to sort",
			},
			"field": {
				Type:        "string",
				Required:    false,
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
		Required: []string{"inputData"},
		Parameters: map[string]ParameterSchema{
			"inputData": {
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
		Required: []string{"pattern"},
		Parameters: map[string]ParameterSchema{
			"inputData": {
				Type:        "string",
				Required:    false,
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
		Required: []string{"cases"},
		Parameters: map[string]ParameterSchema{
			"inputValue": {
				Type:        "",
				Required:    false,
				Description: "Value to match",
			},
			"cases": {
				Type:        "array",
				Required:    true,
				Description: "Array of {value, output} cases",
			},
			"defaultCase": {
				Type:        "string",
				Required:    false,
				Description: "Default output if no match",
			},
		},
	},
	// Error Handler
	"error-handler": {
		Type:     "error-handler",
		Required: []string{"inputData"},
		Parameters: map[string]ParameterSchema{
			"inputData": {
				Type:        "",
				Required:    true,
				Description: "Input data to inspect for errors",
			},
			"maxRetries": {
				Type:        "number",
				Required:    false,
				Default:     3,
				Min:         floatPtr(1),
				Max:         floatPtr(10),
				Description: "Maximum retry attempts",
			},
			"fallbackMode": {
				Type:        "string",
				Required:    false,
				Default:     "ignore",
				Enum:        []interface{}{"ignore", "default", "stop"},
				Description: "Fallback mode when retries are exhausted",
			},
			"defaultValue": {
				Type:        "",
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
			"condition": {
				Type:        "string",
				Required:    false,
				Default:     "always",
				Enum:        []interface{}{"always", "if-true", "if-false", "if-error"},
				Description: "Stop condition",
			},
			"inputValue": {
				Type:        "",
				Required:    false,
				Description: "Value to evaluate",
			},
			"stopMessage": {
				Type:        "string",
				Required:    false,
				Description: "Stop message",
			},
			"stopCode": {
				Type:        "string",
				Required:    false,
				Default:     "success",
				Enum:        []interface{}{"success", "error"},
				Description: "Stop result code",
			},
		},
	},
	// Manual Trigger
	"manual-trigger": {
		Type:       "manual-trigger",
		Required:   []string{},
		Parameters: map[string]ParameterSchema{},
	},
	// Cron Trigger
	"cron-trigger": {
		Type:     "cron-trigger",
		Required: []string{"intervalMinutes"},
		Parameters: map[string]ParameterSchema{
			"intervalMinutes": {
				Type:        "number",
				Required:    true,
				Min:         floatPtr(1),
				Description: "Interval in minutes between executions",
				Example:     "60",
			},
		},
	},
	// Telegram Trigger
	"telegram-trigger": {
		Type:       "telegram-trigger",
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

// getRegisteredNodeTypes returns the list of valid node types
func getRegisteredNodeTypes() []string {
	types := make([]string, 0, len(NodeSchemas))
	for nodeType := range NodeSchemas {
		types = append(types, nodeType)
	}
	return types
}

// GetNodeSchema returns the schema for a node type (for the frontend)
func GetNodeSchema(nodeType string) (NodeSchema, bool) {
	schema, exists := NodeSchemas[nodeType]
	return schema, exists
}

// GetAllNodeSchemas returns all available node schemas
func GetAllNodeSchemas() map[string]NodeSchema {
	return NodeSchemas
}

// GetNodeSchemaFiltered returns the schema filtered by mode (basic or advanced)
func GetNodeSchemaFiltered(nodeType string, mode string) (NodeSchema, bool) {
	schema, exists := NodeSchemas[nodeType]
	if !exists {
		return NodeSchema{}, false
	}

	// If mode is "basic", filter only non-advanced parameters
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

// GetAllNodeSchemasFiltered returns all schemas filtered by mode
func GetAllNodeSchemasFiltered(mode string) map[string]NodeSchema {
	if mode != "basic" {
		return NodeSchemas
	}

	// Filter all schemas for basic mode
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

// ApplyDefaults automatically fills in parameters with their default values
func ApplyDefaults(node *models.Node) error {
	schema, exists := NodeSchemas[node.Type]
	if !exists {
		// If no schema exists, don't apply defaults
		return nil
	}

	params, err := parseNodeParameters(node)
	if err != nil {
		return err
	}

	// Apply defaults for parameters not provided
	modified := false
	for paramName, paramSchema := range schema.Parameters {
		if _, exists := params[paramName]; !exists && paramSchema.Default != nil {
			params[paramName] = paramSchema.Default
			modified = true
		}
	}

	// If something was modified, update the JSON
	if modified {
		updatedJSON, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal updated parameters: %v", err)
		}
		node.Parameters = string(updatedJSON)
	}

	return nil
}

func parseNodeParameters(node *models.Node) (map[string]interface{}, error) {
	trimmed := strings.TrimSpace(node.Parameters)
	if trimmed == "" {
		return make(map[string]interface{}), nil
	}

	var params map[string]interface{}
	if err := json.Unmarshal([]byte(node.Parameters), &params); err == nil {
		return params, nil
	}

	// Backward compatibility: older flows may have stored raw CSV/text directly
	// in trigger parameters. Normalize to a JSON object so validation can continue.
	if node.Type == "manual-trigger" || node.Type == "telegram-trigger" {
		params = map[string]interface{}{"csvContent": node.Parameters}
		updatedJSON, err := json.Marshal(params)
		if err == nil {
			node.Parameters = string(updatedJSON)
		}
		return params, nil
	}

	return nil, fmt.Errorf("invalid JSON parameters for node %s", node.Label)
}

// ValidateNode validates a node's parameters against its schema
func ValidateNode(node *models.Node) error {
	schema, exists := NodeSchemas[node.Type]
	if !exists {
		// REJECT nodes with unregistered types
		return fmt.Errorf("node type '%s' is not registered in NodeSchemas. Valid types are: %v",
			node.Type, getRegisteredNodeTypes())
	}

	if node.Parameters == "" {
		if len(schema.Required) > 0 {
			return fmt.Errorf("node %s (%s) requires parameters: %v", node.Label, node.Type, schema.Required)
		}
		return nil
	}

	params, err := parseNodeParameters(node)
	if err != nil {
		return err
	}

	// Backwards compatibility and normalization for set-data:
	// older flows and some AI prompts may still use "data" instead of "values".
	if node.Type == "set-data" {
		if _, hasValues := params["values"]; !hasValues {
			if legacy, hasData := params["data"]; hasData {
				params["values"] = legacy
				delete(params, "data")
				if updated, err := json.Marshal(params); err == nil {
					node.Parameters = string(updated)
				}
			}
		}
	}

	if node.Type == "if-condition" {
		if _, hasField := params["field"]; !hasField {
			if legacy, hasLegacy := params["left"]; hasLegacy {
				params["field"] = legacy
			}
		}
		if _, hasValue := params["value"]; !hasValue {
			if legacy, hasLegacy := params["right"]; hasLegacy {
				params["value"] = legacy
			}
		}
		if updated, err := json.Marshal(params); err == nil {
			node.Parameters = string(updated)
		}
	}

	// Backwards compatibility for json-parser:
	// frontend uses jsonString while legacy/runtime may use json.
	if node.Type == "json-parser" {
		if _, hasJSON := params["json"]; !hasJSON {
			if modern, hasJSONString := params["jsonString"]; hasJSONString {
				params["json"] = modern
				if updated, err := json.Marshal(params); err == nil {
					node.Parameters = string(updated)
				}
			}
		}
	}

	// Backwards compatibility for csv-parser:
	// older flows may still use "input" instead of "inputData".
	if node.Type == "csv-parser" {
		if _, hasInputData := params["inputData"]; !hasInputData {
			if legacy, hasInput := params["input"]; hasInput {
				params["inputData"] = legacy
				delete(params, "input")
				if updated, err := json.Marshal(params); err == nil {
					node.Parameters = string(updated)
				}
			}
		}
	}

	if node.Type == "telegram" {
		if _, hasChatID := params["chatId"]; !hasChatID {
			if legacy, hasLegacy := params["chat_id"]; hasLegacy {
				params["chatId"] = legacy
				delete(params, "chat_id")
				if updated, err := json.Marshal(params); err == nil {
					node.Parameters = string(updated)
				}
			}
		}
	}

	if node.Type == "regex-extract" {
		if _, hasInputData := params["inputData"]; !hasInputData {
			if legacy, hasLegacy := params["input"]; hasLegacy {
				params["inputData"] = legacy
				delete(params, "input")
				if updated, err := json.Marshal(params); err == nil {
					node.Parameters = string(updated)
				}
			}
		}
	}

	if node.Type == "switch" {
		if _, hasInputValue := params["inputValue"]; !hasInputValue {
			if legacy, hasLegacy := params["value"]; hasLegacy {
				params["inputValue"] = legacy
				delete(params, "value")
			}
		}
		if _, hasDefaultCase := params["defaultCase"]; !hasDefaultCase {
			if legacy, hasLegacy := params["default"]; hasLegacy {
				params["defaultCase"] = legacy
				delete(params, "default")
			}
		}
		if updated, err := json.Marshal(params); err == nil {
			node.Parameters = string(updated)
		}
	}

	// Validate required fields
	for _, required := range schema.Required {
		if _, ok := params[required]; !ok {
			return fmt.Errorf("node %s (%s) missing required parameter: %s", node.Label, node.Type, required)
		}
	}

	// Validate each parameter
	for key, value := range params {
		paramSchema, ok := schema.Parameters[key]
		if !ok {
			continue // Allow extra parameters
		}

		if err := validateParameter(key, value, paramSchema); err != nil {
			return fmt.Errorf("node %s (%s) parameter '%s': %v", node.Label, node.Type, key, err)
		}
	}

	return nil
}

func validateParameter(name string, value interface{}, schema ParameterSchema) error {
	// Validate type
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

	// Validate enum
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

	// Validate min/max for numbers
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
