package database

import (
	"capyflow/api/models"
	"log"
	"strings"

	"gorm.io/gorm"
)

// MigrateNodeCategories actualiza los nodos existentes que tengan subtitle como trigger
// pero no tengan category configurada
func MigrateNodeCategories(db *gorm.DB) error {
	log.Println("🔄 Iniciando migración de categorías de nodos...")

	var nodes []models.Node
	if err := db.Where("category IS NULL OR category = ''").Find(&nodes).Error; err != nil {
		log.Printf("Error al obtener nodos: %v", err)
		return err
	}

	log.Printf("📋 Encontrados %d nodos sin categoría", len(nodes))

	// Mapeo de subtítulos a categorías
	subtitleToCategoryMap := map[string]string{
		"trigger":     "trigger",
		"ai":          "ai",
		"data":        "data",
		"logic":       "logic",
		"io":          "io",
		"integration": "integration",
	}

	updated := 0
	for _, node := range nodes {
		subtitleLower := strings.ToLower(node.Subtitle)
		if category, exists := subtitleToCategoryMap[subtitleLower]; exists {
			if err := db.Model(&node).Update("category", category).Error; err != nil {
				log.Printf("Error actualizando nodo %s: %v", node.ID, err)
				continue
			}
			log.Printf("  ✓ Nodo %s (%s): subtitle '%s' → category '%s'",
				node.ID, node.Label, node.Subtitle, category)
			updated++
		}
	}

	log.Printf("Migración completada: %d nodos actualizados", updated)
	return nil
}
