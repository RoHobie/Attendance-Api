// Package models holds request and response structures.
package models

type User struct {
	ID   int64
	Name string
	Role string
}

type Class struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Grade   int    `json:"grade"`
	Section string `json:"section"`
}

type Student struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type Attendance struct {
	ID          int64  `json:"id"`
	StudentID   int64  `json:"student_id"`
	StudentName string `json:"student_name"`
	Date        string `json:"date"`
	Session     string `json:"session"`
	Status      string `json:"status"`
	RecordedBy  int64  `json:"recorded_by"`
}

type CreateAttendanceRequest struct {
	StudentID int64  `json:"student_id"`
	Date      string `json:"date"`
	Session   string `json:"session"`
	Status    string `json:"status"`
}

type UpdateAttendanceRequest struct {
	Status string `json:"status"`
}

const (
	RoleAdmin   = "ADMIN"
	RoleTeacher = "TEACHER"
)
