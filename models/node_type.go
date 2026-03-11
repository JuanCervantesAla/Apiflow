package models

import "time"

type NodeCategory string

const (
	CategoryTrigger     NodeCategory = "trigger"
	CategoryAI          NodeCategory = "ai"
	CategoryData        NodeCategory = "data"
	CategoryLogic       NodeCategory = "logic"
	CategoryIO          NodeCategory = "io"
	CategoryIntegration NodeCategory = "integration"
	CategoryControl     NodeCategory = "control"
)

type NodeType struct {
	ID          string       `json:"id" gorm:"primaryKey"`
	Name        string       `json:"name" gorm:"not null"`
	Type        string       `json:"type" gorm:"uniqueIndex;not null"` // Identificador único
	Category    NodeCategory `json:"category" gorm:"not null"`
	Description string       `json:"description"`
	Icon        string       `json:"icon"`
	Color       string       `json:"color"`
	Version     string       `json:"version" gorm:"default:1.0.0"`

	// Configuración por defecto
	DefaultInputs  string `json:"defaultInputs" gorm:"type:text"`  // JSON
	DefaultOutputs string `json:"defaultOutputs" gorm:"type:text"` // JSON
	DefaultParams  string `json:"defaultParams" gorm:"type:text"`  // JSON

	// Metadata
	IsActive  bool      `json:"isActive" gorm:"default:true"`
	IsBeta    bool      `json:"isBeta" gorm:"default:false"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (NodeType) TableName() string {
	return "node_types"
}
