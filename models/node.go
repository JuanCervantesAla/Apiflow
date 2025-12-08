package models

import (
	"time"
)

type NodeStatus string

// Basic status for the Node
const (
	StatusIdle     NodeStatus = "idle"
	StatusRunning  NodeStatus = "running"
	StatusSuccess  NodeStatus = "success"
	StatusError    NodeStatus = "error"
	StatusDisabled NodeStatus = "disabled"
	StatusQueued   NodeStatus = "queued"
)

// Basic att of the Node
type NodeParameter struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Value       interface{} `json:"value"`
	Required    bool        `json:"required,omitempty"`
	Options     []string    `json:"options,omitempty"`
	Placeholder string      `json:"placeholder,omitempty"`
	Description string      `json:"description,omitempty"`
}

// Struct for IO
type NodeIO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Description string `json:"description,omitempty"`
}

// Saves info for the CANVAS
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Node struct {
	ID              string     `json:"id" gorm:"primaryKey"`
	FlowID          string     `json:"flowId" gorm:"index"`
	Type            string     `json:"type"`
	Position        string     `json:"position" gorm:"type:text"` // JSON serializado
	Label           string     `json:"label"`
	Subtitle        string     `json:"subtitle,omitempty"`
	Icon            string     `json:"icon,omitempty"`
	Color           string     `json:"color,omitempty"`
	Category        string     `json:"category,omitempty"`
	Description     string     `json:"description,omitempty"`
	Parameters      string     `json:"parameters,omitempty" gorm:"type:text"` // JSON
	Inputs          string     `json:"inputs,omitempty" gorm:"type:text"`     // JSON
	Outputs         string     `json:"outputs,omitempty" gorm:"type:text"`    // JSON
	Status          NodeStatus `json:"status" gorm:"default:idle"`
	LastRun         *time.Time `json:"lastRun,omitempty"`
	ErrorMessage    string     `json:"errorMessage,omitempty"`
	ExecutionTimeMs int        `json:"executionTimeMs,omitempty"`
	Disabled        bool       `json:"disabled"`
	RetryOnFail     bool       `json:"retryOnFail"`
	Retries         int        `json:"retries"`
	Version         string     `json:"version,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

func (Node) TableName() string {
	return "nodes"
}
