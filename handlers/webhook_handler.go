package handlers

import (
	"capyflow/api/models"
	"capyflow/api/services"
	"capyflow/api/websocket"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type WebhookHandler struct {
	DB              *gorm.DB
	ExecutorService *services.ExecutorService
}

func NewWebhookHandler(db *gorm.DB, hub *websocket.Hub) *WebhookHandler {
	return &WebhookHandler{
		DB:              db,
		ExecutorService: services.NewExecutorService(hub),
	}
}

// POST /api/webhooks/{id}
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flowID := vars["id"]

	var flow models.Flow
	if err := h.DB.Preload("Nodes").Preload("Edges").
		First(&flow, "id = ?", flowID).Error; err != nil {
		respondError(w, http.StatusNotFound, "Flow not found")
		return
	}

	body := map[string]interface{}{}
	_ = json.NewDecoder(r.Body).Decode(&body)

	headers := map[string]interface{}{}
	for k, v := range r.Header {
		headers[k] = v
	}

	initialContext := map[string]map[string]interface{}{
		"__webhook__": {
			"body":    body,
			"headers": headers,
		},
	}

	result := h.ExecutorService.ExecuteFlowWithContext(&flow, initialContext, "webhook", "")
	respondJSON(w, http.StatusOK, result)
}
