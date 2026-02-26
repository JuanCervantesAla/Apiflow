package models

import "time"

// CreationMethod represents how a workflow was created
type CreationMethod string

const (
	CreationMethodManual CreationMethod = "manual"
	CreationMethodAI     CreationMethod = "ai"
)

// FlowAnalytics stores detailed metrics about workflow complexity and usage
type FlowAnalytics struct {
	ID                   string         `json:"id" gorm:"primaryKey"`
	FlowID               string         `json:"flowId" gorm:"index;not null"`
	Flow                 Flow           `json:"flow,omitempty" gorm:"foreignKey:FlowID;constraint:OnDelete:CASCADE"`
	CreationMethod       CreationMethod `json:"creationMethod" gorm:"not null"`
	CreationTimeSeconds  int            `json:"creationTimeSeconds" gorm:"default:0"`
	AIPromptsUsed        int            `json:"aiPromptsUsed" gorm:"default:0"`
	AIRepairsUsed        int            `json:"aiRepairsUsed" gorm:"default:0"`
	ComplexityScore      float64        `json:"complexityScore" gorm:"default:0"`
	NodeCount            int            `json:"nodeCount" gorm:"default:0"`
	EdgeCount            int            `json:"edgeCount" gorm:"default:0"`
	MaxDepth             int            `json:"maxDepth" gorm:"default:0"`
	CategoriesUsed       string         `json:"categoriesUsed" gorm:"type:text"` // JSON array of categories
	SuccessfulExecutions int            `json:"successfulExecutions" gorm:"default:0"`
	FailedExecutions     int            `json:"failedExecutions" gorm:"default:0"`
	AvgExecutionTimeMs   float64        `json:"avgExecutionTimeMs" gorm:"default:0"`
	TotalExecutions      int            `json:"totalExecutions" gorm:"default:0"`
	CreatedAt            time.Time      `json:"createdAt"`
	UpdatedAt            time.Time      `json:"updatedAt"`
}

func (FlowAnalytics) TableName() string {
	return "flow_analytics"
}

// WorkflowCreationSession tracks the time spent creating/editing a workflow
type WorkflowCreationSession struct {
	ID              string         `json:"id" gorm:"primaryKey"`
	FlowID          string         `json:"flowId" gorm:"index;not null"`
	UserID          string         `json:"userId" gorm:"index;not null"`
	CreationMethod  CreationMethod `json:"creationMethod" gorm:"not null"`
	StartedAt       time.Time      `json:"startedAt" gorm:"not null"`
	EndedAt         *time.Time     `json:"endedAt,omitempty"`
	DurationSeconds int            `json:"durationSeconds" gorm:"default:0"`
	NodeAdditions   int            `json:"nodeAdditions" gorm:"default:0"`
	NodeDeletions   int            `json:"nodeDeletions" gorm:"default:0"`
	EdgeAdditions   int            `json:"edgeAdditions" gorm:"default:0"`
	EdgeDeletions   int            `json:"edgeDeletions" gorm:"default:0"`
	SaveCount       int            `json:"saveCount" gorm:"default:0"`
	IsActive        bool           `json:"isActive" gorm:"default:true"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

func (WorkflowCreationSession) TableName() string {
	return "workflow_creation_sessions"
}

// AIInteraction logs each interaction with AI (generation, repair)
type AIInteraction struct {
	ID               string    `json:"id" gorm:"primaryKey"`
	FlowID           string    `json:"flowId" gorm:"index;not null"`
	UserID           string    `json:"userId" gorm:"index;not null"`
	SessionID        string    `json:"sessionId" gorm:"index"`          // Links to WorkflowCreationSession
	InteractionType  string    `json:"interactionType" gorm:"not null"` // "generate", "repair"
	Prompt           string    `json:"prompt" gorm:"type:text"`
	ResponseSuccess  bool      `json:"responseSuccess" gorm:"default:false"`
	NodesGenerated   int       `json:"nodesGenerated" gorm:"default:0"`
	EdgesGenerated   int       `json:"edgesGenerated" gorm:"default:0"`
	TokensUsed       int       `json:"tokensUsed" gorm:"default:0"`       // For future API tracking
	ProcessingTimeMs int       `json:"processingTimeMs" gorm:"default:0"` // AI response time
	ErrorMessage     string    `json:"errorMessage,omitempty" gorm:"type:text"`
	CreatedAt        time.Time `json:"createdAt"`
}

func (AIInteraction) TableName() string {
	return "ai_interactions"
}

// AnalyticsSummary is a DTO for aggregated analytics data
type AnalyticsSummary struct {
	TotalFlows               int     `json:"totalFlows"`
	AIGeneratedFlows         int     `json:"aiGeneratedFlows"`
	ManualFlows              int     `json:"manualFlows"`
	AvgCreationTimeAI        float64 `json:"avgCreationTimeAI"`     // seconds
	AvgCreationTimeManual    float64 `json:"avgCreationTimeManual"` // seconds
	AvgComplexityAI          float64 `json:"avgComplexityAI"`
	AvgComplexityManual      float64 `json:"avgComplexityManual"`
	TotalAIInteractions      int     `json:"totalAIInteractions"`
	SuccessfulAIInteractions int     `json:"successfulAIInteractions"`
	TimeSavedPercentage      float64 `json:"timeSavedPercentage"` // AI vs Manual
	TotalExecutions          int     `json:"totalExecutions"`
	SuccessfulExecutions     int     `json:"successfulExecutions"`
	FailedExecutions         int     `json:"failedExecutions"`
	SuccessRate              float64 `json:"successRate"` // percentage
}

// FlowComparisonData compares AI vs Manual workflows
type FlowComparisonData struct {
	AIFlows     []FlowAnalytics  `json:"aiFlows"`
	ManualFlows []FlowAnalytics  `json:"manualFlows"`
	Summary     AnalyticsSummary `json:"summary"`
}
