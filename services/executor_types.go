package services

import "capyflow/api/models"

type ExecutionResult struct {
	NodeID     string                 `json:"nodeId"`
	Status     models.NodeStatus      `json:"status"`
	Output     map[string]interface{} `json:"output"`
	Error      string                 `json:"error,omitempty"`
	DurationMs int64                  `json:"durationMs"`
}

type FlowExecutionResult struct {
	Status        string                      `json:"status"`
	ExecutedNodes []string                    `json:"executedNodes"`
	Results       map[string]*ExecutionResult `json:"results"`
	DurationMs    int64                       `json:"durationMs"`
	ErrorMessage  string                      `json:"errorMessage,omitempty"`
	UserID        string                      `json:"-"`
	ExecutionID   string                      `json:"-"`
}
