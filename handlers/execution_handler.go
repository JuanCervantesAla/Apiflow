package handlers

import (
	"capyflow/api/models"
	"capyflow/api/services"
	"capyflow/api/websocket"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type ExecutionHandler struct {
	DB              *gorm.DB
	ExecutorService *services.ExecutorService
}

// NewExecutionHandler
func NewExecutionHandler(db *gorm.DB, hub *websocket.Hub) *ExecutionHandler {
	return &ExecutionHandler{
		DB:              db,
		ExecutorService: services.NewExecutorService(hub),
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

	execution := models.Execution{
		ID:          uuid.New().String(),
		FlowID:      flowID,
		UserID:      userId,
		Status:      models.ExecutionStatusRunning,
		StartedAt:   time.Now(),
		TriggerType: "manual",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := h.DB.Create(&execution).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Error al crear registro de ejecución")
		return
	}

	result := h.ExecutorService.ExecuteFlow(&flow, userId, execution.ID)
	now := time.Now()

	resultsJSON, _ := json.Marshal(result.Results)
	executedNodesJSON, _ := json.Marshal(result.ExecutedNodes)

	execution.Status = models.ExecutionStatus(result.Status)
	execution.FinishedAt = &now
	execution.DurationMs = result.DurationMs
	execution.Results = string(resultsJSON)
	execution.ExecutedNodes = string(executedNodesJSON)
	execution.ErrorMessage = result.ErrorMessage
	execution.UpdatedAt = now

	h.DB.Save(&execution)

	response := map[string]interface{}{
		"executionId": execution.ID,
		"result":      result,
	}

	respondJSON(w, http.StatusOK, response)
}

// GetFlowExecutions - GET /api/flows/{id}/executions
func (h *ExecutionHandler) GetFlowExecutions(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	vars := mux.Vars(r)
	flowID := vars["id"]

	var flow models.Flow
	if err := h.DB.First(&flow, "id = ? AND user_id = ?", flowID, userId).Error; err != nil {
		respondError(w, http.StatusNotFound, "Flow no encontrado")
		return
	}

	var executions []models.Execution
	if err := h.DB.Where("flow_id = ?", flowID).
		Order("started_at DESC").
		Limit(50).
		Find(&executions).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Error al obtener ejecuciones")
		return
	}

	respondJSON(w, http.StatusOK, executions)
}

// GetExecution - GET /api/executions/{id}
func (h *ExecutionHandler) GetExecution(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	vars := mux.Vars(r)
	executionID := vars["id"]

	var execution models.Execution
	if err := h.DB.First(&execution, "id = ? AND user_id = ?", executionID, userId).Error; err != nil {
		respondError(w, http.StatusNotFound, "Ejecución no encontrada")
		return
	}

	respondJSON(w, http.StatusOK, execution)
}

// GetAllExecutions - GET /api/executions (todas las del usuario)
func (h *ExecutionHandler) GetAllExecutions(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)

	var executions []struct {
		models.Execution
		FlowName string `json:"flowName"`
	}

	if err := h.DB.Table("executions").
		Select("executions.*, flows.name as flow_name").
		Joins("LEFT JOIN flows ON flows.id = executions.flow_id").
		Where("executions.user_id = ?", userId).
		Order("executions.started_at DESC").
		Limit(100).
		Scan(&executions).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Error al obtener ejecuciones")
		return
	}

	respondJSON(w, http.StatusOK, executions)
}
