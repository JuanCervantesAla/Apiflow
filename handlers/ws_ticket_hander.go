package handlers

import (
	"capyflow/api/middleware"
	"encoding/json"
	"net/http"
)

type WSTicketHandler struct{}

func NewWSTicketHandler() *WSTicketHandler {
	return &WSTicketHandler{}
}

type TicketResponse struct {
	Ticket    string `json:"ticket"`
	ExpiresIn int    `json:"expiresIn"`
}

// GenerateTicket - POST /ws/ticket (requiere JWT auth)
func (h *WSTicketHandler) GenerateTicket(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("userId").(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ticket := middleware.GenerateTicket(userID)

	response := TicketResponse{
		Ticket:    ticket,
		ExpiresIn: 30,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
