package database

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math/rand"
	"sort"
	"time"
)

// HashToken returns the hex SHA-256 of a bearer token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type seedUser struct {
	name, role, token string
	classes           []string // class names this teacher is assigned to
}

// Demo accounts. Tokens are for local development only.
var seedUsers = []seedUser{
	{"Dr. Suresh Iyer (Principal)", "ADMIN", "admin-suresh-token", nil},
	{"Ananya Sharma", "TEACHER", "teacher-ananya-token", []string{"Class 10-A", "Class 9-A"}},
	{"Rajesh Verma", "TEACHER", "teacher-rajesh-token", []string{"Class 10-A", "Class 10-B"}},
	{"Priya Nair", "TEACHER", "teacher-priya-token", []string{"Class 9-B", "Class 8-A"}},
	{"Vikram Singh Rathore", "TEACHER", "teacher-vikram-token", []string{"Class 8-B"}},
}

var seedGrades = []int{8, 9, 10}
var seedSections = []string{"A", "B"}

var firstNames = []string{
	"Aarav", "Vivaan", "Aditya", "Arjun", "Sai", "Reyansh", "Krishna", "Ishaan", "Rohan", "Kabir",
	"Dhruv", "Karthik", "Siddharth", "Pranav", "Harsh", "Yash", "Nikhil", "Rahul", "Manish", "Tanmay",
	"Ananya", "Diya", "Aadhya", "Saanvi", "Ishita", "Kavya", "Meera", "Riya", "Priyanka", "Neha",
	"Pooja", "Shreya", "Lakshmi", "Divya", "Aarti", "Sneha", "Tanvi", "Nandini", "Swati", "Anjali",
}

var lastNames = []string{
	"Sharma", "Verma", "Gupta", "Patel", "Iyer", "Nair", "Reddy", "Rao", "Mehta", "Joshi",
	"Kulkarni", "Deshmukh", "Banerjee", "Chatterjee", "Das", "Singh", "Kapoor", "Malhotra", "Pillai", "Menon",
	"Bhatt", "Agarwal", "Chopra", "Naidu", "Shetty", "Mishra", "Pandey", "Yadav", "Thakur", "Ghosh",
}

const studentsPerClass = 20

// Seed fills an empty database with synthetic Indian school data.
// It does nothing if any user already exists.
func Seed(db *sql.DB, now time.Time) error {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rng := rand.New(rand.NewSource(2026)) // deterministic data

	userIDs := map[string]int64{}
	for _, u := range seedUsers {
		res, err := tx.Exec(`INSERT INTO users (name, role, token_hash) VALUES (?, ?, ?)`,
			u.name, u.role, HashToken(u.token))
		if err != nil {
			return err
		}
		userIDs[u.name], _ = res.LastInsertId()
	}

	type classInfo struct {
		id       int64
		students []int64
		teachers []int64
	}
	classes := map[string]*classInfo{}
	var classOrder []string
	for _, g := range seedGrades {
		for _, s := range seedSections {
			name := fmt.Sprintf("Class %d-%s", g, s)
			res, err := tx.Exec(`INSERT INTO classes (name, grade, section) VALUES (?, ?, ?)`, name, g, s)
			if err != nil {
				return err
			}
			id, _ := res.LastInsertId()
			ci := &classInfo{id: id}

			names := make([]string, 0, studentsPerClass)
			for i := 0; i < studentsPerClass; i++ {
				names = append(names, firstNames[rng.Intn(len(firstNames))]+" "+lastNames[rng.Intn(len(lastNames))])
			}
			sort.Strings(names) // roll-number order
			for _, nm := range names {
				r, err := tx.Exec(`INSERT INTO students (name, class_id) VALUES (?, ?)`, nm, id)
				if err != nil {
					return err
				}
				sid, _ := r.LastInsertId()
				ci.students = append(ci.students, sid)
			}
			classes[name] = ci
			classOrder = append(classOrder, name)
		}
	}

	for _, u := range seedUsers {
		for _, cn := range u.classes {
			ci := classes[cn]
			if _, err := tx.Exec(`INSERT INTO teacher_assignments (teacher_id, class_id) VALUES (?, ?)`,
				userIDs[u.name], ci.id); err != nil {
				return err
			}
			ci.teachers = append(ci.teachers, userIDs[u.name])
		}
	}

	// Morning attendance for the last 10 school days (Mon–Fri) before today.
	// Indian school weeks run Monday to Saturday in many boards; we keep Mon–Fri for simplicity.
	var days []time.Time
	for d := now.AddDate(0, 0, -1); len(days) < 10; d = d.AddDate(0, 0, -1) {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			days = append(days, d)
		}
	}
	ist := time.FixedZone("IST", 5*3600+1800)
	for i := len(days) - 1; i >= 0; i-- { // oldest first
		d := days[i]
		date := d.Format("2006-01-02")
		stamp := time.Date(d.Year(), d.Month(), d.Day(), 9, 5, 0, 0, ist).Format(time.RFC3339)
		for _, cn := range classOrder {
			ci := classes[cn]
			teacher := ci.teachers[0]
			for _, sid := range ci.students {
				status := "PRESENT"
				if rng.Intn(100) < 8 {
					status = "ABSENT"
				}
				res, err := tx.Exec(`INSERT INTO attendance (student_id, class_id, attendance_date, session, status, recorded_by)
					VALUES (?, ?, ?, 'MORNING', ?, ?)`, sid, ci.id, date, status, teacher)
				if err != nil {
					return err
				}
				aid, _ := res.LastInsertId()
				if _, err := tx.Exec(`INSERT INTO audit_logs (actor_id, action, entity_id, detail, created_at)
					VALUES (?, 'ATTENDANCE_CREATED', ?, ?, ?)`, teacher, aid, "status="+status, stamp); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit()
}
