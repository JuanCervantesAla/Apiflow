package nodes

import "capyflow/api/models"

type NodeHandler interface {
	Execute(
		node *models.Node,
		context map[string]map[string]interface{},
	) (map[string]interface{}, error)
}
