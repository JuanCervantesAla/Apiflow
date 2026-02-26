package handlers

import (
	"capyflow/api/models"
	"capyflow/api/services"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

type AnalyticsHandler struct {
	analyticsService *services.AnalyticsService
}

func NewAnalyticsHandler(analyticsService *services.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{
		analyticsService: analyticsService,
	}
}

// GetAnalyticsSummary returns aggregated analytics for the authenticated user
// GET /api/analytics/summary
func (h *AnalyticsHandler) GetAnalyticsSummary(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("userId").(string)
	if !ok {
		RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	summary, err := h.analyticsService.GetAnalyticsSummary(userID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to retrieve analytics summary")
		return
	}

	RespondJSON(w, http.StatusOK, summary)
}

// GetFlowAnalytics returns analytics for a specific flow
// GET /api/analytics/flows/:id
func (h *AnalyticsHandler) GetFlowAnalytics(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flowID := vars["id"]

	analytics, err := h.analyticsService.GetFlowAnalytics(flowID)
	if err != nil {
		RespondError(w, http.StatusNotFound, "Flow analytics not found")
		return
	}

	RespondJSON(w, http.StatusOK, analytics)
}

// GetFlowSessions returns all creation sessions for a flow
// GET /api/analytics/flows/:id/sessions
func (h *AnalyticsHandler) GetFlowSessions(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flowID := vars["id"]

	sessions, err := h.analyticsService.GetCreationSessions(flowID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to retrieve sessions")
		return
	}

	RespondJSON(w, http.StatusOK, sessions)
}

// GetFlowAIInteractions returns AI interactions for a flow
// GET /api/analytics/flows/:id/ai-interactions
func (h *AnalyticsHandler) GetFlowAIInteractions(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flowID := vars["id"]

	interactions, err := h.analyticsService.GetAIInteractions(flowID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to retrieve AI interactions")
		return
	}

	RespondJSON(w, http.StatusOK, interactions)
}

// StartSession starts a new creation session for a flow
// POST /api/analytics/flows/:id/sessions/start
func (h *AnalyticsHandler) StartSession(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("userId").(string)
	if !ok {
		RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	flowID := vars["id"]

	var req struct {
		CreationMethod models.CreationMethod `json:"creationMethod"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	session, err := h.analyticsService.StartCreationSession(flowID, userID, req.CreationMethod)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to start session")
		return
	}

	RespondJSON(w, http.StatusCreated, session)
}

// EndSession ends an active creation session
// POST /api/analytics/sessions/:sessionId/end
func (h *AnalyticsHandler) EndSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	if err := h.analyticsService.EndCreationSession(sessionID); err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to end session")
		return
	}

	RespondJSON(w, http.StatusOK, map[string]string{"message": "Session ended successfully"})
}

// UpdateSessionActivity tracks node/edge changes during a session
// POST /api/analytics/sessions/:sessionId/activity
func (h *AnalyticsHandler) UpdateSessionActivity(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	var req struct {
		NodeAdded   int `json:"nodeAdded"`
		NodeDeleted int `json:"nodeDeleted"`
		EdgeAdded   int `json:"edgeAdded"`
		EdgeDeleted int `json:"edgeDeleted"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.analyticsService.UpdateSessionActivity(sessionID, req.NodeAdded, req.NodeDeleted, req.EdgeAdded, req.EdgeDeleted); err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to update activity")
		return
	}

	RespondJSON(w, http.StatusOK, map[string]string{"message": "Activity updated"})
}

// IncrementSave tracks saves during a session
// POST /api/analytics/sessions/:sessionId/save
func (h *AnalyticsHandler) IncrementSave(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	if err := h.analyticsService.IncrementSaveCount(sessionID); err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to increment save count")
		return
	}

	RespondJSON(w, http.StatusOK, map[string]string{"message": "Save count incremented"})
}

// LogAIInteraction records an AI generation/repair event
// POST /api/analytics/flows/:id/ai-interaction
func (h *AnalyticsHandler) LogAIInteraction(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("userId").(string)
	if !ok {
		RespondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	flowID := vars["id"]

	var req struct {
		SessionID        string `json:"sessionId"`
		InteractionType  string `json:"interactionType"`
		Prompt           string `json:"prompt"`
		Success          bool   `json:"success"`
		NodesGenerated   int    `json:"nodesGenerated"`
		EdgesGenerated   int    `json:"edgesGenerated"`
		ProcessingTimeMs int    `json:"processingTimeMs"`
		ErrorMessage     string `json:"errorMessage"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.analyticsService.LogAIInteraction(
		flowID,
		userID,
		req.SessionID,
		req.InteractionType,
		req.Prompt,
		req.Success,
		req.NodesGenerated,
		req.EdgesGenerated,
		req.ProcessingTimeMs,
		req.ErrorMessage,
	); err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to log interaction")
		return
	}

	RespondJSON(w, http.StatusCreated, map[string]string{"message": "AI interaction logged"})
}

// CalculateComplexity calculates and updates complexity score for a flow
// POST /api/analytics/flows/:id/complexity
func (h *AnalyticsHandler) CalculateComplexity(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flowID := vars["id"]

	score, err := h.analyticsService.CalculateComplexityScore(flowID)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to calculate complexity")
		return
	}

	RespondJSON(w, http.StatusOK, map[string]float64{"complexityScore": score})
}

// UpdateExecutionStats updates execution metrics (called after flow execution)
// POST /api/analytics/flows/:id/execution
func (h *AnalyticsHandler) UpdateExecutionStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flowID := vars["id"]

	var req struct {
		Success         bool `json:"success"`
		ExecutionTimeMs int  `json:"executionTimeMs"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.analyticsService.UpdateExecutionStats(flowID, req.Success, req.ExecutionTimeMs); err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to update execution stats")
		return
	}

	RespondJSON(w, http.StatusOK, map[string]string{"message": "Execution stats updated"})
}

// UpdateCreationTime recalculates total creation time from sessions
// POST /api/analytics/flows/:id/update-time
func (h *AnalyticsHandler) UpdateCreationTime(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	flowID := vars["id"]

	if err := h.analyticsService.UpdateCreationTime(flowID); err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to update creation time")
		return
	}

	RespondJSON(w, http.StatusOK, map[string]string{"message": "Creation time updated"})
}
