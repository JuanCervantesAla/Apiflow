package handlers

import (
	"capyflow/api/models"
	"capyflow/api/services"
	"capyflow/api/validators"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type FlowHandler struct {
	DB               *gorm.DB
	analyticsService *services.AnalyticsService
}

func NewFlowHandler(db *gorm.DB) *FlowHandler {
	return &FlowHandler{
		DB:               db,
		analyticsService: services.NewAnalyticsService(db),
	}
}

// GetAllFlows - GET /api/flows (user's flows only)
func (h *FlowHandler) GetAllFlows(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	var flows []models.Flow
	result := h.DB.Preload("Nodes").Preload("Edges").Where("user_id = ?", userId).Find(&flows)
	if result.Error != nil {
		RespondError(w, http.StatusInternalServerError, "Error getting all flows")
		return
	}
	RespondJSON(w, http.StatusOK, flows)
}

// GetFlow - GET /api/flows/{id} (only if belongs to user)
func (h *FlowHandler) GetFlow(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	vars := mux.Vars(r)
	flowID := vars["id"]

	var flow models.Flow
	result := h.DB.Preload("Nodes").Preload("Edges").First(&flow, "id = ? AND user_id = ?", flowID, userId)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			RespondError(w, http.StatusNotFound, "Flow not found")
			return
		}
		RespondError(w, http.StatusInternalServerError, "Error getting flow")
		return
	}
	RespondJSON(w, http.StatusOK, flow)
}

// CreateFlow - POST /api/flows (associated with user)
func (h *FlowHandler) CreateFlow(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	var req models.FlowCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Incorrect body request format")
		return
	}

	// Default to manual if not specified
	creationMethod := req.CreationMethod
	if creationMethod == "" {
		creationMethod = models.CreationMethodManual
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
		RespondError(w, http.StatusInternalServerError, "Error creating flow")
		return
	}

	// Create analytics for the flow
	if _, err := h.analyticsService.CreateFlowAnalytics(flow.ID, creationMethod); err != nil {
		// Log error but don't fail flow creation
		// In production, you might want to use a proper logger
		println("Warning: Failed to create analytics for flow:", flow.ID)
	}

	RespondJSON(w, http.StatusCreated, flow)
}

// UpdateFlow - PUT /api/flows/{id} (only if belongs to user)
func (h *FlowHandler) UpdateFlow(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	vars := mux.Vars(r)
	flowID := vars["id"]

	var req models.FlowUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid data")
		return
	}

	var flow models.Flow
	if err := h.DB.First(&flow, "id = ? AND user_id = ?", flowID, userId).Error; err != nil {
		RespondError(w, http.StatusBadRequest, "Flow not found")
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
		RespondError(w, http.StatusInternalServerError, "Error updating flow")
	}
	RespondJSON(w, http.StatusOK, flow)
}

// DeleteFlow - DELETE /api/flows/{id} (only if belongs to user)
func (h *FlowHandler) DeleteFlow(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	vars := mux.Vars(r)
	flowID := vars["id"]

	result := h.DB.Delete(&models.Flow{}, "id = ? AND user_id = ?", flowID, userId)
	if result.Error != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to delete flow")
		return
	}
	if result.RowsAffected == 0 {
		RespondError(w, http.StatusNotFound, "Flow not found")
		return
	}
	RespondJSON(w, http.StatusOK, map[string]string{"message": "Flow deleted"})
}

// SaveFlowData - POST /api/flows/{id}/save (only if belongs to user)
func (h *FlowHandler) SaveFlowData(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	vars := mux.Vars(r)
	flowID := vars["id"]

	var payload struct {
		Nodes []models.Node `json:"nodes"`
		Edges []models.Edge `json:"edges"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid data")
		return
	}

	var flow models.Flow
	if err := h.DB.First(&flow, "id = ? AND user_id = ?", flowID, userId).Error; err != nil {
		RespondError(w, http.StatusNotFound, "Flow not found")
		return
	}

	h.DB.Where("flow_id = ?", flowID).Delete(&models.Node{})
	h.DB.Where("flow_id = ?", flowID).Delete(&models.Edge{})

	// Create a map to translate node IDs (to maintain references in edges)
	idMap := make(map[string]string)

	// Apply defaults and validate nodes before saving
	for i := range payload.Nodes {
		// Generate a new unique UUID for each node
		oldID := payload.Nodes[i].ID
		newID := uuid.New().String()
		idMap[oldID] = newID
		payload.Nodes[i].ID = newID

		payload.Nodes[i].FlowID = flowID
		payload.Nodes[i].CreatedAt = time.Now()
		payload.Nodes[i].UpdatedAt = time.Now()

		// Apply automatic defaults
		if err := validators.ApplyDefaults(&payload.Nodes[i]); err != nil {
			RespondError(w, http.StatusBadRequest, fmt.Sprintf("Error applying defaults to node %s: %v", payload.Nodes[i].Label, err))
			return
		}

		// Validate parameters
		if err := validators.ValidateNode(&payload.Nodes[i]); err != nil {
			RespondError(w, http.StatusBadRequest, fmt.Sprintf("Validation error: %v", err))
			return
		}
	}
	if len(payload.Nodes) > 0 {
		if err := h.DB.Create(&payload.Nodes).Error; err != nil {
			RespondError(w, http.StatusInternalServerError, "Error to save nodes")
			return
		}
	}
	for i := range payload.Edges {
		// Update source and target references with new IDs
		if newSource, ok := idMap[payload.Edges[i].Source]; ok {
			payload.Edges[i].Source = newSource
		}
		if newTarget, ok := idMap[payload.Edges[i].Target]; ok {
			payload.Edges[i].Target = newTarget
		}

		// Generate a new unique UUID for each edge
		payload.Edges[i].ID = uuid.New().String()

		payload.Edges[i].FlowID = flowID
		payload.Edges[i].CreatedAt = time.Now()
		payload.Edges[i].UpdatedAt = time.Now()
		// Debug: print incoming edges
		println("DEBUG Edge received:", payload.Edges[i].ID)
		println("  Source:", payload.Edges[i].Source)
		println("  Target:", payload.Edges[i].Target)
		println("  SourceHandle:", payload.Edges[i].SourceHandle)
		println("  TargetHandle:", payload.Edges[i].TargetHandle)
	}
	if len(payload.Edges) > 0 {
		if err := h.DB.Create(&payload.Edges).Error; err != nil {
			RespondError(w, http.StatusInternalServerError, "Error to save edges")
			return
		}
	}
	flow.UpdatedAt = time.Now()
	h.DB.Save(&flow)
	RespondJSON(w, http.StatusOK, map[string]string{"message": "Flow saved"})
}

// RespondJSON is a helper to return the response with the status
func RespondJSON(w http.ResponseWriter, status int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(response)
}

func RespondError(w http.ResponseWriter, status int, message string) {
	RespondJSON(w, status, map[string]string{"error": message})
}
