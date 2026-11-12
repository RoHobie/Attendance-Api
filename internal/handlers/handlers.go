// Package handlers contains the HTTP handlers. They run SQL directly.
package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"school-management/internal/auth"
	"school-management/internal/models"
)

type Handler struct {
	DB *sql.DB
}

// Routes registers all API routes (behind authentication) and the health endpoints.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.Healthz)
	mux.HandleFunc("GET /readyz", h.Readyz)

	api := http.NewServeMux()
	api.HandleFunc("GET /api/me", h.Me)
	api.HandleFunc("GET /api/classes", h.ListClasses)
	api.HandleFunc("GET /api/classes/{id}/students", h.ListStudents)
	api.HandleFunc("POST /api/classes/{id}/attendance", h.CreateAttendance)
	api.HandleFunc("GET /api/classes/{id}/attendance", h.ListAttendance)
	api.HandleFunc("PUT /api/classes/{id}/attendance/{attendanceID}", h.UpdateAttendance)
	mux.Handle("/api/", auth.Authenticate(h.DB, api))
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func serverErr(w http.ResponseWriter) {
	writeErr(w, http.StatusInternalServerError, "internal error")
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// classAccess checks the class exists and the caller may access it.
// Admins may read everything; teachers need an assignment. When write is true only
// assigned teachers pass. It writes the error response and returns false on failure.
func (h *Handler) classAccess(w http.ResponseWriter, r *http.Request, write bool) (int64, bool) {
	classID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid class id")
		return 0, false
	}
	u := auth.UserFrom(r)
	if write && u.Role != models.RoleTeacher {
		writeErr(w, http.StatusForbidden, "only teachers can record attendance")
		return 0, false
	}
	var exists int
	if err := h.DB.QueryRowContext(r.Context(), `SELECT 1 FROM classes WHERE id = ?`, classID).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			writeErr(w, http.StatusNotFound, "class not found")
		} else {
			serverErr(w)
		}
		return 0, false
	}
	if u.Role == models.RoleAdmin {
		return classID, true
	}
	err = h.DB.QueryRowContext(r.Context(),
		`SELECT 1 FROM teacher_assignments WHERE teacher_id = ? AND class_id = ?`, u.ID, classID).Scan(&exists)
	if err == sql.ErrNoRows {
		writeErr(w, http.StatusForbidden, "you are not assigned to this class")
		return 0, false
	}
	if err != nil {
		serverErr(w)
		return 0, false
	}
	return classID, true
}
