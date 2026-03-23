package handlers

import (
	"capyflow/api/models"
	"capyflow/api/services"
	"capyflow/api/validators"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type FlowHandler struct {
	DB               *gorm.DB
	analyticsService *services.AnalyticsService
}

type FlowExportResponse struct {
	Version    string      `json:"version"`
	ExportedAt time.Time   `json:"exportedAt"`
	Flow       models.Flow `json:"flow"`
}

type FlowImportRequest struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Nodes       []models.Node `json:"nodes"`
	Edges       []models.Edge `json:"edges"`
	Flow        *struct {
		Name        string        `json:"name"`
		Description string        `json:"description"`
		Nodes       []models.Node `json:"nodes"`
		Edges       []models.Edge `json:"edges"`
	} `json:"flow,omitempty"`
}

type FlowCloneRequest struct {
	Name string `json:"name"`
}

type FlowVersionSnapshot struct {
	Nodes []models.Node `json:"nodes"`
	Edges []models.Edge `json:"edges"`
}

type FlowRollbackRequest struct {
	VersionID string `json:"versionId"`
}

type FlowShareResponse struct {
	ShareID      string     `json:"shareId"`
	ShareURL     string     `json:"shareUrl"`
	IsActive     bool       `json:"isActive"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`
	LastAccessAt *time.Time `json:"lastAccessAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type FlowShareUpdateRequest struct {
	IsActive  *bool  `json:"isActive,omitempty"`
	ExpiresAt string `json:"expiresAt,omitempty"`
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

	if err := h.saveFlowSnapshot(flowID, userId, payload.Nodes, payload.Edges, "manual-save", true); err != nil {
		RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	RespondJSON(w, http.StatusOK, map[string]string{"message": "Flow saved"})
}

func (h *FlowHandler) saveFlowSnapshot(
	flowID string,
	userID string,
	nodes []models.Node,
	edges []models.Edge,
	note string,
	createVersion bool,
) error {
	var flow models.Flow
	if err := h.DB.First(&flow, "id = ? AND user_id = ?", flowID, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("Flow not found")
		}
		return fmt.Errorf("Error loading flow")
	}

	now := time.Now()

	if createVersion {
		snapshot := FlowVersionSnapshot{Nodes: nodes, Edges: edges}
		snapshotJSON, err := json.Marshal(snapshot)
		if err != nil {
			return fmt.Errorf("Error serializing snapshot")
		}

		var latest models.FlowVersion
		nextVersion := 1
		if err := h.DB.Where("flow_id = ?", flowID).Order("version_number DESC").First(&latest).Error; err == nil {
			nextVersion = latest.VersionNumber + 1
		}

		version := models.FlowVersion{
			ID:            uuid.New().String(),
			FlowID:        flowID,
			UserID:        userID,
			VersionNumber: nextVersion,
			Note:          note,
			Snapshot:      string(snapshotJSON),
			CreatedAt:     now,
		}

		if err := h.DB.Create(&version).Error; err != nil {
			return fmt.Errorf("Error creating flow version")
		}
	}

	h.DB.Where("flow_id = ?", flowID).Delete(&models.Node{})
	h.DB.Where("flow_id = ?", flowID).Delete(&models.Edge{})

	// Create a map to translate node IDs (to maintain references in edges)
	idMap := make(map[string]string)

	// Apply defaults and validate nodes before saving
	for i := range nodes {
		// Generate a new unique UUID for each node
		oldID := nodes[i].ID
		newID := uuid.New().String()
		idMap[oldID] = newID
		nodes[i].ID = newID

		nodes[i].FlowID = flowID
		nodes[i].CreatedAt = now
		nodes[i].UpdatedAt = now

		// Apply automatic defaults
		if err := validators.ApplyDefaults(&nodes[i]); err != nil {
			return fmt.Errorf("Error applying defaults to node %s: %v", nodes[i].Label, err)
		}

		// Validate parameters
		if err := validators.ValidateNode(&nodes[i]); err != nil {
			return fmt.Errorf("Validation error: %v", err)
		}
	}
	if len(nodes) > 0 {
		if err := h.DB.Create(&nodes).Error; err != nil {
			return fmt.Errorf("Error to save nodes")
		}
	}
	for i := range edges {
		// Update source and target references with new IDs
		if newSource, ok := idMap[edges[i].Source]; ok {
			edges[i].Source = newSource
		}
		if newTarget, ok := idMap[edges[i].Target]; ok {
			edges[i].Target = newTarget
		}

		// Generate a new unique UUID for each edge
		edges[i].ID = uuid.New().String()

		edges[i].FlowID = flowID
		edges[i].CreatedAt = now
		edges[i].UpdatedAt = now
		// Debug: print incoming edges
		println("DEBUG Edge received:", edges[i].ID)
		println("  Source:", edges[i].Source)
		println("  Target:", edges[i].Target)
		println("  SourceHandle:", edges[i].SourceHandle)
		println("  TargetHandle:", edges[i].TargetHandle)
	}
	if len(edges) > 0 {
		if err := h.DB.Create(&edges).Error; err != nil {
			return fmt.Errorf("Error to save edges")
		}
	}
	flow.UpdatedAt = now
	h.DB.Save(&flow)

	return nil
}

// CloneFlow - POST /api/flows/{id}/clone (only if belongs to user)
func (h *FlowHandler) CloneFlow(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	flowID := mux.Vars(r)["id"]

	var req FlowCloneRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	var source models.Flow
	if err := h.DB.Preload("Nodes").Preload("Edges").First(&source, "id = ? AND user_id = ?", flowID, userId).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			RespondError(w, http.StatusNotFound, "Flow not found")
			return
		}
		RespondError(w, http.StatusInternalServerError, "Error getting source flow")
		return
	}

	cloneName := strings.TrimSpace(req.Name)
	if cloneName == "" {
		cloneName = source.Name + " (Copy)"
	}

	now := time.Now()
	cloned := models.Flow{
		ID:          uuid.New().String(),
		UserID:      userId,
		Name:        cloneName,
		Description: source.Description,
		Status:      models.FlowStatusDraft,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.DB.Create(&cloned).Error; err != nil {
		RespondError(w, http.StatusInternalServerError, "Error creating cloned flow")
		return
	}

	idMap := make(map[string]string, len(source.Nodes))
	clonedNodes := make([]models.Node, 0, len(source.Nodes))

	for _, node := range source.Nodes {
		newID := uuid.New().String()
		idMap[node.ID] = newID

		clonedNode := node
		clonedNode.ID = newID
		clonedNode.FlowID = cloned.ID
		clonedNode.Status = models.StatusIdle
		clonedNode.LastRun = nil
		clonedNode.ErrorMessage = ""
		clonedNode.ExecutionTimeMs = 0
		clonedNode.CreatedAt = now
		clonedNode.UpdatedAt = now
		clonedNodes = append(clonedNodes, clonedNode)
	}

	if len(clonedNodes) > 0 {
		if err := h.DB.Create(&clonedNodes).Error; err != nil {
			RespondError(w, http.StatusInternalServerError, "Error cloning nodes")
			return
		}
	}

	clonedEdges := make([]models.Edge, 0, len(source.Edges))
	for _, edge := range source.Edges {
		clonedEdge := edge
		clonedEdge.ID = uuid.New().String()
		clonedEdge.FlowID = cloned.ID
		if mapped, ok := idMap[edge.Source]; ok {
			clonedEdge.Source = mapped
		}
		if mapped, ok := idMap[edge.Target]; ok {
			clonedEdge.Target = mapped
		}
		clonedEdge.CreatedAt = now
		clonedEdge.UpdatedAt = now
		clonedEdges = append(clonedEdges, clonedEdge)
	}

	if len(clonedEdges) > 0 {
		if err := h.DB.Create(&clonedEdges).Error; err != nil {
			RespondError(w, http.StatusInternalServerError, "Error cloning edges")
			return
		}
	}

	if err := h.DB.Preload("Nodes").Preload("Edges").First(&cloned, "id = ?", cloned.ID).Error; err != nil {
		RespondError(w, http.StatusInternalServerError, "Error loading cloned flow")
		return
	}

	RespondJSON(w, http.StatusCreated, cloned)
}

// ExportFlow - GET /api/flows/{id}/export (only if belongs to user)
func (h *FlowHandler) ExportFlow(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	flowID := mux.Vars(r)["id"]

	var flow models.Flow
	if err := h.DB.Preload("Nodes").Preload("Edges").First(&flow, "id = ? AND user_id = ?", flowID, userId).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			RespondError(w, http.StatusNotFound, "Flow not found")
			return
		}
		RespondError(w, http.StatusInternalServerError, "Error exporting flow")
		return
	}

	RespondJSON(w, http.StatusOK, FlowExportResponse{
		Version:    "1.0",
		ExportedAt: time.Now(),
		Flow:       flow,
	})
}

// ImportFlow - POST /api/flows/import
func (h *FlowHandler) ImportFlow(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)

	var req FlowImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid import payload")
		return
	}

	name := strings.TrimSpace(req.Name)
	description := req.Description
	nodes := req.Nodes
	edges := req.Edges

	if req.Flow != nil {
		if name == "" {
			name = strings.TrimSpace(req.Flow.Name)
		}
		if description == "" {
			description = req.Flow.Description
		}
		if len(nodes) == 0 {
			nodes = req.Flow.Nodes
		}
		if len(edges) == 0 {
			edges = req.Flow.Edges
		}
	}

	if name == "" {
		name = "Imported Flow"
	}

	now := time.Now()
	importedFlow := models.Flow{
		ID:          uuid.New().String(),
		UserID:      userId,
		Name:        name,
		Description: description,
		Status:      models.FlowStatusDraft,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.DB.Create(&importedFlow).Error; err != nil {
		RespondError(w, http.StatusInternalServerError, "Error creating imported flow")
		return
	}

	idMap := make(map[string]string, len(nodes))
	preparedNodes := make([]models.Node, 0, len(nodes))
	for i := range nodes {
		newID := uuid.New().String()
		idMap[nodes[i].ID] = newID

		nodes[i].ID = newID
		nodes[i].FlowID = importedFlow.ID
		nodes[i].Status = models.StatusIdle
		nodes[i].LastRun = nil
		nodes[i].ErrorMessage = ""
		nodes[i].ExecutionTimeMs = 0
		nodes[i].CreatedAt = now
		nodes[i].UpdatedAt = now

		if err := validators.ApplyDefaults(&nodes[i]); err != nil {
			RespondError(w, http.StatusBadRequest, fmt.Sprintf("Error applying defaults to imported node %s: %v", nodes[i].Label, err))
			return
		}
		if err := validators.ValidateNode(&nodes[i]); err != nil {
			RespondError(w, http.StatusBadRequest, fmt.Sprintf("Validation error in imported node %s: %v", nodes[i].Label, err))
			return
		}

		preparedNodes = append(preparedNodes, nodes[i])
	}

	if len(preparedNodes) > 0 {
		if err := h.DB.Create(&preparedNodes).Error; err != nil {
			RespondError(w, http.StatusInternalServerError, "Error importing nodes")
			return
		}
	}

	preparedEdges := make([]models.Edge, 0, len(edges))
	for i := range edges {
		edges[i].ID = uuid.New().String()
		edges[i].FlowID = importedFlow.ID
		if mapped, ok := idMap[edges[i].Source]; ok {
			edges[i].Source = mapped
		}
		if mapped, ok := idMap[edges[i].Target]; ok {
			edges[i].Target = mapped
		}
		edges[i].CreatedAt = now
		edges[i].UpdatedAt = now
		preparedEdges = append(preparedEdges, edges[i])
	}

	if len(preparedEdges) > 0 {
		if err := h.DB.Create(&preparedEdges).Error; err != nil {
			RespondError(w, http.StatusInternalServerError, "Error importing edges")
			return
		}
	}

	if err := h.DB.Preload("Nodes").Preload("Edges").First(&importedFlow, "id = ?", importedFlow.ID).Error; err != nil {
		RespondError(w, http.StatusInternalServerError, "Error loading imported flow")
		return
	}

	RespondJSON(w, http.StatusCreated, importedFlow)
}

// GetFlowVersions - GET /api/flows/{id}/versions
func (h *FlowHandler) GetFlowVersions(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	flowID := mux.Vars(r)["id"]

	var flow models.Flow
	if err := h.DB.First(&flow, "id = ? AND user_id = ?", flowID, userId).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			RespondError(w, http.StatusNotFound, "Flow not found")
			return
		}
		RespondError(w, http.StatusInternalServerError, "Error loading flow")
		return
	}

	var versions []models.FlowVersion
	if err := h.DB.Where("flow_id = ?", flowID).Order("version_number DESC").Limit(20).Find(&versions).Error; err != nil {
		RespondError(w, http.StatusInternalServerError, "Error loading versions")
		return
	}

	RespondJSON(w, http.StatusOK, versions)
}

// RollbackFlow - POST /api/flows/{id}/rollback
func (h *FlowHandler) RollbackFlow(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	flowID := mux.Vars(r)["id"]

	var req FlowRollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid rollback request")
		return
	}

	if strings.TrimSpace(req.VersionID) == "" {
		RespondError(w, http.StatusBadRequest, "versionId is required")
		return
	}

	var version models.FlowVersion
	if err := h.DB.First(&version, "id = ? AND flow_id = ? AND user_id = ?", req.VersionID, flowID, userId).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			RespondError(w, http.StatusNotFound, "Version not found")
			return
		}
		RespondError(w, http.StatusInternalServerError, "Error loading version")
		return
	}

	var snapshot FlowVersionSnapshot
	if err := json.Unmarshal([]byte(version.Snapshot), &snapshot); err != nil {
		RespondError(w, http.StatusInternalServerError, "Snapshot data is corrupted")
		return
	}

	note := fmt.Sprintf("rollback-to-v%d", version.VersionNumber)
	if err := h.saveFlowSnapshot(flowID, userId, snapshot.Nodes, snapshot.Edges, note, true); err != nil {
		RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	var updated models.Flow
	if err := h.DB.Preload("Nodes").Preload("Edges").First(&updated, "id = ? AND user_id = ?", flowID, userId).Error; err != nil {
		RespondError(w, http.StatusInternalServerError, "Error loading rolled back flow")
		return
	}

	RespondJSON(w, http.StatusOK, updated)
}

// CreateOrGetFlowShare - POST /api/flows/{id}/share
func (h *FlowHandler) CreateOrGetFlowShare(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	flowID := mux.Vars(r)["id"]

	var flow models.Flow
	if err := h.DB.First(&flow, "id = ? AND user_id = ?", flowID, userId).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			RespondError(w, http.StatusNotFound, "Flow not found")
			return
		}
		RespondError(w, http.StatusInternalServerError, "Error loading flow")
		return
	}

	var share models.FlowShare
	if err := h.DB.First(&share, "flow_id = ? AND user_id = ?", flowID, userId).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			RespondError(w, http.StatusInternalServerError, "Error loading flow share")
			return
		}

		now := time.Now()
		share = models.FlowShare{
			ID:        uuid.New().String(),
			FlowID:    flowID,
			UserID:    userId,
			ShareID:   uuid.NewString(),
			IsActive:  true,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := h.DB.Create(&share).Error; err != nil {
			RespondError(w, http.StatusInternalServerError, "Error creating share link")
			return
		}
	}

	if !share.IsActive {
		share.IsActive = true
		share.UpdatedAt = time.Now()
		if err := h.DB.Save(&share).Error; err != nil {
			RespondError(w, http.StatusInternalServerError, "Error reactivating share link")
			return
		}
	}

	RespondJSON(w, http.StatusOK, FlowShareResponse{
		ShareID:      share.ShareID,
		ShareURL:     buildPublicShareURL(r, share.ShareID),
		IsActive:     share.IsActive,
		ExpiresAt:    share.ExpiresAt,
		LastAccessAt: share.LastAccessAt,
		CreatedAt:    share.CreatedAt,
		UpdatedAt:    share.UpdatedAt,
	})
}

// GetPublicSharedFlow - GET /api/public/flows/{shareId}
func (h *FlowHandler) GetPublicSharedFlow(w http.ResponseWriter, r *http.Request) {
	shareID := mux.Vars(r)["shareId"]

	var share models.FlowShare
	if err := h.DB.First(&share, "share_id = ?", shareID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			RespondError(w, http.StatusNotFound, "Shared flow not found")
			return
		}
		RespondError(w, http.StatusInternalServerError, "Error loading share")
		return
	}

	if !share.IsActive {
		RespondError(w, http.StatusForbidden, "Shared flow is disabled")
		return
	}

	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		RespondError(w, http.StatusForbidden, "Shared flow link has expired")
		return
	}

	now := time.Now()
	share.LastAccessAt = &now
	share.UpdatedAt = now
	_ = h.DB.Model(&models.FlowShare{}).Where("id = ?", share.ID).Updates(map[string]interface{}{
		"last_access_at": share.LastAccessAt,
		"updated_at":     share.UpdatedAt,
	}).Error

	var flow models.Flow
	if err := h.DB.Preload("Nodes").Preload("Edges").First(&flow, "id = ?", share.FlowID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			RespondError(w, http.StatusNotFound, "Shared flow not found")
			return
		}
		RespondError(w, http.StatusInternalServerError, "Error loading flow")
		return
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"shareId": share.ShareID,
		"flow":    flow,
	})
}

// UpdateFlowShare - PUT /api/flows/{id}/share
func (h *FlowHandler) UpdateFlowShare(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	flowID := mux.Vars(r)["id"]

	var req FlowShareUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid share update request")
		return
	}

	var share models.FlowShare
	if err := h.DB.First(&share, "flow_id = ? AND user_id = ?", flowID, userId).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			RespondError(w, http.StatusNotFound, "Share link not found")
			return
		}
		RespondError(w, http.StatusInternalServerError, "Error loading flow share")
		return
	}

	if req.IsActive != nil {
		share.IsActive = *req.IsActive
	}

	if strings.TrimSpace(req.ExpiresAt) == "" {
		share.ExpiresAt = nil
	} else {
		expiresAt, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			RespondError(w, http.StatusBadRequest, "expiresAt must be a valid RFC3339 datetime")
			return
		}
		share.ExpiresAt = &expiresAt
	}

	share.UpdatedAt = time.Now()
	if err := h.DB.Save(&share).Error; err != nil {
		RespondError(w, http.StatusInternalServerError, "Error updating share link")
		return
	}

	RespondJSON(w, http.StatusOK, FlowShareResponse{
		ShareID:      share.ShareID,
		ShareURL:     buildPublicShareURL(r, share.ShareID),
		IsActive:     share.IsActive,
		ExpiresAt:    share.ExpiresAt,
		LastAccessAt: share.LastAccessAt,
		CreatedAt:    share.CreatedAt,
		UpdatedAt:    share.UpdatedAt,
	})
}

// RegenerateFlowShare - POST /api/flows/{id}/share/regenerate
func (h *FlowHandler) RegenerateFlowShare(w http.ResponseWriter, r *http.Request) {
	userId, _ := r.Context().Value("userId").(string)
	flowID := mux.Vars(r)["id"]

	var share models.FlowShare
	if err := h.DB.First(&share, "flow_id = ? AND user_id = ?", flowID, userId).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			RespondError(w, http.StatusNotFound, "Share link not found")
			return
		}
		RespondError(w, http.StatusInternalServerError, "Error loading flow share")
		return
	}

	share.ShareID = uuid.NewString()
	share.IsActive = true
	share.UpdatedAt = time.Now()

	if err := h.DB.Save(&share).Error; err != nil {
		RespondError(w, http.StatusInternalServerError, "Error regenerating share link")
		return
	}

	RespondJSON(w, http.StatusOK, FlowShareResponse{
		ShareID:      share.ShareID,
		ShareURL:     buildPublicShareURL(r, share.ShareID),
		IsActive:     share.IsActive,
		ExpiresAt:    share.ExpiresAt,
		LastAccessAt: share.LastAccessAt,
		CreatedAt:    share.CreatedAt,
		UpdatedAt:    share.UpdatedAt,
	})
}

func buildPublicShareURL(r *http.Request, shareID string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/api/public/flows/%s", scheme, r.Host, shareID)
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
