// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// v1SchemaSQL is a frozen copy of the v1 schema, used to synthesize "old DBs"
// for migration tests. Do not modify when the production schemaSQL evolves;
// future versions should add their own vNSchemaSQL if a test needs that
// starting point.
const v1SchemaSQL = `
CREATE TABLE IF NOT EXISTS schema_version (
  version INTEGER PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS projects (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  name            TEXT NOT NULL UNIQUE,
  next_ticket_num INTEGER NOT NULL DEFAULT 1,
  created_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tickets (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id  INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  num         INTEGER NOT NULL,
  title       TEXT NOT NULL,
  body        TEXT NOT NULL DEFAULT '',
  status      TEXT NOT NULL CHECK (status IN ('open','in-progress','closed')),
  created_at  TEXT NOT NULL,
  updated_at  TEXT NOT NULL,
  closed_at   TEXT,
  UNIQUE (project_id, num)
);

CREATE INDEX IF NOT EXISTS idx_tickets_project_status ON tickets(project_id, status);
CREATE INDEX IF NOT EXISTS idx_tickets_listing       ON tickets(project_id, updated_at DESC, num DESC);

CREATE TABLE IF NOT EXISTS ticket_labels (
  ticket_id INTEGER NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
  label     TEXT NOT NULL,
  UNIQUE (ticket_id, label)
);
CREATE INDEX IF NOT EXISTS idx_ticket_labels_label ON ticket_labels(label);

CREATE TABLE IF NOT EXISTS ticket_links (
  from_id INTEGER NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
  to_id   INTEGER NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
  type    TEXT NOT NULL CHECK (type IN ('blocks','parent','duplicate-of','related')),
  UNIQUE (from_id, to_id, type),
  CHECK (from_id != to_id)
);
CREATE INDEX IF NOT EXISTS idx_links_to ON ticket_links(to_id);

CREATE VIRTUAL TABLE IF NOT EXISTS tickets_fts USING fts5 (
  title, body,
  content='tickets',
  content_rowid='id'
);

CREATE TRIGGER IF NOT EXISTS tickets_ai AFTER INSERT ON tickets BEGIN
  INSERT INTO tickets_fts(rowid, title, body) VALUES (new.id, new.title, new.body);
END;
CREATE TRIGGER IF NOT EXISTS tickets_ad AFTER DELETE ON tickets BEGIN
  INSERT INTO tickets_fts(tickets_fts, rowid, title, body) VALUES ('delete', old.id, old.title, old.body);
END;
CREATE TRIGGER IF NOT EXISTS tickets_au AFTER UPDATE ON tickets BEGIN
  INSERT INTO tickets_fts(tickets_fts, rowid, title, body) VALUES ('delete', old.id, old.title, old.body);
  INSERT INTO tickets_fts(rowid, title, body) VALUES (new.id, new.title, new.body);
END;
`

// synthesizeDB writes the given schema and records the given version, then
// closes the raw connection. Returns the DB path so the caller can re-open
// through the production openStore path.
func synthesizeDB(t *testing.T, schema string, recordedVersion int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "synth.sqlite")
	raw, err := sql.Open("sqlite", buildDSN(path))
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	if _, err := raw.Exec(schema); err != nil {
		raw.Close()
		t.Fatalf("apply schema: %v", err)
	}
	if _, err := raw.Exec(`INSERT INTO schema_version(version) VALUES (?)`, recordedVersion); err != nil {
		raw.Close()
		t.Fatalf("record version: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close raw: %v", err)
	}
	return path
}

func TestEnsureSchema_FromExistingV1(t *testing.T) {
	path := synthesizeDB(t, v1SchemaSQL, 1)

	s, err := openStore(path)
	if err != nil {
		t.Fatalf("re-open synthesized v1 DB: %v", err)
	}
	defer s.Close()

	var v int
	if err := s.db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != currentSchemaVersion {
		t.Fatalf("after re-open, version=%d, want %d", v, currentSchemaVersion)
	}
}

func TestEnsureSchema_RefusesNewerVersion(t *testing.T) {
	path := synthesizeDB(t, v1SchemaSQL, currentSchemaVersion+10)

	if _, err := openStore(path); err == nil {
		t.Fatal("expected error on newer schema_version, got nil")
	}
}

func TestRunMigrations_AppliesCustomMigrations(t *testing.T) {
	path := synthesizeDB(t, v1SchemaSQL, 1)
	db, err := sql.Open("sqlite", buildDSN(path))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	migs := []func(*sql.Tx) error{
		nil, // index 0
		nil, // index 1, v1
		func(tx *sql.Tx) error { // index 2, v2
			_, err := tx.Exec(`CREATE TABLE migration_marker (id INTEGER)`)
			return err
		},
		func(tx *sql.Tx) error { // index 3, v3
			_, err := tx.Exec(`INSERT INTO migration_marker(id) VALUES (42)`)
			return err
		},
	}
	if err := runMigrations(db, 1, 3, migs); err != nil {
		t.Fatalf("runMigrations: %v", err)
	}

	var v int
	if err := db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != 3 {
		t.Fatalf("schema_version=%d, want 3", v)
	}
	var marker int
	if err := db.QueryRow(`SELECT id FROM migration_marker`).Scan(&marker); err != nil {
		t.Fatal(err)
	}
	if marker != 42 {
		t.Fatalf("migration_marker.id=%d, want 42", marker)
	}
}

func TestRunMigrations_RollsBackOnFailure(t *testing.T) {
	path := synthesizeDB(t, v1SchemaSQL, 1)
	db, err := sql.Open("sqlite", buildDSN(path))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	failingErr := errors.New("intentional migration failure")
	migs := []func(*sql.Tx) error{
		nil, // 0
		nil, // 1, v1
		func(tx *sql.Tx) error { // 2, v2
			if _, err := tx.Exec(`CREATE TABLE rollback_marker (id INTEGER)`); err != nil {
				return err
			}
			return failingErr
		},
	}
	err = runMigrations(db, 1, 2, migs)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, failingErr) {
		t.Fatalf("error doesn't wrap failingErr: %v", err)
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='rollback_marker'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("rollback_marker should not exist after failed migration, count=%d", n)
	}

	var v int
	if err := db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != 1 {
		t.Fatalf("version after failed migration=%d, want 1", v)
	}
}

func TestRunMigrations_ErrorsOnMissingStep(t *testing.T) {
	path := synthesizeDB(t, v1SchemaSQL, 1)
	db, err := sql.Open("sqlite", buildDSN(path))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	migs := []func(*sql.Tx) error{nil, nil} // length 2: indices 0 and 1, no v2 entry
	if err := runMigrations(db, 1, 2, migs); err == nil {
		t.Fatal("expected error for missing migration step")
	}
}
