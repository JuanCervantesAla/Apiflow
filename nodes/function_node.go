package nodes

import (
	"encoding/json"
	"fmt"
	"strings"

	"capyflow/api/models"

	"github.com/dop251/goja"
)

type FunctionNode struct{}

type FunctionParams struct {
	InputData interface{} `json:"inputData"`
	Code      string      `json:"code"`
}

func (n *FunctionNode) Execute(
	node *models.Node,
	prev map[string]map[string]interface{},
) (map[string]interface{}, error) {

	var params FunctionParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		return nil, fmt.Errorf("invalid function parameters: %v", err)
	}

	// Get JavaScript code
	code := params.Code
	if code == "" {
		return nil, fmt.Errorf("code parameter is required")
	}

	// Get input data. If not explicitly provided, try to infer
	// it from previous node outputs (common case: use "data"
	// array from a CSV Parser or similar node).
	inputData := params.InputData
	if inputData == nil {
		for _, out := range prev {
			if candidate, ok := out["data"]; ok {
				inputData = candidate
				break
			}
		}
	}

	// Create JavaScript VM
	vm := goja.New()

	// Set up console.log
	consoleObj := vm.NewObject()
	var logs []string
	consoleObj.Set("log", func(args ...interface{}) {
		var parts []string
		for _, arg := range args {
			parts = append(parts, fmt.Sprintf("%v", arg))
		}
		logs = append(logs, strings.Join(parts, " "))
	})
	vm.Set("console", consoleObj)

	// Set input data
	vm.Set("input", inputData)
	vm.Set("context", prev)

	// Set common utilities
	vm.Set("JSON", map[string]interface{}{
		"parse": func(str string) interface{} {
			var result interface{}
			if err := json.Unmarshal([]byte(str), &result); err != nil {
				return nil
			}
			return result
		},
		"stringify": func(obj interface{}) string {
			bytes, err := json.Marshal(obj)
			if err != nil {
				return ""
			}
			return string(bytes)
		},
	})

	// Always wrap user code in an IIFE so that
	// "return" statements are valid and execute
	// without causing a top-level return syntax error.
	code = fmt.Sprintf("(function() { %s })()", code)

	// Execute the code
	value, err := vm.RunString(code)
	if err != nil {
		return map[string]interface{}{
			"error":  err.Error(),
			"logs":   logs,
			"result": nil,
		}, fmt.Errorf("JavaScript execution error: %v", err)
	}

	// Export the result
	result := value.Export()

	return map[string]interface{}{
		"result": result,
		"logs":   logs,
		"error":  nil,
	}, nil
}
