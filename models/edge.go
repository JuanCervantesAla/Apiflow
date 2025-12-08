package models

import "time"

//Basic att for the edge Connects only 2 nodes
type Edge struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	FlowID    string    `json:"flowId" gorm:"index"`
	Source    string    `json:"source"`
	Target    string    `json:"target"`
	Type      string    `json:"type,omitempty"`
	Animated  bool      `json:"animated"`
	Label     string    `json:"label,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (Edge) TableName() string {
	return "edges"
}
