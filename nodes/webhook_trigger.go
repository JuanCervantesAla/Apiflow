package nodes

import (
	"capyflow/api/models"
	"encoding/json"
)

type WebhookTriggerNode struct{}

type WebhookParams struct {
	// The webhook doesn't need parameters, it just processes the received payload
	ValidationEnabled bool   `json:"validationEnabled"` // Optional: validate signature
	Secret            string `json:"secret"`            // Optional: secret for validation
}

func (n *WebhookTriggerNode) Execute(node *models.Node, context map[string]map[string]interface{}) (map[string]interface{}, error) {
	var params WebhookParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		// If there are no parameters, use defaults
		params.ValidationEnabled = false
	}

	// The webhook already received the data and passed it in the context
	// Just return the payload that comes in the initial context

	// Look for the webhook payload in the context
	if webhookData, exists := context["webhook"]; exists {
		return webhookData, nil
	}

	// If there is no webhook data, return basic structure
	return map[string]interface{}{
		"triggered": true,
		"source":    "webhook",
		"timestamp": 0,
	}, nil
}
