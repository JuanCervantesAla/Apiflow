package handlers

import (
	"capyflow/api/models"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type FlowHandler struct {
	DB *gorm.DB
}

func NewFlowHandler(db *gorm.DB) *FlowHandler {
	return &FlowHandler{DB: db}
}

// GetAllFlows - GET /api/flows (solo los del usuario)
func (h *FlowHandler) GetAllFlows(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	var flows []models.Flow
	result := h.DB.Preload("Nodes").Preload("Edges").Where("user_id = ?", userId).Find(&flows)
	if result.Error != nil {
		respondError(w, http.StatusInternalServerError, "Error getting all flows")
		return
	}
	respondJSON(w, http.StatusOK, flows)
}

// GetFlow - GET /api/flows/{id} (solo si es del usuario)
func (h *FlowHandler) GetFlow(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	vars := mux.Vars(r)
	flowID := vars["id"]

	var flow models.Flow
	result := h.DB.Preload("Nodes").Preload("Edges").First(&flow, "id = ? AND user_id = ?", flowID, userId)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			respondError(w, http.StatusNotFound, "Flujo no encontrado")
			return
		}
		respondError(w, http.StatusInternalServerError, "Error al obtener flujo")
		return
	}
	respondJSON(w, http.StatusOK, flow)
}

// CreateFlow - POST /api/flows (asociado al usuario)
func (h *FlowHandler) CreateFlow(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	var req models.FlowCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Incorrect body request format")
		return
	}
	flow := models.Flow{
		ID:          uuid.New().String(),
		UserID:      userId,
		Name:        req.Name,
		Description: req.Description,
		Status:      models.FlowStatusDraft,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := h.DB.Create(&flow).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Error creating flow")
		return
	}
	respondJSON(w, http.StatusCreated, flow)
}

// UpdateFlow - PUT /api/flows/{id} (solo si es del usuario)
func (h *FlowHandler) UpdateFlow(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	vars := mux.Vars(r)
	flowID := vars["id"]

	var req models.FlowUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid data")
		return
	}

	var flow models.Flow
	if err := h.DB.First(&flow, "id = ? AND user_id = ?", flowID, userId).Error; err != nil {
		respondError(w, http.StatusBadRequest, "Flow not found")
		return
	}

	if req.Name != "" {
		flow.Name = req.Name
	}
	if req.Description != "" {
		flow.Description = req.Description
	}
	if req.Status != "" {
		flow.Status = req.Status
	}
	flow.UpdatedAt = time.Now()

	if err := h.DB.Save(&flow).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Error updating flow")
	}
	respondJSON(w, http.StatusOK, flow)
}

// DeleteFlow - DELETE /api/flows/{id} (solo si es del usuario)
func (h *FlowHandler) DeleteFlow(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	vars := mux.Vars(r)
	flowID := vars["id"]

	result := h.DB.Delete(&models.Flow{}, "id = ? AND user_id = ?", flowID, userId)
	if result.Error != nil {
		respondError(w, http.StatusInternalServerError, "Failed to delete flow")
		return
	}
	if result.RowsAffected == 0 {
		respondError(w, http.StatusNotFound, "Flow not found")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "Flow deleted"})
}

// SaveFlowData - POST /api/flows/{id}/save (solo si es del usuario)
func (h *FlowHandler) SaveFlowData(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	vars := mux.Vars(r)
	flowID := vars["id"]

	var payload struct {
		Nodes []models.Node `json:"nodes"`
		Edges []models.Edge `json:"edges"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid data")
		return
	}

	var flow models.Flow
	if err := h.DB.First(&flow, "id = ? AND user_id = ?", flowID, userId).Error; err != nil {
		respondError(w, http.StatusNotFound, "Flow not found")
		return
	}

	h.DB.Where("flow_id = ?", flowID).Delete(&models.Node{})
	h.DB.Where("flow_id = ?", flowID).Delete(&models.Edge{})

	for i := range payload.Nodes {
		payload.Nodes[i].FlowID = flowID
		payload.Nodes[i].CreatedAt = time.Now()
		payload.Nodes[i].UpdatedAt = time.Now()
	}
	if len(payload.Nodes) > 0 {
		if err := h.DB.Create(&payload.Nodes).Error; err != nil {
			respondError(w, http.StatusInternalServerError, "Error to save nodes")
			return
		}
	}
	for i := range payload.Edges {
		payload.Edges[i].FlowID = flowID
		payload.Edges[i].CreatedAt = time.Now()
		payload.Edges[i].UpdatedAt = time.Now()
	}
	if len(payload.Edges) > 0 {
		if err := h.DB.Create(&payload.Edges).Error; err != nil {
			respondError(w, http.StatusInternalServerError, "Error to save edges")
			return
		}
	}
	flow.UpdatedAt = time.Now()
	h.DB.Save(&flow)
	respondJSON(w, http.StatusOK, map[string]string{"message": "Flow saved"})
}

// Helper to return the response with the status
func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(response)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
