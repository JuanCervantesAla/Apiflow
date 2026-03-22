package nodes

import (
	"capyflow/api/models"
	"encoding/json"
	"time"
)

type ManualTriggerNode struct{}

func (n *ManualTriggerNode) Execute(
	node *models.Node,
	context map[string]map[string]interface{},
) (map[string]interface{}, error) {

	// Start with basic trigger metadata
	output := map[string]interface{}{
		"triggered": true,
		"timestamp": time.Now().Unix(),
		"message":   "Flow initializated manually",
	}

	// If the node has parameters configured (e.g. csvContent),
	// expose them directly in the output so downstream nodes
	// can reference {{node-1.output.csvContent}}.
	if node.Parameters != "" {
		var params map[string]interface{}
		if err := json.Unmarshal([]byte(node.Parameters), &params); err == nil {
			for k, v := range params {
				output[k] = v
			}
		}
	}

	return output, nil
}
