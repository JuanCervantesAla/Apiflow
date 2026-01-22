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
	nodeTypeHandler := handlers.NewNodeTypeHandler(db)

	// API Routes
	api := router.PathPrefix("/api").Subrouter()

	// Rutas públicas
	// Allow preflight OPTIONS on register as well (some browsers send OPTIONS before POST)
	api.HandleFunc("/register", userHandler.Register).Methods("POST", "OPTIONS")
	api.HandleFunc("/login", userHandler.Login).Methods("POST", "OPTIONS")

	// Health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	// Rutas protegidas (requieren JWT)
	protected := api.NewRoute().Subrouter()
	protected.Use(middleware.JWTAuth)

	//User
	protected.HandleFunc("/me", userHandler.GetMe).Methods("GET", "OPTIONS")

	//Flows
	protected.HandleFunc("/flows", flowHandler.GetAllFlows).Methods("GET")
	protected.HandleFunc("/flows", flowHandler.CreateFlow).Methods("POST")
	protected.HandleFunc("/flows/{id}", flowHandler.GetFlow).Methods("GET")
	protected.HandleFunc("/flows/{id}", flowHandler.UpdateFlow).Methods("PUT")
	protected.HandleFunc("/flows/{id}", flowHandler.DeleteFlow).Methods("DELETE")
	protected.HandleFunc("/flows/{id}/save", flowHandler.SaveFlowData).Methods("POST")
	protected.HandleFunc("/flows/{id}/execute", execHandler.ExecuteFlow).Methods("POST")

	// Node Types
	protected.HandleFunc("/node-types", nodeTypeHandler.GetAllNodeTypes).Methods("GET")
	protected.HandleFunc("/node-types/category", nodeTypeHandler.GetNodeTypesByCategory).Methods("GET")

	router.Methods(http.MethodOptions).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return router
}
