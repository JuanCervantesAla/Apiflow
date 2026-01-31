package main

import (
	"capyflow/api/config"
	"capyflow/api/database"
	"capyflow/api/middleware"
	"capyflow/api/routes"
	"capyflow/api/websocket"
	"log"
	"net/http"
)

func main() {
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

	log.Printf("Server ready on port %s", cfg.Port)
	log.Printf("WebSocket server ready at ws://localhost:%s/api/ws", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, router))
}
