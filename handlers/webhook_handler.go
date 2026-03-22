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

// HandleWebhook handles incoming webhook requests (POST and GET)
// POST /api/webhooks/:id
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flowID := vars["id"]

	if flowID == "" {
		RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "Flow ID is required"})
		return
	}

	// Validate that the flow exists
	var flow models.Flow
	if err := h.DB.Preload("Nodes").Preload("Edges").First(&flow, "id = ?", flowID).Error; err != nil {
		RespondJSON(w, http.StatusNotFound, map[string]string{"error": "Flow not found"})
		return
	}

	// Read the webhook body
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

	// Verify that the flow has a trigger that can be started via webhook
	hasWebhookTrigger := false
	for _, node := range flow.Nodes {
		if node.Type == "webhook-trigger" || node.Type == "telegram-trigger" {
			hasWebhookTrigger = true
			break
		}
	}

	if !hasWebhookTrigger {
		RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "Flow does not have a webhook trigger"})
		return
	}

	// Create initial context with webhook data
	initialContext := map[string]map[string]interface{}{
		"webhook": payload,
	}

	// Execute the flow asynchronously
	executionID := uuid.New().String()
	userID := flow.UserID

	// Execute in background
	go func() {
		executorService := services.NewExecutorService(h.Hub)
		executorService.ExecuteFlowWithContext(&flow, initialContext, userID, executionID)
	}()

	// Return immediate response
	RespondJSON(w, http.StatusAccepted, map[string]interface{}{
		"message":     "Webhook received and flow execution started",
		"executionId": executionID,
		"flowId":      flowID,
	})
}

// extractHeadersFromRequest extracts relevant headers from the request
func extractHeadersFromRequest(r *http.Request) map[string]string {
	headers := make(map[string]string)

	// Common webhook headers
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

// GetWebhookURL returns the webhook URL for a flow
// GET /api/webhooks/:id/url
func (h *WebhookHandler) GetWebhookURL(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flowID := vars["id"]

	if flowID == "" {
		RespondJSON(w, http.StatusBadRequest, map[string]string{"error": "Flow ID is required"})
		return
	}

	// Validate that the flow exists
	var flow models.Flow
	if err := h.DB.First(&flow, "id = ?", flowID).Error; err != nil {
		RespondJSON(w, http.StatusNotFound, map[string]string{"error": "Flow not found"})
		return
	}

	// Build the webhook URL
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
