package handlers

import (
	"capyflow/api/models"
	"net/http"

	"gorm.io/gorm"
)

type NodeTypeHandler struct {
	DB *gorm.DB
}

func NewNodeTypeHandler(db *gorm.DB) *NodeTypeHandler {
	return &NodeTypeHandler{DB: db}
}

// GetAllNodeTypes - Obtiene todos los tipos de nodos disponibles
func (h *NodeTypeHandler) GetAllNodeTypes(w http.ResponseWriter, r *http.Request) {
	var nodeTypes []models.NodeType

	if err := h.DB.Where("is_active = ?", true).Order("category, name").Find(&nodeTypes).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Error al obtener tipos de nodos")
		return
	}

	respondJSON(w, http.StatusOK, nodeTypes)
}

// GetNodeTypesByCategory - Obtiene tipos de nodos por categoría
func (h *NodeTypeHandler) GetNodeTypesByCategory(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")

	if category == "" {
		respondError(w, http.StatusBadRequest, "Categoría no especificada")
		return
	}

	var nodeTypes []models.NodeType

	if err := h.DB.Where("category = ? AND is_active = ?", category, true).
		Order("name").Find(&nodeTypes).Error; err != nil {
		respondError(w, http.StatusInternalServerError, "Error al obtener tipos de nodos")
		return
	}

	respondJSON(w, http.StatusOK, nodeTypes)
}
