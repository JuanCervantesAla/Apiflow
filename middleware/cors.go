package middleware

import (
	"net/http"
	"strings"
)

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		origin := r.Header.Get("Origin")
<<<<<<< HEAD

		// 🔥 Siempre reflejar el origin en dev
		if origin != "" {
=======
		if origin == "http://localhost:5173" || origin == "http://localhost:5174" {
>>>>>>> c7c39b9eeb87dab5da0f03fb52d8eb477de4dd6c
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// 🔥 Reflejar headers solicitados en el preflight
		if reqHeaders := r.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
			w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
		} else {
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

<<<<<<< HEAD
		// 🔥 Cortar el preflight aquí
=======
		if strings.ToLower(r.Header.Get("Upgrade")) == "websocket" {
			next.ServeHTTP(w, r)
			return
		}

>>>>>>> c7c39b9eeb87dab5da0f03fb52d8eb477de4dd6c
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
