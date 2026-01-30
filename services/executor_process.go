package services

import (
	"capyflow/api/models"
	"capyflow/api/nodes"
	"fmt"
)

func (es *ExecutorService) processNode(
	node *models.Node,
	prev map[string]map[string]interface{},
) (map[string]interface{}, error) {

	handler, ok := nodes.Registry[node.Type]
	if !ok {
		return nil, fmt.Errorf("Node not implemented: %s", node.Type)
	}

	return handler.Execute(node, prev)
}
