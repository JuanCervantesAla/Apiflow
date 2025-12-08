package handlers

import (
	"capyflow/api/models"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type ExecutionHandler struct {
	DB *gorm.DB
}

// "Creates the connection"
func NewExecutionHandler(db *gorm.DB) *ExecutionHandler {
	return &ExecutionHandler{DB: db}
}

// ExecuteFlow - POST /api/flows/{id}/execute
func (h *ExecutionHandler) ExecuteFlow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flowID := vars["id"]

	//Searches for the flow
	var flow models.Flow
	if err := h.DB.Preload("Nodes").Preload("Edges").First(&flow, "id = ?", flowID).Error; err != nil {
		respondError(w, http.StatusNotFound, "Flujo no encontrado")
		return
	}

	result := h.executeFlowLogic(&flow)

	respondJSON(w, http.StatusOK, result)

}

//Execute basic logi of the flow

func (h *ExecutionHandler) executeFlowLogic(flow *models.Flow) map[string]interface{} {
	startTime := time.Now()
	executedNodes := []string{}

	//Finds the initial node
	nodeMap := make(map[string]*models.Node)
	for i := range flow.Nodes {
		nodeMap[flow.Nodes[i].ID] = &flow.Nodes[i]
	}

	for _, node := range flow.Nodes {
		//Update status
		node.Status = models.StatusRunning
		h.DB.Save(&node)

		//Simulates process
		time.Sleep(100 * time.Millisecond)

		node.Status = models.StatusSuccess
		now := time.Now()
		node.LastRun = &now
		node.ExecutionTimeMs = 100
		h.DB.Save(&node)

		executedNodes = append(executedNodes, node.ID)

	}

	duration := time.Since(startTime)

	//Return the results
	return map[string]interface{}{
		"status":        "success",
		"executedNodes": executedNodes,
		"durationMs":    duration.Milliseconds(),
		"startedAt":     startTime,
		"completedAt":   time.Now(),
	}

}
