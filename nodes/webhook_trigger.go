package nodes

import (
	"capyflow/api/models"
	"encoding/json"
)

type WebhookTriggerNode struct{}

type WebhookParams struct {
	// El webhook no necesita parámetros, solo procesa el payload recibido
	ValidationEnabled bool   `json:"validationEnabled"` // Opcional: validar signature
	Secret            string `json:"secret"`            // Opcional: secret para validación
}

func (n *WebhookTriggerNode) Execute(node *models.Node, context map[string]map[string]interface{}) (map[string]interface{}, error) {
	var params WebhookParams
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		// Si no hay parámetros, usar defaults
		params.ValidationEnabled = false
	}

	// El webhook ya recibió los datos y los pasó en el contexto
	// Solo retornar el payload que viene en el contexto inicial

	// Buscar el payload del webhook en el contexto
	if webhookData, exists := context["webhook"]; exists {
		return webhookData, nil
	}

	// Si no hay datos del webhook, retornar estructura básica
	return map[string]interface{}{
		"triggered": true,
		"source":    "webhook",
		"timestamp": 0,
	}, nil
}
