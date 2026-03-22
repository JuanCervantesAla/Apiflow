package main

import (
	"capyflow/api/config"
	"capyflow/api/database"
	"capyflow/api/middleware"
	"capyflow/api/routes"
	"capyflow/api/services"
	"capyflow/api/websocket"
	"log"
	"net/http"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	cfg := config.LoadConfig()

	db, err := database.InitDB(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Cant initialize database: %v", err)
	}

	// Migrations
	database.SeedAdminUser(db)
	database.SeedNodeTypes(db)
	database.MigrateNodeCategories(db)

	hub := websocket.NewHub()
	go hub.Run()

	router := routes.SetupRoutes(db, hub)
	router.Use(middleware.CORS)

	// Start cron scheduler (background execution of cron-trigger flows)
	cronService := services.NewCronService(db, hub)
	go cronService.Start()

	log.Printf("Server ready on port %s", cfg.Port)
	log.Printf("WebSocket server ready at ws://localhost:%s/api/ws", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, router))
}
