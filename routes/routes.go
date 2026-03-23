package routes

import (
	"capyflow/api/handlers"
	"capyflow/api/middleware"
	"capyflow/api/services"
	"capyflow/api/websocket"
	"net/http"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

func SetupRoutes(db *gorm.DB, hub *websocket.Hub) *mux.Router {
	router := mux.NewRouter()

	// Global CORS Middleware
	router.Use(middleware.CORS)

	// Services
	analyticsService := services.NewAnalyticsService(db)

	// Handlers
	flowHandler := handlers.NewFlowHandler(db)
	execHandler := handlers.NewExecutionHandler(db, hub)
	userHandler := handlers.NewUserHandler(db)
	nodeTypeHandler := handlers.NewNodeTypeHandler(db)
	connectionHandler := handlers.NewConnectionHandler(db)
	webhookHandler := handlers.NewWebhookHandler(db, hub)
	wsHandler := handlers.NewWebSocketHandler(hub)
	wsTicketHandler := handlers.NewWSTicketHandler()
	aiHandler := handlers.NewAIHandler(db)
	analyticsHandler := handlers.NewAnalyticsHandler(analyticsService)

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

	// AI Generation
	protected.HandleFunc("/ai/generate-flow", aiHandler.GenerateFlowWithAI).Methods("POST", "OPTIONS")
	protected.HandleFunc("/ai/repair-flow", aiHandler.RepairFlowWithAI).Methods("POST", "OPTIONS")
	protected.HandleFunc("/ai/fix-flow", aiHandler.FixFlowWithAI).Methods("POST", "OPTIONS")

	// Flows
	protected.HandleFunc("/flows", flowHandler.GetAllFlows).Methods("GET")
	protected.HandleFunc("/flows", flowHandler.CreateFlow).Methods("POST")
	protected.HandleFunc("/flows/{id}", flowHandler.GetFlow).Methods("GET")
	protected.HandleFunc("/flows/{id}", flowHandler.UpdateFlow).Methods("PUT")
	protected.HandleFunc("/flows/{id}", flowHandler.DeleteFlow).Methods("DELETE")
	protected.HandleFunc("/flows/{id}/save", flowHandler.SaveFlowData).Methods("POST")
	protected.HandleFunc("/flows/{id}/clone", flowHandler.CloneFlow).Methods("POST")
	protected.HandleFunc("/flows/{id}/export", flowHandler.ExportFlow).Methods("GET")
	protected.HandleFunc("/flows/import", flowHandler.ImportFlow).Methods("POST")
	protected.HandleFunc("/flows/{id}/versions", flowHandler.GetFlowVersions).Methods("GET")
	protected.HandleFunc("/flows/{id}/rollback", flowHandler.RollbackFlow).Methods("POST")
	protected.HandleFunc("/flows/{id}/share", flowHandler.CreateOrGetFlowShare).Methods("POST")
	protected.HandleFunc("/flows/{id}/share", flowHandler.UpdateFlowShare).Methods("PUT")
	protected.HandleFunc("/flows/{id}/share/regenerate", flowHandler.RegenerateFlowShare).Methods("POST")
	protected.HandleFunc("/flows/{id}/execute", execHandler.ExecuteFlow).Methods("POST")

	// Executions
	protected.HandleFunc("/flows/{id}/executions", execHandler.GetFlowExecutions).Methods("GET")
	protected.HandleFunc("/executions", execHandler.GetAllExecutions).Methods("GET")
	protected.HandleFunc("/executions/{id}", execHandler.GetExecution).Methods("GET")

	// Node Types
	protected.HandleFunc("/node-types", nodeTypeHandler.GetAllNodeTypes).Methods("GET")
	protected.HandleFunc("/node-types/category", nodeTypeHandler.GetNodeTypesByCategory).Methods("GET")
	protected.HandleFunc("/node-schemas", nodeTypeHandler.GetNodeSchemas).Methods("GET")
	protected.HandleFunc("/node-schemas/{type}", nodeTypeHandler.GetNodeSchema).Methods("GET")

	// Connections (reusable credentials)
	protected.HandleFunc("/connections", connectionHandler.GetAllConnections).Methods("GET")
	protected.HandleFunc("/connections", connectionHandler.CreateConnection).Methods("POST")
	protected.HandleFunc("/connections/{id}", connectionHandler.GetConnection).Methods("GET")
	protected.HandleFunc("/connections/{id}", connectionHandler.UpdateConnection).Methods("PUT")
	protected.HandleFunc("/connections/{id}", connectionHandler.DeleteConnection).Methods("DELETE")

	// Analytics
	protected.HandleFunc("/analytics/summary", analyticsHandler.GetAnalyticsSummary).Methods("GET", "OPTIONS")
	protected.HandleFunc("/analytics/flows/{id}", analyticsHandler.GetFlowAnalytics).Methods("GET", "OPTIONS")
	protected.HandleFunc("/analytics/flows/{id}/sessions", analyticsHandler.GetFlowSessions).Methods("GET", "OPTIONS")
	protected.HandleFunc("/analytics/flows/{id}/ai-interactions", analyticsHandler.GetFlowAIInteractions).Methods("GET", "OPTIONS")
	protected.HandleFunc("/analytics/flows/{id}/sessions/start", analyticsHandler.StartSession).Methods("POST", "OPTIONS")
	protected.HandleFunc("/analytics/sessions/{sessionId}/end", analyticsHandler.EndSession).Methods("POST", "OPTIONS")
	protected.HandleFunc("/analytics/sessions/{sessionId}/activity", analyticsHandler.UpdateSessionActivity).Methods("POST", "OPTIONS")
	protected.HandleFunc("/analytics/sessions/{sessionId}/save", analyticsHandler.IncrementSave).Methods("POST", "OPTIONS")
	protected.HandleFunc("/analytics/flows/{id}/ai-interaction", analyticsHandler.LogAIInteraction).Methods("POST", "OPTIONS")
	protected.HandleFunc("/analytics/flows/{id}/complexity", analyticsHandler.CalculateComplexity).Methods("POST", "OPTIONS")
	protected.HandleFunc("/analytics/flows/{id}/execution", analyticsHandler.UpdateExecutionStats).Methods("POST", "OPTIONS")
	protected.HandleFunc("/analytics/flows/{id}/update-time", analyticsHandler.UpdateCreationTime).Methods("POST", "OPTIONS")

	// Webhooks (protected to get URL)
	protected.HandleFunc("/webhooks/{id}/url", webhookHandler.GetWebhookURL).Methods("GET", "OPTIONS")

	// Public webhooks (to receive external webhooks)
	api.HandleFunc("/webhooks/{id}", webhookHandler.HandleWebhook).Methods("POST", "GET", "OPTIONS")
	api.HandleFunc("/public/flows/{shareId}", flowHandler.GetPublicSharedFlow).Methods("GET")

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
