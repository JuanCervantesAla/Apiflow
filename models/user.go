package models

import (
	"time"
)

type User struct {
	ID           string    `json:"id" gorm:"primaryKey"`
	Name         string    `json:"name"`
	Email        string    `json:"email" gorm:"unique"`
	Password     string    `json:"password"`                       //Hash
	GeminiAPIKey string    `json:"-" gorm:"column:gemini_api_key"` // Encrypted API key, hidden from JSON
	Flows        []Flow    `json:"flows" gorm:"foreignKey:UserID"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (User) TableName() string {
	return "users"
}
