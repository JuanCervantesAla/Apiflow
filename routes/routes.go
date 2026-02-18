package routes

import (
	"capyflow/api/handlers"
	"capyflow/api/middleware"
	"capyflow/api/websocket"
	"net/http"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

func SetupRoutes(db *gorm.DB, hub *websocket.Hub) *mux.Router {
	router := mux.NewRouter()

	// Middleware global CORS
	router.Use(middleware.CORS)

	// Handlers
	flowHandler := handlers.NewFlowHandler(db)
	execHandler := handlers.NewExecutionHandler(db, hub)
	userHandler := handlers.NewUserHandler(db)
	nodeTypeHandler := handlers.NewNodeTypeHandler(db)
	webhookHandler := handlers.NewWebhookHandler(db, hub)
	wsHandler := handlers.NewWebSocketHandler(hub)
	wsTicketHandler := handlers.NewWSTicketHandler()

	// API Routes
	api := router.PathPrefix("/api").Subrouter()

	// Public routes
	api.HandleFunc("/register", userHandler.Register).Methods("POST", "OPTIONS")
	api.HandleFunc("/login", userHandler.Login).Methods("POST", "OPTIONS")

	// Health check
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}).Methods("GET")

	// Protected routezzzz
	protected := api.NewRoute().Subrouter()
	protected.Use(middleware.JWTAuth)

	// User
	protected.HandleFunc("/me", userHandler.GetMe).Methods("GET", "OPTIONS")

	// AI Generation (opcional - requiere OPENAI_API_KEY en env)
	protected.HandleFunc("/ai/generate-flow", handlers.GenerateFlowWithAI).Methods("POST", "OPTIONS")

	// Flows
	protected.HandleFunc("/flows", flowHandler.GetAllFlows).Methods("GET")
	protected.HandleFunc("/flows", flowHandler.CreateFlow).Methods("POST")
	protected.HandleFunc("/flows/{id}", flowHandler.GetFlow).Methods("GET")
	protected.HandleFunc("/flows/{id}", flowHandler.UpdateFlow).Methods("PUT")
	protected.HandleFunc("/flows/{id}", flowHandler.DeleteFlow).Methods("DELETE")
	protected.HandleFunc("/flows/{id}/save", flowHandler.SaveFlowData).Methods("POST")
	protected.HandleFunc("/flows/{id}/execute", execHandler.ExecuteFlow).Methods("POST")

	// Executions
	protected.HandleFunc("/flows/{id}/executions", execHandler.GetFlowExecutions).Methods("GET")
	protected.HandleFunc("/executions", execHandler.GetAllExecutions).Methods("GET")
	protected.HandleFunc("/executions/{id}", execHandler.GetExecution).Methods("GET")

	// Node Types
	protected.HandleFunc("/node-types", nodeTypeHandler.GetAllNodeTypes).Methods("GET")
	protected.HandleFunc("/node-types/category", nodeTypeHandler.GetNodeTypesByCategory).Methods("GET")

	// Webhooks (protegidas para obtener URL)
	protected.HandleFunc("/webhooks/{id}/url", webhookHandler.GetWebhookURL).Methods("GET", "OPTIONS")

	// Webhooks públicos (para recibir webhooks externos)
	api.HandleFunc("/webhooks/{id}", webhookHandler.HandleWebhook).Methods("POST", "GET", "OPTIONS")

	// WebSocket Ticket
	protected.HandleFunc("/ws/ticket", wsTicketHandler.GenerateTicket).Methods("POST", "OPTIONS")

	// WebSocket Connection
	wsProtected := api.NewRoute().Subrouter()
	wsProtected.Use(middleware.WSTicketAuth)
	wsProtected.HandleFunc("/ws", wsHandler.HandleWebSocket).Methods("GET")

	// OPTIONS global
	router.Methods(http.MethodOptions).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return router
}
