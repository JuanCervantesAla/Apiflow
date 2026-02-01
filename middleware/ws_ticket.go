package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

type TicketInfo struct {
	UserID    string
	ExpiresAt time.Time
}

type TicketStore struct {
	mu      sync.RWMutex
	tickets map[string]TicketInfo
}

var ticketStore = &TicketStore{
	tickets: make(map[string]TicketInfo),
}

func GenerateTicket(userID string) string {
	ticketBytes := make([]byte, 32)
	rand.Read(ticketBytes)
	ticket := hex.EncodeToString(ticketBytes)

	ticketStore.mu.Lock()
	defer ticketStore.mu.Unlock()

	ticketStore.tickets[ticket] = TicketInfo{
		UserID:    userID,
		ExpiresAt: time.Now().Add(30 * time.Second),
	}

	go func() {
		time.Sleep(35 * time.Second)
		ticketStore.mu.Lock()
		delete(ticketStore.tickets, ticket)
		ticketStore.mu.Unlock()
	}()

	return ticket
}

func ValidateAndConsumeTicket(ticket string) (string, bool) {
	ticketStore.mu.Lock()
	defer ticketStore.mu.Unlock()

	info, exists := ticketStore.tickets[ticket]
	if !exists {
		return "", false
	}

	if time.Now().After(info.ExpiresAt) {
		delete(ticketStore.tickets, ticket)
		return "", false
	}

	delete(ticketStore.tickets, ticket)

	return info.UserID, true
}

func WSTicketAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ticket := r.URL.Query().Get("ticket")
		if ticket == "" {
			http.Error(w, "Missing ticket", http.StatusUnauthorized)
			return
		}

		userID, valid := ValidateAndConsumeTicket(ticket)
		if !valid {
			http.Error(w, "Invalid or expired ticket", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "userId", userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
