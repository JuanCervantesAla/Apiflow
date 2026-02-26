package models

import "time"

type FlowStatus string

const (
	FlowStatusDraft   FlowStatus = "draft"
	FlowStatusActive  FlowStatus = "active"
	FlowStatusRunning FlowStatus = "running"
	FlowStatusPaused  FlowStatus = "paused"
)

//Flow Attributes, Node, edges.
//TODO: Make an user to have various flows, at least 2
type Flow struct {
	ID          string         `json:"id" gorm:"primaryKey"`
	UserID      string         `json:"userId" gorm:"index"`
	Name        string         `json:"name" gorm:"not null"`
	Description string         `json:"description"`
	Status      FlowStatus     `json:"status" gorm:"default:draft"`
	Nodes       []Node         `json:"nodes" gorm:"foreignKey:FlowID;constraint:OnDelete:CASCADE"`
	Edges       []Edge         `json:"edges" gorm:"foreignKey:FlowID;constraint:OnDelete:CASCADE"`
	Analytics   *FlowAnalytics `json:"analytics,omitempty" gorm:"foreignKey:FlowID;constraint:OnDelete:CASCADE"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

func (Flow) TableName() string {
	return "flows"
}

//Request to create a flow
type FlowCreateRequest struct {
	Name           string         `json:"name" binding:"required"`
	Description    string         `json:"description"`
	CreationMethod CreationMethod `json:"creationMethod,omitempty"` // "manual" or "ai"
}

//FlowUpdateRequest to update a flow
type FlowUpdateRequest struct {
	Name        string     `json:"name,omitempty"`
	Description string     `json:"description,omitempty"`
	Status      FlowStatus `json:"status,omitempty"`
}
