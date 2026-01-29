package database

import (
	"capyflow/api/models"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func SeedNodeTypes(db *gorm.DB) {
	nodeTypes := []models.NodeType{

		// ===== TRIGGERS =====
		{
			ID:             uuid.New().String(),
			Name:           "Manual Trigger",
			Type:           "manual-trigger",
			Category:       models.CategoryTrigger,
			Description:    "Inicia el workflow manualmente",
			Icon:           "IconPlayerPlay",
			Color:          "#10B981",
			Version:        "1.0.0",
			DefaultInputs:  "[]",
			DefaultOutputs: `[{"id":"output","name":"Output","type":"any"}]`,
			DefaultParams:  "[]",
			IsActive:       true,
			IsBeta:         false,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New().String(),
			Name:           "Webhook Trigger",
			Type:           "webhook-trigger",
			Category:       models.CategoryTrigger,
			Description:    "Se activa con peticiones HTTP externas",
			Icon:           "IconWebhook",
			Color:          "#10B981",
			Version:        "1.0.0",
			DefaultInputs:  "[]",
			DefaultOutputs: `[{"id":"body","name":"Body","type":"object"},{"id":"headers","name":"Headers","type":"object"}]`,
			DefaultParams:  `[{"id":"method","name":"Method","type":"select","options":["GET","POST","PUT","DELETE"],"value":"POST"}]`,
			IsActive:       true,
			IsBeta:         false,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},

		// ===== DATA =====
		{
			ID:             uuid.New().String(),
			Name:           "Set Data",
			Type:           "set-data",
			Category:       models.CategoryData,
			Description:    "Define o modifica valores dentro del flujo",
			Icon:           "IconEdit",
			Color:          "#3B82F6",
			Version:        "1.0.0",
			DefaultInputs:  `[{"id":"input","name":"Input","type":"any"}]`,
			DefaultOutputs: `[{"id":"output","name":"Output","type":"any"}]`,
			DefaultParams:  `[{"id":"values","name":"Values","type":"object","required":true}]`,
			IsActive:       true,
			IsBeta:         false,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New().String(),
			Name:           "Transform Data",
			Type:           "transform-data",
			Category:       models.CategoryData,
			Description:    "Transforma estructuras de datos",
			Icon:           "IconTransform",
			Color:          "#3B82F6",
			Version:        "1.0.0",
			DefaultInputs:  `[{"id":"data","name":"Data","type":"any","required":true}]`,
			DefaultOutputs: `[{"id":"result","name":"Result","type":"any"}]`,
			DefaultParams:  `[{"id":"operation","name":"Operation","type":"select","options":["map","filter"],"value":"map"}]`,
			IsActive:       true,
			IsBeta:         false,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New().String(),
			Name:           "JSON Parser",
			Type:           "json-parser",
			Category:       models.CategoryData,
			Description:    "Parsea strings JSON a objetos",
			Icon:           "IconBraces",
			Color:          "#3B82F6",
			Version:        "1.0.0",
			DefaultInputs:  `[{"id":"json","name":"JSON","type":"string","required":true}]`,
			DefaultOutputs: `[{"id":"parsed","name":"Parsed","type":"object"}]`,
			DefaultParams:  "[]",
			IsActive:       true,
			IsBeta:         false,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},

		// ===== LOGIC =====
		{
			ID:             uuid.New().String(),
			Name:           "If Condition",
			Type:           "if-condition",
			Category:       models.CategoryLogic,
			Description:    "Evalúa una condición y bifurca el flujo",
			Icon:           "IconGitBranch",
			Color:          "#F59E0B",
			Version:        "1.0.0",
			DefaultInputs:  `[{"id":"input","name":"Input","type":"any","required":true}]`,
			DefaultOutputs: `[{"id":"true","name":"True","type":"any"},{"id":"false","name":"False","type":"any"}]`,
			DefaultParams:  `[{"id":"condition","name":"Condition","type":"string","required":true}]`,
			IsActive:       true,
			IsBeta:         false,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},

		// ===== IO =====
		{
			ID:             uuid.New().String(),
			Name:           "HTTP Request",
			Type:           "http-request",
			Category:       models.CategoryIO,
			Description:    "Realiza peticiones HTTP a APIs externas",
			Icon:           "IconWorld",
			Color:          "#EC4899",
			Version:        "1.0.0",
			DefaultInputs:  `[{"id":"url","name":"URL","type":"string","required":true},{"id":"body","name":"Body","type":"object"}]`,
			DefaultOutputs: `[{"id":"response","name":"Response","type":"object"},{"id":"status","name":"Status Code","type":"number"}]`,
			DefaultParams:  `[{"id":"method","name":"Method","type":"select","options":["GET","POST","PUT","DELETE"],"value":"GET"}]`,
			IsActive:       true,
			IsBeta:         false,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}

	for _, nt := range nodeTypes {
		var existing models.NodeType
		if err := db.First(&existing, "type = ?", nt.Type).Error; err != nil {
			db.Create(&nt)
		}
	}
}
