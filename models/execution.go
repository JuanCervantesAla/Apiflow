package models

import (
	"time"
)

type ExecutionStatus string

const (
	ExecutionStatusRunning  ExecutionStatus = "running"
	ExecutionStatusSuccess  ExecutionStatus = "success"
	ExecutionStatusPartial  ExecutionStatus = "partial"
	ExecutionStatusError    ExecutionStatus = "error"
	ExecutionStatusCanceled ExecutionStatus = "canceled"
)

type Execution struct {
	ID          string          `json:"id" gorm:"primaryKey"`
	FlowID      string          `json:"flowId" gorm:"index;not null"`
	UserID      string          `json:"userId" gorm:"index;not null"`
	Status      ExecutionStatus `json:"status" gorm:"not null"`
	StartedAt   time.Time       `json:"startedAt" gorm:"not null"`
	FinishedAt  *time.Time      `json:"finishedAt,omitempty"`
	DurationMs  int64           `json:"durationMs"`
	TriggerType string          `json:"triggerType"` // manual, webhook, scheduled, etc.

	Results       string `json:"results" gorm:"type:text"`       // JSON: map[nodeId]ExecutionResult
	ExecutedNodes string `json:"executedNodes" gorm:"type:text"` // JSON: []string

	ErrorMessage string `json:"errorMessage,omitempty" gorm:"type:text"`
	Logs         string `json:"logs,omitempty" gorm:"type:text"` // JSON: []LogEntry

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (Execution) TableName() string {
	return "executions"
}

type ExecutionListItem struct {
	ID          string          `json:"id"`
	FlowID      string          `json:"flowId"`
	FlowName    string          `json:"flowName"`
	Status      ExecutionStatus `json:"status"`
	StartedAt   time.Time       `json:"startedAt"`
	FinishedAt  *time.Time      `json:"finishedAt,omitempty"`
	DurationMs  int64           `json:"durationMs"`
	TriggerType string          `json:"triggerType"`
}
