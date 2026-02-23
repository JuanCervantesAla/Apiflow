package handlers

import (
	"capyflow/api/helpers"
	"capyflow/api/models"
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var jwtSecret = []byte("supersecretkey") //TODO: Change in production

type UserHandler struct {
	DB *gorm.DB
}

func NewUserHandler(db *gorm.DB) *UserHandler {
	return &UserHandler{DB: db}
}

// Insert a user
func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Datos inválidos")
		return
	}

	//Hashing the password
	hashed, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	user := models.User{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Email:     req.Email,
		Password:  string(hashed),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.DB.Create(&user).Error; err != nil {
		respondError(w, http.StatusBadRequest, "Email already registered!")
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": user.ID,
		"email":  user.Email,
		"exp":    time.Now().Add(time.Hour * 72).Unix(),
	})

	tokenString, _ := token.SignedString(jwtSecret)
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "User Created!",
		"token":   tokenString,
		"user": map[string]interface{}{
			"id":    user.ID,
			"name":  user.Name,
			"email": user.Email,
		},
	})
}

// LOGIN FUNC
func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusUnauthorized, "User not found")
		return
	}

	var user models.User
	if err := h.DB.First(&user, "email = ?", req.Email).Error; err != nil {
		respondError(w, http.StatusUnauthorized, "Incorrect password")
		return
	}

	//JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userId": user.ID,
		"email":  user.Email,
		"exp":    time.Now().Add(time.Hour * 72).Unix(),
	})

	tokenString, _ := token.SignedString(jwtSecret)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"token": tokenString,
		"user": map[string]interface{}{
			"id":    user.ID,
			"name":  user.Name,
			"email": user.Email,
		},
	})
}

func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userId")
	if userID == nil {
		respondError(w, http.StatusUnauthorized, "Usuario no autenticado")
		return
	}

	var user models.User
	if err := h.DB.First(&user, "id = ?", userID).Error; err != nil {
		respondError(w, http.StatusNotFound, "Usuario no encontrado")
		return
	}

	// Check if user has API key configured
	hasAPIKey := user.GeminiAPIKey != ""

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"id":        user.ID,
		"name":      user.Name,
		"email":     user.Email,
		"hasAPIKey": hasAPIKey,
	})
}

// SaveGeminiAPIKey saves or updates the user's Gemini API key (encrypted)
func (h *UserHandler) SaveGeminiAPIKey(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userId")
	if userID == nil {
		respondError(w, http.StatusUnauthorized, "Usuario no autenticado")
		return
	}

	var req struct {
		APIKey string `json:"apiKey"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Datos inválidos")
		return
	}

	if req.APIKey == "" {
		respondError(w, http.StatusBadRequest, "API key no puede estar vacía")
		return
	}

	// Encrypt the API key
	encrypted, err := helpers.EncryptString(req.APIKey)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Error al encriptar la API key")
		return
	}

	// Update user
	if err := h.DB.Model(&models.User{}).Where("id = ?", userID).Update("gemini_api_key", encrypted).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Error al guardar la API key")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "API key guardada exitosamente",
		"success": true,
	})
}

// DeleteGeminiAPIKey removes the user's stored API key
func (h *UserHandler) DeleteGeminiAPIKey(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userId")
	if userID == nil {
		respondError(w, http.StatusUnauthorized, "Usuario no autenticado")
		return
	}

	if err := h.DB.Model(&models.User{}).Where("id = ?", userID).Update("gemini_api_key", "").Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Error al eliminar la API key")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "API key eliminada exitosamente",
		"success": true,
	})
}

// GetGeminiAPIKey returns the decrypted API key for the authenticated user
func (h *UserHandler) GetGeminiAPIKey(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("userId")
	if userID == nil {
		respondError(w, http.StatusUnauthorized, "Usuario no autenticado")
		return
	}

	var user models.User
	if err := h.DB.First(&user, "id = ?", userID).Error; err != nil {
		respondError(w, http.StatusNotFound, "Usuario no encontrado")
		return
	}

	if user.GeminiAPIKey == "" {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"apiKey": nil,
			"hasKey": false,
		})
		return
	}

	// Decrypt the API key
	decrypted, err := helpers.DecryptString(user.GeminiAPIKey)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Error al desencriptar la API key")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"apiKey": decrypted,
		"hasKey": true,
	})
}
