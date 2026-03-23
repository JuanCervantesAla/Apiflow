package services

import (
	"capyflow/api/models"
	"encoding/json"
	"fmt"
	"strings"
)

func (es *ExecutorService) hydrateNodeConnection(node *models.Node, userID string) error {
	if es.DB == nil || node == nil || node.Parameters == "" || userID == "" {
		return nil
	}

	params := map[string]interface{}{}
	if err := json.Unmarshal([]byte(node.Parameters), &params); err != nil {
		// Ignore non-JSON parameters for backward compatibility.
		return nil
	}

	connectionID := ""
	if id, ok := params["connectionId"].(string); ok {
		connectionID = strings.TrimSpace(id)
	}
	if connectionID == "" {
		if id, ok := params["credentialId"].(string); ok {
			connectionID = strings.TrimSpace(id)
		}
	}
	if connectionID == "" {
		return nil
	}

	var conn models.Connection
	if err := es.DB.First(&conn, "id = ? AND user_id = ? AND is_active = ?", connectionID, userID, true).Error; err != nil {
		return fmt.Errorf("connection not found or inactive")
	}

	secrets, err := conn.GetSecrets()
	if err != nil {
		return fmt.Errorf("failed to load connection secrets")
	}

	provider := strings.ToLower(strings.TrimSpace(conn.Provider))
	nodeType := strings.ToLower(strings.TrimSpace(node.Type))

	switch {
	case nodeType == "email" && provider == "sendgrid":
		if token, ok := params["sendgridApiToken"].(string); !ok || strings.TrimSpace(token) == "" {
			if v, ok := secrets["apiKey"]; ok {
				params["sendgridApiToken"] = fmt.Sprint(v)
			}
		}
		if from, ok := params["from"].(string); !ok || strings.TrimSpace(from) == "" {
			if v, ok := secrets["fromEmail"]; ok {
				params["from"] = fmt.Sprint(v)
			}
		}
		if fromName, ok := params["fromName"].(string); !ok || strings.TrimSpace(fromName) == "" {
			if v, ok := secrets["fromName"]; ok {
				params["fromName"] = fmt.Sprint(v)
			}
		}
		if region, ok := params["sendgridDataResidency"].(string); !ok || strings.TrimSpace(region) == "" {
			if v, ok := secrets["dataResidency"]; ok {
				params["sendgridDataResidency"] = fmt.Sprint(v)
			}
		}
		if p, ok := params["provider"].(string); !ok || strings.TrimSpace(p) == "" {
			params["provider"] = "sendgrid-api"
		}

	case nodeType == "telegram" && provider == "telegram":
		if chatID, ok := params["chatId"].(string); !ok || strings.TrimSpace(chatID) == "" {
			if v, ok := secrets["chatId"]; ok {
				params["chatId"] = fmt.Sprint(v)
			}
		}
	}

	updated, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("failed to update node parameters with connection")
	}
	node.Parameters = string(updated)
	return nil
}
