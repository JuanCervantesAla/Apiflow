package database

import (
	"capyflow/api/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitDB(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(
		&models.User{},
		&models.Flow{},
		&models.Node{},
		&models.Edge{},
		&models.NodeType{},
		&models.Execution{},
		&models.FlowAnalytics{},
		&models.WorkflowCreationSession{},
		&models.AIInteraction{},
	)
	if err != nil {
		return nil, err
	}

	return db, nil
}
