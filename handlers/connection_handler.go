package handlers

import (
	"capyflow/api/models"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

type ConnectionHandler struct {
	DB *gorm.DB
}

func NewConnectionHandler(db *gorm.DB) *ConnectionHandler {
	return &ConnectionHandler{DB: db}
}

type ConnectionCreateRequest struct {
	Name     string                 `json:"name"`
	Provider string                 `json:"provider"`
	Secrets  map[string]interface{} `json:"secrets"`
	Metadata map[string]interface{} `json:"metadata"`
}

type ConnectionUpdateRequest struct {
	Name     *string                 `json:"name"`
	Provider *string                 `json:"provider"`
	Secrets  *map[string]interface{} `json:"secrets"`
	Metadata *map[string]interface{} `json:"metadata"`
	IsActive *bool                   `json:"isActive"`
}

type ConnectionResponse struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"userId"`
	Name      string                 `json:"name"`
	Provider  string                 `json:"provider"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	IsActive  bool                   `json:"isActive"`
	HasSecret bool                   `json:"hasSecret"`
	CreatedAt time.Time              `json:"createdAt"`
	UpdatedAt time.Time              `json:"updatedAt"`
}

func toConnectionResponse(c models.Connection) ConnectionResponse {
	metadata := map[string]interface{}{}
	if c.Metadata != "" {
		_ = json.Unmarshal([]byte(c.Metadata), &metadata)
	}

	return ConnectionResponse{
		ID:        c.ID,
		UserID:    c.UserID,
		Name:      c.Name,
		Provider:  c.Provider,
		Metadata:  metadata,
		IsActive:  c.IsActive,
		HasSecret: c.SecretsEncrypted != "",
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

func (h *ConnectionHandler) GetAllConnections(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value("userId").(string)

	var connections []models.Connection
	if err := h.DB.Where("user_id = ?", userID).Order("provider, name").Find(&connections).Error; err != nil {
		RespondError(w, http.StatusInternalServerError, "Error getting connections")
		return
	}

	response := make([]ConnectionResponse, 0, len(connections))
	for _, c := range connections {
		response = append(response, toConnectionResponse(c))
	}

	RespondJSON(w, http.StatusOK, response)
}

func (h *ConnectionHandler) GetConnection(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value("userId").(string)
	id := mux.Vars(r)["id"]

	var connection models.Connection
	if err := h.DB.First(&connection, "id = ? AND user_id = ?", id, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			RespondError(w, http.StatusNotFound, "Connection not found")
			return
		}
		RespondError(w, http.StatusInternalServerError, "Error getting connection")
		return
	}

	RespondJSON(w, http.StatusOK, toConnectionResponse(connection))
}

func (h *ConnectionHandler) CreateConnection(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value("userId").(string)

	var req ConnectionCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		RespondError(w, http.StatusBadRequest, "Name is required")
		return
	}
	if req.Provider == "" {
		RespondError(w, http.StatusBadRequest, "Provider is required")
		return
	}

	metadataJSON := ""
	if req.Metadata != nil {
		b, err := json.Marshal(req.Metadata)
		if err != nil {
			RespondError(w, http.StatusBadRequest, "Invalid metadata")
			return
		}
		metadataJSON = string(b)
	}

	connection := models.Connection{
		ID:        uuid.New().String(),
		UserID:    userID,
		Name:      req.Name,
		Provider:  req.Provider,
		Metadata:  metadataJSON,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := connection.SetSecrets(req.Secrets); err != nil {
		RespondError(w, http.StatusInternalServerError, "Error encrypting secrets")
		return
	}

	if err := h.DB.Create(&connection).Error; err != nil {
		RespondError(w, http.StatusInternalServerError, "Error creating connection")
		return
	}

	RespondJSON(w, http.StatusCreated, toConnectionResponse(connection))
}

func (h *ConnectionHandler) UpdateConnection(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value("userId").(string)
	id := mux.Vars(r)["id"]

	var req ConnectionUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var connection models.Connection
	if err := h.DB.First(&connection, "id = ? AND user_id = ?", id, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			RespondError(w, http.StatusNotFound, "Connection not found")
			return
		}
		RespondError(w, http.StatusInternalServerError, "Error getting connection")
		return
	}

	if req.Name != nil {
		connection.Name = *req.Name
	}
	if req.Provider != nil {
		connection.Provider = *req.Provider
	}
	if req.IsActive != nil {
		connection.IsActive = *req.IsActive
	}
	if req.Metadata != nil {
		b, err := json.Marshal(req.Metadata)
		if err != nil {
			RespondError(w, http.StatusBadRequest, "Invalid metadata")
			return
		}
		connection.Metadata = string(b)
	}
	if req.Secrets != nil {
		if err := connection.SetSecrets(*req.Secrets); err != nil {
			RespondError(w, http.StatusInternalServerError, "Error encrypting secrets")
			return
		}
	}

	connection.UpdatedAt = time.Now()
	if err := h.DB.Save(&connection).Error; err != nil {
		RespondError(w, http.StatusInternalServerError, "Error updating connection")
		return
	}

	RespondJSON(w, http.StatusOK, toConnectionResponse(connection))
}

func (h *ConnectionHandler) DeleteConnection(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value("userId").(string)
	id := mux.Vars(r)["id"]

	result := h.DB.Delete(&models.Connection{}, "id = ? AND user_id = ?", id, userID)
	if result.Error != nil {
		RespondError(w, http.StatusInternalServerError, "Error deleting connection")
		return
	}
	if result.RowsAffected == 0 {
		RespondError(w, http.StatusNotFound, "Connection not found")
		return
	}

	RespondJSON(w, http.StatusOK, map[string]string{"message": "Connection deleted"})
}
