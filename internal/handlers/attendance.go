package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"school-management/internal/auth"
	"school-management/internal/models"
)

func validStatus(s string) bool  { return s == "PRESENT" || s == "ABSENT" }
func validSession(s string) bool { return s == "MORNING" || s == "AFTERNOON" }
func validDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func decode(r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	return dec.Decode(v) == nil
}

func (h *Handler) CreateAttendance(w http.ResponseWriter, r *http.Request) {
	classID, ok := h.classAccess(w, r, true)
	if !ok {
		return
	}
	var req models.CreateAttendanceRequest
	if !decode(r, &req) {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.StudentID <= 0 || !validDate(req.Date) || !validSession(req.Session) || !validStatus(req.Status) {
		writeErr(w, http.StatusBadRequest, "student_id, date (YYYY-MM-DD), session (MORNING|AFTERNOON) and status (PRESENT|ABSENT) are required")
		return
	}

	var studentClass int64
	err := h.DB.QueryRowContext(r.Context(), `SELECT class_id FROM students WHERE id = ?`, req.StudentID).Scan(&studentClass)
	if err == sql.ErrNoRows {
		writeErr(w, http.StatusNotFound, "student not found")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	if studentClass != classID {
		writeErr(w, http.StatusBadRequest, "student does not belong to this class")
		return
	}

	actor := auth.UserFrom(r).ID
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback()

	// The UNIQUE constraint decides duplicates, so concurrent requests are safe.
	res, err := tx.ExecContext(r.Context(),
		`INSERT INTO attendance (student_id, class_id, attendance_date, session, status, recorded_by)
		 VALUES (?, ?, ?, ?, ?, ?)`, req.StudentID, classID, req.Date, req.Session, req.Status, actor)
	if isUniqueViolation(err) {
		writeErr(w, http.StatusConflict, "attendance already recorded for this student, date and session")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	id, _ := res.LastInsertId()
	if err := writeAudit(r, tx, actor, "ATTENDANCE_CREATED", id, "status="+req.Status); err != nil {
		serverErr(w)
		return
	}
	if err := tx.Commit(); err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": models.Attendance{
		ID: id, StudentID: req.StudentID, Date: req.Date, Session: req.Session,
		Status: req.Status, RecordedBy: actor,
	}})
}

func (h *Handler) UpdateAttendance(w http.ResponseWriter, r *http.Request) {
	classID, ok := h.classAccess(w, r, true)
	if !ok {
		return
	}
	attID, err := strconv.ParseInt(r.PathValue("attendanceID"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid attendance id")
		return
	}
	var req models.UpdateAttendanceRequest
	if !decode(r, &req) || !validStatus(req.Status) {
		writeErr(w, http.StatusBadRequest, "status (PRESENT|ABSENT) is required")
		return
	}

	actor := auth.UserFrom(r).ID
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		serverErr(w)
		return
	}
	defer tx.Rollback()

	var old string
	err = tx.QueryRowContext(r.Context(),
		`SELECT status FROM attendance WHERE id = ? AND class_id = ?`, attID, classID).Scan(&old)
	if err == sql.ErrNoRows {
		writeErr(w, http.StatusNotFound, "attendance record not found")
		return
	}
	if err != nil {
		serverErr(w)
		return
	}
	if old != req.Status {
		if _, err := tx.ExecContext(r.Context(),
			`UPDATE attendance SET status = ?, recorded_by = ? WHERE id = ?`, req.Status, actor, attID); err != nil {
			serverErr(w)
			return
		}
		if err := writeAudit(r, tx, actor, "ATTENDANCE_UPDATED", attID, "status="+old+"->"+req.Status); err != nil {
			serverErr(w)
			return
		}
	}
	if err := tx.Commit(); err != nil {
		serverErr(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"id": attID, "status": req.Status}})
}

func (h *Handler) ListAttendance(w http.ResponseWriter, r *http.Request) {
	classID, ok := h.classAccess(w, r, false)
	if !ok {
		return
	}
	date := r.URL.Query().Get("date")
	if !validDate(date) {
		writeErr(w, http.StatusBadRequest, "date query parameter (YYYY-MM-DD) is required")
		return
	}
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT a.id, a.student_id, s.name, a.attendance_date, a.session, a.status, a.recorded_by
		 FROM attendance a JOIN students s ON s.id = a.student_id
		 WHERE a.class_id = ? AND a.attendance_date = ? ORDER BY a.session, s.name, a.id`, classID, date)
	if err != nil {
		serverErr(w)
		return
	}
	defer rows.Close()
	out := []models.Attendance{}
	for rows.Next() {
		var a models.Attendance
		if err := rows.Scan(&a.ID, &a.StudentID, &a.StudentName, &a.Date, &a.Session, &a.Status, &a.RecordedBy); err != nil {
			serverErr(w)
			return
		}
		out = append(out, a)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func writeAudit(r *http.Request, tx *sql.Tx, actor int64, action string, entityID int64, detail string) error {
	_, err := tx.ExecContext(r.Context(),
		`INSERT INTO audit_logs (actor_id, action, entity_id, detail, created_at) VALUES (?, ?, ?, ?, ?)`,
		actor, action, entityID, detail, time.Now().UTC().Format(time.RFC3339))
	return err
}
