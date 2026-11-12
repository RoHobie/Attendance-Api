// Package auth validates bearer tokens and enforces roles.
package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"school-management/internal/database"
	"school-management/internal/models"
)

type ctxKey struct{}

// Authenticate resolves the bearer token to a user and stores it in the request context.
func Authenticate(db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		token, ok := strings.CutPrefix(h, "Bearer ")
		if !ok || strings.TrimSpace(token) == "" {
			writeErr(w, http.StatusUnauthorized, "missing or invalid token")
			return
		}
		var u models.User
		err := db.QueryRowContext(r.Context(),
			`SELECT id, name, role FROM users WHERE token_hash = ?`,
			database.HashToken(strings.TrimSpace(token))).Scan(&u.ID, &u.Name, &u.Role)
		if err == sql.ErrNoRows {
			writeErr(w, http.StatusUnauthorized, "missing or invalid token")
			return
		}
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "internal error")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
	})
}

// UserFrom returns the authenticated user set by Authenticate.
func UserFrom(r *http.Request) models.User {
	u, _ := r.Context().Value(ctxKey{}).(models.User)
	return u
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
