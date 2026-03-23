package models

import "time"

// FlowShare stores public read-only links for flows.
type FlowShare struct {
	ID           string     `json:"id" gorm:"primaryKey"`
	FlowID       string     `json:"flowId" gorm:"index;not null"`
	UserID       string     `json:"userId" gorm:"index;not null"`
	ShareID      string     `json:"shareId" gorm:"uniqueIndex;not null"`
	IsActive     bool       `json:"isActive" gorm:"default:true"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`
	LastAccessAt *time.Time `json:"lastAccessAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

func (FlowShare) TableName() string {
	return "flow_shares"
}
