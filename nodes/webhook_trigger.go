package nodes

import "capyflow/api/models"

type WebhookTriggerNode struct{}

func (n *WebhookTriggerNode) Execute(
	node *models.Node,
	prev map[string]map[string]interface{},
) (map[string]interface{}, error) {

	if ctx, ok := prev["__webhook__"]; ok {
		return ctx, nil
	}

	return map[string]interface{}{}, nil
}
