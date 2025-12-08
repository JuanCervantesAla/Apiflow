package database

import (
	"capyflow/api/models"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func SeedAdminUser(db *gorm.DB) {
	email := "admin@capyflow.com"
	password := "123456"
	name := "Admin User"

	var user models.User
	if err := db.First(&user, "email = ?", email).Error; err == nil {
		// Ya existe, no hacer nada
		return
	}

	hashed, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	user = models.User{
		ID:        uuid.New().String(),
		Name:      name,
		Email:     email,
		Password:  string(hashed),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	db.Create(&user)
}
