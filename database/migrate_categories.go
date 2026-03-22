package database

import (
	"capyflow/api/models"
	"log"
	"strings"

	"gorm.io/gorm"
)

func MigrateNodeCategories(db *gorm.DB) error {
	log.Println("Starting node categories migration...")

	var nodes []models.Node
	if err := db.Where("category IS NULL OR category = ''").Find(&nodes).Error; err != nil {
		log.Printf("Error getting nodes: %v", err)
		return err
	}

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
				log.Printf("Error updating node %s: %v", node.ID, err)
				continue
			}
			log.Printf("  ✓ Node %s (%s): subtitle '%s' → category '%s'",
				node.ID, node.Label, node.Subtitle, category)
			updated++
		}
	}

	log.Printf("Migration completed: %d nodes updated", updated)
	return nil
}
