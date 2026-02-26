package services

import (
	"encoding/json"
	"math"
	"time"

	"capyflow/api/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AnalyticsService struct {
	db *gorm.DB
}

func NewAnalyticsService(db *gorm.DB) *AnalyticsService {
	return &AnalyticsService{db: db}
}

// CreateFlowAnalytics initializes analytics for a new flow
func (s *AnalyticsService) CreateFlowAnalytics(flowID string, creationMethod models.CreationMethod) (*models.FlowAnalytics, error) {
	analytics := &models.FlowAnalytics{
		ID:             uuid.New().String(),
		FlowID:         flowID,
		CreationMethod: creationMethod,
	}

	if err := s.db.Create(analytics).Error; err != nil {
		return nil, err
	}

	return analytics, nil
}

// StartCreationSession begins tracking a workflow creation/edit session
func (s *AnalyticsService) StartCreationSession(flowID, userID string, creationMethod models.CreationMethod) (*models.WorkflowCreationSession, error) {
	// End any active sessions for this flow
	s.db.Model(&models.WorkflowCreationSession{}).
		Where("flow_id = ? AND is_active = ?", flowID, true).
		Updates(map[string]interface{}{
			"is_active":        false,
			"ended_at":         time.Now(),
			"duration_seconds": gorm.Expr("EXTRACT(EPOCH FROM (? - started_at))", time.Now()),
		})

	session := &models.WorkflowCreationSession{
		ID:             uuid.New().String(),
		FlowID:         flowID,
		UserID:         userID,
		CreationMethod: creationMethod,
		StartedAt:      time.Now(),
		IsActive:       true,
	}

	if err := s.db.Create(session).Error; err != nil {
		return nil, err
	}

	return session, nil
}

// EndCreationSession finalizes a creation session
func (s *AnalyticsService) EndCreationSession(sessionID string) error {
	now := time.Now()
	return s.db.Model(&models.WorkflowCreationSession{}).
		Where("id = ?", sessionID).
		Updates(map[string]interface{}{
			"is_active":        false,
			"ended_at":         now,
			"duration_seconds": gorm.Expr("EXTRACT(EPOCH FROM (? - started_at))", now),
		}).Error
}

// UpdateSessionActivity tracks node/edge changes
func (s *AnalyticsService) UpdateSessionActivity(sessionID string, nodeAdded, nodeDeleted, edgeAdded, edgeDeleted int) error {
	updates := make(map[string]interface{})

	if nodeAdded > 0 {
		updates["node_additions"] = gorm.Expr("node_additions + ?", nodeAdded)
	}
	if nodeDeleted > 0 {
		updates["node_deletions"] = gorm.Expr("node_deletions + ?", nodeDeleted)
	}
	if edgeAdded > 0 {
		updates["edge_additions"] = gorm.Expr("edge_additions + ?", edgeAdded)
	}
	if edgeDeleted > 0 {
		updates["edge_deletions"] = gorm.Expr("edge_deletions + ?", edgeDeleted)
	}

	if len(updates) == 0 {
		return nil
	}

	return s.db.Model(&models.WorkflowCreationSession{}).
		Where("id = ?", sessionID).
		Updates(updates).Error
}

// IncrementSaveCount tracks saves during session
func (s *AnalyticsService) IncrementSaveCount(sessionID string) error {
	return s.db.Model(&models.WorkflowCreationSession{}).
		Where("id = ?", sessionID).
		Update("save_count", gorm.Expr("save_count + 1")).Error
}

// LogAIInteraction records an AI generation/repair event
func (s *AnalyticsService) LogAIInteraction(flowID, userID, sessionID, interactionType, prompt string, success bool, nodesGen, edgesGen, processingTimeMs int, errorMsg string) error {
	interaction := &models.AIInteraction{
		ID:               uuid.New().String(),
		FlowID:           flowID,
		UserID:           userID,
		SessionID:        sessionID,
		InteractionType:  interactionType,
		Prompt:           prompt,
		ResponseSuccess:  success,
		NodesGenerated:   nodesGen,
		EdgesGenerated:   edgesGen,
		ProcessingTimeMs: processingTimeMs,
		ErrorMessage:     errorMsg,
	}

	if err := s.db.Create(interaction).Error; err != nil {
		return err
	}

	// Update flow analytics AI counters
	if interactionType == "generate" {
		s.db.Model(&models.FlowAnalytics{}).
			Where("flow_id = ?", flowID).
			Update("ai_prompts_used", gorm.Expr("ai_prompts_used + 1"))
	} else if interactionType == "repair" {
		s.db.Model(&models.FlowAnalytics{}).
			Where("flow_id = ?", flowID).
			Update("ai_repairs_used", gorm.Expr("ai_repairs_used + 1"))
	}

	return nil
}

// CalculateComplexityScore computes workflow complexity based on structure
func (s *AnalyticsService) CalculateComplexityScore(flowID string) (float64, error) {
	var flow models.Flow
	if err := s.db.Preload("Nodes").Preload("Edges").First(&flow, "id = ?", flowID).Error; err != nil {
		return 0, err
	}

	nodeCount := len(flow.Nodes)
	edgeCount := len(flow.Edges)

	// Count unique categories
	categoryMap := make(map[string]bool)
	for _, node := range flow.Nodes {
		categoryMap[node.Category] = true
	}
	categoryCount := len(categoryMap)

	// Calculate max depth (simplified - assumes linear flow)
	maxDepth := s.calculateMaxDepth(flow.Nodes, flow.Edges)

	// Complexity formula: (nodes * 1.5) + (edges * 1.2) + (categories * 2) + (depth * 1.3)
	complexity := float64(nodeCount)*1.5 + float64(edgeCount)*1.2 + float64(categoryCount)*2.0 + float64(maxDepth)*1.3

	// Update analytics
	categoriesJSON, _ := json.Marshal(getCategoryList(categoryMap))
	s.db.Model(&models.FlowAnalytics{}).
		Where("flow_id = ?", flowID).
		Updates(map[string]interface{}{
			"complexity_score": complexity,
			"node_count":       nodeCount,
			"edge_count":       edgeCount,
			"max_depth":        maxDepth,
			"categories_used":  string(categoriesJSON),
		})

	return complexity, nil
}

// calculateMaxDepth finds the longest path in the workflow graph
func (s *AnalyticsService) calculateMaxDepth(nodes []models.Node, edges []models.Edge) int {
	if len(nodes) == 0 {
		return 0
	}

	// Build adjacency list
	graph := make(map[string][]string)
	inDegree := make(map[string]int)

	for _, node := range nodes {
		graph[node.ID] = []string{}
		inDegree[node.ID] = 0
	}

	for _, edge := range edges {
		graph[edge.Source] = append(graph[edge.Source], edge.Target)
		inDegree[edge.Target]++
	}

	// Find start nodes (no incoming edges)
	var startNodes []string
	for nodeID, degree := range inDegree {
		if degree == 0 {
			startNodes = append(startNodes, nodeID)
		}
	}

	if len(startNodes) == 0 {
		return 1 // Circular or single node
	}

	// DFS to find max depth
	maxDepth := 0
	visited := make(map[string]bool)

	var dfs func(string, int)
	dfs = func(nodeID string, depth int) {
		visited[nodeID] = true
		if depth > maxDepth {
			maxDepth = depth
		}
		for _, neighbor := range graph[nodeID] {
			dfs(neighbor, depth+1)
		}
	}

	for _, startNode := range startNodes {
		dfs(startNode, 1)
	}

	return maxDepth
}

func getCategoryList(categoryMap map[string]bool) []string {
	categories := make([]string, 0, len(categoryMap))
	for cat := range categoryMap {
		categories = append(categories, cat)
	}
	return categories
}

// UpdateExecutionStats updates execution metrics for a flow
func (s *AnalyticsService) UpdateExecutionStats(flowID string, success bool, executionTimeMs int) error {
	updates := map[string]interface{}{
		"total_executions": gorm.Expr("total_executions + 1"),
	}

	if success {
		updates["successful_executions"] = gorm.Expr("successful_executions + 1")
	} else {
		updates["failed_executions"] = gorm.Expr("failed_executions + 1")
	}

	// Update average execution time
	s.db.Model(&models.FlowAnalytics{}).
		Where("flow_id = ?", flowID).
		Update("avg_execution_time_ms", gorm.Expr(
			"((avg_execution_time_ms * (total_executions - 1)) + ?) / total_executions",
			executionTimeMs,
		))

	return s.db.Model(&models.FlowAnalytics{}).
		Where("flow_id = ?", flowID).
		Updates(updates).Error
}

// GetAnalyticsSummary generates aggregated analytics for a user
func (s *AnalyticsService) GetAnalyticsSummary(userID string) (*models.AnalyticsSummary, error) {
	var summary models.AnalyticsSummary

	// Count total flows
	s.db.Model(&models.Flow{}).Where("user_id = ?", userID).Count(&[]int64{int64(summary.TotalFlows)}[0])

	// Get AI vs Manual stats
	var aiFlows, manualFlows []models.FlowAnalytics
	s.db.Joins("JOIN flows ON flows.id = flow_analytics.flow_id").
		Where("flows.user_id = ? AND flow_analytics.creation_method = ?", userID, models.CreationMethodAI).
		Find(&aiFlows)

	s.db.Joins("JOIN flows ON flows.id = flow_analytics.flow_id").
		Where("flows.user_id = ? AND flow_analytics.creation_method = ?", userID, models.CreationMethodManual).
		Find(&manualFlows)

	summary.AIGeneratedFlows = len(aiFlows)
	summary.ManualFlows = len(manualFlows)

	// Calculate averages
	if len(aiFlows) > 0 {
		var totalTimeAI int64
		var totalComplexityAI float64
		for _, flow := range aiFlows {
			totalTimeAI += int64(flow.CreationTimeSeconds)
			totalComplexityAI += flow.ComplexityScore
		}
		summary.AvgCreationTimeAI = float64(totalTimeAI) / float64(len(aiFlows))
		summary.AvgComplexityAI = totalComplexityAI / float64(len(aiFlows))
	}

	if len(manualFlows) > 0 {
		var totalTimeManual int64
		var totalComplexityManual float64
		for _, flow := range manualFlows {
			totalTimeManual += int64(flow.CreationTimeSeconds)
			totalComplexityManual += flow.ComplexityScore
		}
		summary.AvgCreationTimeManual = float64(totalTimeManual) / float64(len(manualFlows))
		summary.AvgComplexityManual = totalComplexityManual / float64(len(manualFlows))
	}

	// Calculate time saved percentage
	if summary.AvgCreationTimeManual > 0 && summary.AvgCreationTimeAI > 0 {
		summary.TimeSavedPercentage = ((summary.AvgCreationTimeManual - summary.AvgCreationTimeAI) / summary.AvgCreationTimeManual) * 100
	}

	// AI Interactions
	var totalInteractions int64
	s.db.Model(&models.AIInteraction{}).Where("user_id = ?", userID).Count(&totalInteractions)
	summary.TotalAIInteractions = int(totalInteractions)

	var successfulInteractions int64
	s.db.Model(&models.AIInteraction{}).Where("user_id = ? AND response_success = ?", userID, true).Count(&successfulInteractions)
	summary.SuccessfulAIInteractions = int(successfulInteractions)

	// Execution stats
	var allAnalytics []models.FlowAnalytics
	s.db.Joins("JOIN flows ON flows.id = flow_analytics.flow_id").
		Where("flows.user_id = ?", userID).
		Find(&allAnalytics)

	for _, analytics := range allAnalytics {
		summary.TotalExecutions += analytics.TotalExecutions
		summary.SuccessfulExecutions += analytics.SuccessfulExecutions
		summary.FailedExecutions += analytics.FailedExecutions
	}

	if summary.TotalExecutions > 0 {
		summary.SuccessRate = (float64(summary.SuccessfulExecutions) / float64(summary.TotalExecutions)) * 100
		summary.SuccessRate = math.Round(summary.SuccessRate*100) / 100 // Round to 2 decimals
	}

	return &summary, nil
}

// GetFlowAnalytics retrieves analytics for a specific flow
func (s *AnalyticsService) GetFlowAnalytics(flowID string) (*models.FlowAnalytics, error) {
	var analytics models.FlowAnalytics
	if err := s.db.Where("flow_id = ?", flowID).First(&analytics).Error; err != nil {
		return nil, err
	}
	return &analytics, nil
}

// GetCreationSessions retrieves all creation sessions for a flow
func (s *AnalyticsService) GetCreationSessions(flowID string) ([]models.WorkflowCreationSession, error) {
	var sessions []models.WorkflowCreationSession
	if err := s.db.Where("flow_id = ?", flowID).Order("started_at DESC").Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

// GetAIInteractions retrieves AI interactions for a flow
func (s *AnalyticsService) GetAIInteractions(flowID string) ([]models.AIInteraction, error) {
	var interactions []models.AIInteraction
	if err := s.db.Where("flow_id = ?", flowID).Order("created_at DESC").Find(&interactions).Error; err != nil {
		return nil, err
	}
	return interactions, nil
}

// UpdateCreationTime updates total creation time from all sessions
func (s *AnalyticsService) UpdateCreationTime(flowID string) error {
	var totalSeconds int64
	s.db.Model(&models.WorkflowCreationSession{}).
		Where("flow_id = ?", flowID).
		Select("COALESCE(SUM(duration_seconds), 0)").
		Scan(&totalSeconds)

	return s.db.Model(&models.FlowAnalytics{}).
		Where("flow_id = ?", flowID).
		Update("creation_time_seconds", totalSeconds).Error
}
