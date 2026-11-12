// Package database opens the SQLite file and applies the schema.
package database

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"net/url"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// Open opens (and creates, if needed) the SQLite database at path.
func Open(path string) (*sql.DB, error) {
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	// SQLite allows a single writer; one connection keeps things simple and safe.
	db.SetMaxOpenConns(1)
	if err := db.PingContext(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return db, nil
}

// Backup writes a consistent snapshot of the live database to dest using VACUUM INTO
// and verifies the copy with an integrity check. dest must not already exist.
func Backup(db *sql.DB, dest string) error {
	if _, err := db.Exec(`VACUUM INTO ?`, dest); err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}
	snap, err := sql.Open("sqlite", "file:"+dest+"?mode=ro")
	if err != nil {
		return err
	}
	defer snap.Close()
	var res string
	if err := snap.QueryRow(`PRAGMA integrity_check`).Scan(&res); err != nil {
		return err
	}
	if res != "ok" {
		return fmt.Errorf("integrity check failed: %s", res)
	}
	return nil
}
