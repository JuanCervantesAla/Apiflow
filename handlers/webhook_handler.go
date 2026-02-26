package handlers

import (
	"capyflow/api/models"
	"capyflow/api/services"
	"capyflow/api/websocket"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type WebhookHandler struct {
	DB  *gorm.DB
	Hub *websocket.Hub
}

func NewWebhookHandler(db *gorm.DB, hub *websocket.Hub) *WebhookHandler {
	return &WebhookHandler{
		DB:  db,
		Hub: hub,
	}
}

// HandleWebhook maneja las peticiones webhook entrantes (POST y GET)
// POST /api/webhooks/:id
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flowID := vars["id"]

	if flowID == "" {
		RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "Flow ID is required"})
		return
	}

	// Validar que el flow existe
	var flow models.Flow
	if err := h.DB.Preload("Nodes").Preload("Edges").First(&flow, "id = ?", flowID).Error; err != nil {
		RespondJSON(w, http.StatusNotFound, map[string]string{"error": "Flow not found"})
		return
	}

	// Leer el body del webhook
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "Failed to read request body"})
		return
	}
	defer r.Body.Close()

	var payload map[string]interface{}
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			payload = map[string]interface{}{
				"body": string(bodyBytes),
			}
		}
	} else {
		payload = map[string]interface{}{}
	}
	payload["method"] = r.Method
	payload["headers"] = extractHeadersFromRequest(r)
	payload["query"] = r.URL.Query()
	payload["timestamp"] = time.Now().Unix()

	// Verificar que el flow tenga un webhook trigger
	hasWebhookTrigger := false
	for _, node := range flow.Nodes {
		if node.Type == "webhook-trigger" {
			hasWebhookTrigger = true
			break
		}
	}

	if !hasWebhookTrigger {
		RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "Flow does not have a webhook trigger"})
		return
	}

	// Crear contexto inicial con los datos del webhook
	initialContext := map[string]map[string]interface{}{
		"webhook": payload,
	}

	// Ejecutar el flow de manera asíncrona
	executionID := uuid.New().String()
	userID := flow.UserID

	// Ejecutar en background
	go func() {
		executorService := services.NewExecutorService(h.Hub)
		executorService.ExecuteFlowWithContext(&flow, initialContext, userID, executionID)
	}()

	// Retornar respuesta inmediata
	RespondJSON(w, http.StatusAccepted, map[string]interface{}{
		"message":     "Webhook received and flow execution started",
		"executionId": executionID,
		"flowId":      flowID,
	})
}

// extractHeadersFromRequest extrae headers relevantes del request
func extractHeadersFromRequest(r *http.Request) map[string]string {
	headers := make(map[string]string)

	// Headers comunes de webhooks
	relevantHeaders := []string{
		"Content-Type",
		"User-Agent",
		"X-GitHub-Event",
		"X-Hub-Signature",
		"X-Hub-Signature-256",
		"X-Webhook-Signature",
		"Authorization",
	}

	for _, key := range relevantHeaders {
		if value := r.Header.Get(key); value != "" {
			headers[key] = value
		}
	}

	return headers
}

// GetWebhookURL retorna la URL del webhook para un flow
// GET /api/webhooks/:id/url
func (h *WebhookHandler) GetWebhookURL(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flowID := vars["id"]

	if flowID == "" {
		RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "Flow ID is required"})
		return
	}

	// Validar que el flow existe
	var flow models.Flow
	if err := h.DB.First(&flow, "id = ?", flowID).Error; err != nil {
		RespondJSON(w, http.StatusNotFound, map[string]string{"error": "Flow not found"})
		return
	}

	// Construir la URL del webhook
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	webhookURL := scheme + "://" + r.Host + "/api/webhooks/" + flowID

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"flowId":     flowID,
		"webhookUrl": webhookURL,
		"methods":    []string{"POST", "GET"},
	})
}
