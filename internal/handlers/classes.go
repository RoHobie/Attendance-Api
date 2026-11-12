package handlers

import (
	"net/http"

	"school-management/internal/auth"
	"school-management/internal/models"
)

func (h *Handler) ListClasses(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	q := `SELECT id, name, grade, section FROM classes ORDER BY grade, section`
	args := []any{}
	if u.Role != models.RoleAdmin {
		q = `SELECT c.id, c.name, c.grade, c.section FROM classes c
			JOIN teacher_assignments ta ON ta.class_id = c.id
			WHERE ta.teacher_id = ? ORDER BY c.grade, c.section`
		args = append(args, u.ID)
	}
	rows, err := h.DB.QueryContext(r.Context(), q, args...)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	out := []models.Class{}
	for rows.Next() {
		var c models.Class
		if err := rows.Scan(&c.ID, &c.Name, &c.Grade, &c.Section); err != nil {
			serverErr(w)
			return
		}
		out = append(out, c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) ListStudents(w http.ResponseWriter, r *http.Request) {
	classID, ok := h.classAccess(w, r, false)
	if !ok {
		return
	}
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT id, name FROM students WHERE class_id = ? ORDER BY name, id`, classID)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	out := []models.Student{}
	for rows.Next() {
		var s models.Student
		if err := rows.Scan(&s.ID, &s.Name); err != nil {
			serverErr(w)
			return
		}
		out = append(out, s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r)
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"id": u.ID, "name": u.Name, "role": u.Role}})
}
