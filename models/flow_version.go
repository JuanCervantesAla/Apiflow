package models

import "time"

// FlowVersion stores immutable snapshots of a flow graph for rollback/version history.
type FlowVersion struct {
	ID            string    `json:"id" gorm:"primaryKey"`
	FlowID        string    `json:"flowId" gorm:"index;not null"`
	UserID        string    `json:"userId" gorm:"index;not null"`
	VersionNumber int       `json:"versionNumber" gorm:"not null"`
	Note          string    `json:"note,omitempty"`
	Snapshot      string    `json:"snapshot" gorm:"type:text;not null"`
	CreatedAt     time.Time `json:"createdAt"`
}

func (FlowVersion) TableName() string {
	return "flow_versions"
}
