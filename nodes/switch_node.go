package nodes

import (
	"encoding/json"
	"fmt"
	"strings"

	"capyflow/api/models"
)

type SwitchNode struct{}

type SwitchParams struct {
	InputValue  interface{}              `json:"inputValue"`  // Value to evaluate
	Cases       []map[string]interface{} `json:"cases"`       // Array of cases: [{value: "x", output: "case1"}, ...]
	DefaultCase string                   `json:"defaultCase"` // Output port for default case
	Mode        string                   `json:"mode"`        // "equals" or "contains"
}

func (n *SwitchNode) Execute(node *models.Node, prev map[string]map[string]interface{}) (map[string]interface{}, error) {
	var params SwitchParams
	paramsJSON, _ := json.Marshal(node.Parameters)
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return nil, fmt.Errorf("error parsing parameters: %v", err)
	}

	// Default mode
	if params.Mode == "" {
		params.Mode = "equals"
	}

	// Validate cases
	if len(params.Cases) == 0 {
		return nil, fmt.Errorf("at least one case is required")
	}

	// Convert input value to string for comparison
	inputStr := fmt.Sprintf("%v", params.InputValue)

	// Find matching case
	var matchedOutput string
	var matched bool

	for _, caseItem := range params.Cases {
		caseValue, hasValue := caseItem["value"]
		caseOutput, hasOutput := caseItem["output"]

		if !hasValue || !hasOutput {
			continue
		}

		caseValueStr := fmt.Sprintf("%v", caseValue)
		caseOutputStr := fmt.Sprintf("%v", caseOutput)

		switch params.Mode {
		case "equals":
			if strings.EqualFold(inputStr, caseValueStr) {
				matchedOutput = caseOutputStr
				matched = true
			}
		case "contains":
			if strings.Contains(strings.ToLower(inputStr), strings.ToLower(caseValueStr)) {
				matchedOutput = caseOutputStr
				matched = true
			}
		}

		if matched {
			break
		}
	}

	// Use default case if no match
	if !matched {
		if params.DefaultCase != "" {
			matchedOutput = params.DefaultCase
		} else {
			matchedOutput = "default"
		}
	}

	return map[string]interface{}{
		"output":      matchedOutput, // Which output port to use
		"inputValue":  params.InputValue,
		"matched":     matched,
		"matchedCase": matchedOutput,
	}, nil
}
