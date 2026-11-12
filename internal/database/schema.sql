CREATE TABLE IF NOT EXISTS users (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL,
    role       TEXT NOT NULL CHECK (role IN ('ADMIN', 'TEACHER')),
    token_hash TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS classes (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    name    TEXT NOT NULL,
    grade   INTEGER NOT NULL,
    section TEXT NOT NULL,
    UNIQUE (grade, section)
);

CREATE TABLE IF NOT EXISTS students (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    name     TEXT NOT NULL,
    class_id INTEGER NOT NULL REFERENCES classes(id)
);

CREATE TABLE IF NOT EXISTS teacher_assignments (
    teacher_id INTEGER NOT NULL REFERENCES users(id),
    class_id   INTEGER NOT NULL REFERENCES classes(id),
    PRIMARY KEY (teacher_id, class_id)
);

CREATE TABLE IF NOT EXISTS attendance (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    student_id      INTEGER NOT NULL REFERENCES students(id),
    class_id        INTEGER NOT NULL REFERENCES classes(id),
    attendance_date TEXT    NOT NULL,
    session         TEXT    NOT NULL CHECK (session IN ('MORNING', 'AFTERNOON')),
    status          TEXT    NOT NULL CHECK (status IN ('PRESENT', 'ABSENT')),
    recorded_by     INTEGER NOT NULL REFERENCES users(id),
    UNIQUE (student_id, class_id, attendance_date, session)
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    actor_id   INTEGER NOT NULL REFERENCES users(id),
    action     TEXT    NOT NULL,
    entity_id  INTEGER NOT NULL,
    detail     TEXT    NOT NULL DEFAULT '',
    created_at TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_attendance_class_date ON attendance (class_id, attendance_date);
