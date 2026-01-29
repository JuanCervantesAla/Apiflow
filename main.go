package main

import (
	"capyflow/api/config"
	"capyflow/api/database"
	"capyflow/api/middleware"
	"capyflow/api/routes"
	"log"
	"net/http"
)

func main() {
	cfg := config.LoadConfig()

	db, err := database.InitDB(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Cant initialize database: %v", err)
	}

	//Migrations
	//TODO: Make better migrations
	database.SeedAdminUser(db)
	database.SeedNodeTypes(db)
	database.MigrateNodeCategories(db)

	router := routes.SetupRoutes(db)
	router.Use(middleware.CORS)

	log.Printf("Server ready on port %s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, router))
}
