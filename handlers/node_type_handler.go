package handlers

import (
	"capyflow/api/models"
	"capyflow/api/validators"
	"net/http"

	"github.com/gorilla/mux"
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
		RespondError(w, http.StatusInternalServerError, "Error al obtener tipos de nodos")
		return
	}

	RespondJSON(w, http.StatusOK, nodeTypes)
}

// GetNodeTypesByCategory - Obtiene tipos de nodos por categoría
func (h *NodeTypeHandler) GetNodeTypesByCategory(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")

	if category == "" {
		RespondError(w, http.StatusBadRequest, "Categoría no especificada")
		return
	}

	var nodeTypes []models.NodeType

	if err := h.DB.Where("category = ? AND is_active = ?", category, true).
		Order("name").Find(&nodeTypes).Error; err != nil {
		RespondError(w, http.StatusInternalServerError, "Error al obtener tipos de nodos")
		return
	}

	RespondJSON(w, http.StatusOK, nodeTypes)
}

// GetNodeSchemas - Obtiene los schemas de validación para todos los nodos
// Query param opcional: ?mode=basic (solo parámetros básicos) o ?mode=advanced (todos)
func (h *NodeTypeHandler) GetNodeSchemas(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")

	var schemas map[string]validators.NodeSchema
	if mode == "basic" {
		schemas = validators.GetAllNodeSchemasFiltered("basic")
	} else {
		schemas = validators.GetAllNodeSchemas()
	}

	RespondJSON(w, http.StatusOK, schemas)
}

// GetNodeSchema - Obtiene el schema de validación para un tipo de nodo específico
// Query param opcional: ?mode=basic (solo parámetros básicos) o ?mode=advanced (todos)
func (h *NodeTypeHandler) GetNodeSchema(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeType := vars["type"]
	mode := r.URL.Query().Get("mode")

	var schema validators.NodeSchema
	var exists bool

	if mode == "basic" {
		schema, exists = validators.GetNodeSchemaFiltered(nodeType, "basic")
	} else {
		schema, exists = validators.GetNodeSchema(nodeType)
	}

	if !exists {
		RespondError(w, http.StatusNotFound, "Schema not found for node type")
		return
	}

	RespondJSON(w, http.StatusOK, schema)
}
