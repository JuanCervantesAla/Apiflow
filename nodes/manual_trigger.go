package nodes

import (
	"capyflow/api/models"
	"time"
)

type ManualTriggerNode struct{}

func (n *ManualTriggerNode) Execute(
	node *models.Node,
	context map[string]map[string]interface{},
) (map[string]interface{}, error) {
	return map[string]interface{}{
		"triggered": true,
		"timestamp": time.Now().Unix(),
		"message":   "Flow initializated manually",
	}, nil
}
