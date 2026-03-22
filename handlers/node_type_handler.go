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

// GetAllNodeTypes - Gets all available node types
func (h *NodeTypeHandler) GetAllNodeTypes(w http.ResponseWriter, r *http.Request) {
	var nodeTypes []models.NodeType

	if err := h.DB.Where("is_active = ?", true).Order("category, name").Find(&nodeTypes).Error; err != nil {
		RespondError(w, http.StatusInternalServerError, "Error getting node types")
		return
	}

	RespondJSON(w, http.StatusOK, nodeTypes)
}

// GetNodeTypesByCategory - Gets node types by category
func (h *NodeTypeHandler) GetNodeTypesByCategory(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")

	if category == "" {
		RespondError(w, http.StatusBadRequest, "Category not specified")
		return
	}

	var nodeTypes []models.NodeType

	if err := h.DB.Where("category = ? AND is_active = ?", category, true).
		Order("name").Find(&nodeTypes).Error; err != nil {
		RespondError(w, http.StatusInternalServerError, "Error getting node types")
		return
	}

	RespondJSON(w, http.StatusOK, nodeTypes)
}

// GetNodeSchemas - Gets validation schemas for all nodes
// Optional query param: ?mode=basic (basic parameters only) or ?mode=advanced (all)
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

// GetNodeSchema - Gets the validation schema for a specific node type
// Optional query param: ?mode=basic (basic parameters only) or ?mode=advanced (all)
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
