package handlers

import (
	"capyflow/api/models"
	"capyflow/api/services"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type ExecutionHandler struct {
	DB              *gorm.DB
	ExecutorService *services.ExecutorService
}

// NewExecutionHandler
func NewExecutionHandler(db *gorm.DB) *ExecutionHandler {
	return &ExecutionHandler{
		DB:              db,
		ExecutorService: services.NewExecutorService(),
	}
}

// ExecuteFlow - POST /api/flows/{id}/execute
func (h *ExecutionHandler) ExecuteFlow(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	vars := mux.Vars(r)
	flowID := vars["id"]

	var flow models.Flow
	if err := h.DB.Preload("Nodes").Preload("Edges").
		First(&flow, "id = ? AND user_id = ?", flowID, userId).Error; err != nil {
		respondError(w, http.StatusNotFound, "Flujo no encontrado")
		return
	}

	for i, node := range flow.Nodes {
		log.Printf("  Nodo [%d]: ID=%s, Type=%s, Category=%s, Label=%s",
			i, node.ID, node.Type, node.Category, node.Label)
	}
	for i, edge := range flow.Edges {
		log.Printf("  Edge [%d]: %s -> %s", i, edge.Source, edge.Target)
	}

	result := h.ExecutorService.ExecuteFlow(&flow)

	respondJSON(w, http.StatusOK, result)
}
