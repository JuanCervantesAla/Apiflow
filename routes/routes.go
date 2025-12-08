package routes

import (
	"capyflow/api/handlers"
	"capyflow/api/middleware"
	"net/http"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

func SetupRoutes(db *gorm.DB) *mux.Router {
	router := mux.NewRouter()

	// Middleware global CORS
	router.Use(middleware.CORS)

	// Handlers
	flowHandler := handlers.NewFlowHandler(db)
	execHandler := handlers.NewExecutionHandler(db)
	userHandler := handlers.NewUserHandler(db)

	// API Routes
	api := router.PathPrefix("/api").Subrouter()

	// Rutas públicas
	api.HandleFunc("/register", userHandler.Register).Methods("POST")
	api.HandleFunc("/login", userHandler.Login).Methods("POST", "OPTIONS")

	// Health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	// Rutas protegidas (requieren JWT)
	protected := api.NewRoute().Subrouter()
	protected.Use(middleware.JWTAuth)

	protected.HandleFunc("/flows", flowHandler.GetAllFlows).Methods("GET")
	protected.HandleFunc("/flows", flowHandler.CreateFlow).Methods("POST")
	protected.HandleFunc("/flows/{id}", flowHandler.GetFlow).Methods("GET")
	protected.HandleFunc("/flows/{id}", flowHandler.UpdateFlow).Methods("PUT")
	protected.HandleFunc("/flows/{id}", flowHandler.DeleteFlow).Methods("DELETE")
	protected.HandleFunc("/flows/{id}/save", flowHandler.SaveFlowData).Methods("POST")
	protected.HandleFunc("/flows/{id}/execute", execHandler.ExecuteFlow).Methods("POST")

	return router
}
