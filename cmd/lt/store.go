package main

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const currentSchemaVersion = 1

type store struct {
	db *sql.DB
}

func defaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".localticket", "db.sqlite"), nil
}

func openStore(path string) (*store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	dsn := buildDSN(path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	s := &store{db: db}
	if err := s.ensureSchema(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func buildDSN(path string) string {
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(wal)")
	q.Add("_pragma", "busy_timeout(5000)")
	return "file:" + path + "?" + q.Encode()
}

func (s *store) Close() error { return s.db.Close() }

func (s *store) ensureSchema() error {
	if _, err := s.db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}

	var v sql.NullInt64
	if err := s.db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&v); err != nil {
		return fmt.Errorf("read schema_version: %w", err)
	}
	if !v.Valid {
		// Fresh DB: schemaSQL just created the current schema. Record it as such.
		if _, err := s.db.Exec(`INSERT INTO schema_version(version) VALUES (?)`, currentSchemaVersion); err != nil {
			return fmt.Errorf("set schema_version: %w", err)
		}
		return nil
	}
	recorded := int(v.Int64)
	if recorded > currentSchemaVersion {
		return fmt.Errorf("database schema version %d is newer than this binary supports (%d)", recorded, currentSchemaVersion)
	}
	if recorded < currentSchemaVersion {
		return runMigrations(s.db, recorded, currentSchemaVersion, migrations)
	}
	return nil
}

const schemaSQL = `
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
