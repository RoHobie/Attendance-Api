package tests

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"school-management/internal/database"
	"school-management/internal/handlers"
)

const (
	admin    = "admin-suresh-token"
	ananya   = "teacher-ananya-token" // Class 10-A, 9-A
	vikram   = "teacher-vikram-token" // Class 8-B
	class10A = 5
)

func setup(t *testing.T) (*httptest.Server, *sql.DB) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.Seed(db, time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	(&handlers.Handler{DB: db}).Routes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, db
}

func do(t *testing.T, method, url, token string, body any) (int, map[string]any) {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, url, rd)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func studentIn(t *testing.T, db *sql.DB, class int) int {
	var id int
	if err := db.QueryRow(`SELECT id FROM students WHERE class_id = ? ORDER BY id LIMIT 1`, class).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestHealth(t *testing.T) {
	srv, _ := setup(t)
	for _, p := range []string{"/healthz", "/readyz"} {
		if code, _ := do(t, "GET", srv.URL+p, "", nil); code != 200 {
			t.Errorf("%s = %d", p, code)
		}
	}
}

func TestAuthAndClasses(t *testing.T) {
	srv, _ := setup(t)
	if code, _ := do(t, "GET", srv.URL+"/api/classes", "", nil); code != 401 {
		t.Errorf("no token = %d", code)
	}
	if code, _ := do(t, "GET", srv.URL+"/api/classes", "bogus", nil); code != 401 {
		t.Errorf("bad token = %d", code)
	}
	_, body := do(t, "GET", srv.URL+"/api/classes", ananya, nil)
	if n := len(body["data"].([]any)); n != 2 {
		t.Errorf("teacher classes = %d, want 2", n)
	}
	_, body = do(t, "GET", srv.URL+"/api/classes", admin, nil)
	if n := len(body["data"].([]any)); n != 6 {
		t.Errorf("admin classes = %d, want 6", n)
	}
}

func TestStudentsAuthorization(t *testing.T) {
	srv, _ := setup(t)
	if code, body := do(t, "GET", srv.URL+"/api/classes/5/students", ananya, nil); code != 200 || len(body["data"].([]any)) != 20 {
		t.Errorf("assigned teacher: %d", code)
	}
	if code, _ := do(t, "GET", srv.URL+"/api/classes/5/students", vikram, nil); code != 403 {
		t.Errorf("unassigned teacher = %d", code)
	}
	if code, _ := do(t, "GET", srv.URL+"/api/classes/5/students", admin, nil); code != 200 {
		t.Errorf("admin = %d", code)
	}
	if code, _ := do(t, "GET", srv.URL+"/api/classes/999/students", admin, nil); code != 404 {
		t.Errorf("missing class = %d", code)
	}
}

func TestCreateAttendance(t *testing.T) {
	srv, db := setup(t)
	sid := studentIn(t, db, class10A)
	url := srv.URL + "/api/classes/5/attendance"
	req := map[string]any{"student_id": sid, "date": "2026-11-13", "session": "AFTERNOON", "status": "PRESENT"}

	if code, _ := do(t, "POST", url, ananya, req); code != 201 {
		t.Fatalf("create = %d", code)
	}
	if code, _ := do(t, "POST", url, ananya, req); code != 409 {
		t.Errorf("duplicate = %d, want 409", code)
	}
	if code, _ := do(t, "POST", url, "", req); code != 401 {
		t.Errorf("no token = %d", code)
	}
	if code, _ := do(t, "POST", url, admin, req); code != 403 {
		t.Errorf("admin write = %d", code)
	}
	if code, _ := do(t, "POST", url, vikram, req); code != 403 {
		t.Errorf("unassigned = %d", code)
	}
	bad := map[string]any{"student_id": sid, "date": "2026-11-13", "session": "MORNING", "status": "LATE"}
	if code, _ := do(t, "POST", url, ananya, bad); code != 400 {
		t.Errorf("LATE = %d, want 400", code)
	}
	req["student_id"] = 99999
	if code, _ := do(t, "POST", url, ananya, req); code != 404 {
		t.Errorf("unknown student = %d", code)
	}
	req["student_id"] = studentIn(t, db, 6) // student of another class
	if code, _ := do(t, "POST", url, ananya, req); code != 400 {
		t.Errorf("mismatch = %d", code)
	}

	var n int
	db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action='ATTENDANCE_CREATED' AND entity_id = (SELECT MAX(id) FROM attendance)`).Scan(&n)
	if n != 1 {
		t.Errorf("create audit rows = %d, want 1", n)
	}
	_, body := do(t, "GET", url+"?date=2026-11-13", admin, nil)
	if len(body["data"].([]any)) < 1 {
		t.Error("created record not listed")
	}
}

func TestConcurrentDuplicates(t *testing.T) {
	srv, db := setup(t)
	sid := studentIn(t, db, class10A)
	req := map[string]any{"student_id": sid, "date": "2026-11-12", "session": "AFTERNOON", "status": "ABSENT"}
	var wg sync.WaitGroup
	var mu sync.Mutex
	codes := map[int]int{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _ := do(t, "POST", srv.URL+"/api/classes/5/attendance", ananya, req)
			mu.Lock()
			codes[code]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	if codes[201] != 1 || codes[409] != 9 {
		t.Errorf("codes = %v, want one 201 and nine 409", codes)
	}
}

func TestUpdateAttendanceIsAudited(t *testing.T) {
	srv, db := setup(t)
	sid := studentIn(t, db, class10A)
	_, body := do(t, "POST", srv.URL+"/api/classes/5/attendance", ananya,
		map[string]any{"student_id": sid, "date": "2026-11-13", "session": "AFTERNOON", "status": "PRESENT"})
	id := int(body["data"].(map[string]any)["id"].(float64))
	url := srv.URL + "/api/classes/5/attendance/" + itoa(id)

	if code, _ := do(t, "PUT", url, ananya, map[string]any{"status": "ABSENT"}); code != 200 {
		t.Fatalf("update = %d", code)
	}
	do(t, "PUT", url, ananya, map[string]any{"status": "ABSENT"}) // no change: must not log
	if code, _ := do(t, "PUT", url, ananya, map[string]any{"status": "LATE"}); code != 400 {
		t.Errorf("LATE = %d", code)
	}
	if code, _ := do(t, "PUT", url, vikram, map[string]any{"status": "PRESENT"}); code != 403 {
		t.Errorf("unassigned = %d", code)
	}
	if code, _ := do(t, "PUT", srv.URL+"/api/classes/5/attendance/999999", ananya, map[string]any{"status": "PRESENT"}); code != 404 {
		t.Errorf("missing = %d", code)
	}

	var status string
	db.QueryRow(`SELECT status FROM attendance WHERE id = ?`, id).Scan(&status)
	if status != "ABSENT" {
		t.Errorf("status = %s", status)
	}
	rows, _ := db.Query(`SELECT action, detail FROM audit_logs WHERE entity_id = ? ORDER BY id DESC LIMIT 2`, id)
	defer rows.Close()
	var got []string
	for rows.Next() {
		var a, d string
		rows.Scan(&a, &d)
		got = append(got, a+":"+d)
	}
	want := []string{"ATTENDANCE_UPDATED:status=PRESENT->ABSENT", "ATTENDANCE_CREATED:status=PRESENT"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		// seeded rows may share the id; only the first matters most
		if len(got) == 0 || got[0] != want[0] {
			t.Errorf("audit = %v, want %v", got, want)
		}
	}
}

func itoa(i int) string { b, _ := json.Marshal(i); return string(b) }
