package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) *store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "db.sqlite")
	s, err := openStore(path)
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSchemaApplies(t *testing.T) {
	s := newTestStore(t)
	var v int
	if err := s.db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != currentSchemaVersion {
		t.Fatalf("schema_version=%d, want %d", v, currentSchemaVersion)
	}
}

func TestForeignKeysEnforced(t *testing.T) {
	s := newTestStore(t)
	_, err := s.db.Exec(`INSERT INTO tickets(project_id, num, title, status, created_at, updated_at) VALUES (999, 1, 't', 'open', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	if err == nil {
		t.Fatal("expected FK violation")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "foreign") {
		t.Fatalf("expected FK error, got: %v", err)
	}
}

func TestStatusCheckEnforced(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.db.Exec(`INSERT INTO projects(name, created_at) VALUES ('p','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	_, err := s.db.Exec(`INSERT INTO tickets(project_id, num, title, status, created_at, updated_at) VALUES (1, 1, 't', 'bogus', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	if err == nil {
		t.Fatal("expected CHECK violation")
	}
}

func TestSelfLinkBlocked(t *testing.T) {
	s := newTestStore(t)
	mustExec(t, s, `INSERT INTO projects(name, created_at) VALUES ('p','2026-01-01T00:00:00Z')`)
	mustExec(t, s, `INSERT INTO tickets(project_id, num, title, status, created_at, updated_at) VALUES (1, 1, 't', 'open', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	if _, err := s.db.Exec(`INSERT INTO ticket_links(from_id, to_id, type) VALUES (1, 1, 'related')`); err == nil {
		t.Fatal("expected self-link CHECK violation")
	}
}

func TestFTSPopulatedByTrigger(t *testing.T) {
	s := newTestStore(t)
	mustExec(t, s, `INSERT INTO projects(name, created_at) VALUES ('p','2026-01-01T00:00:00Z')`)
	mustExec(t, s, `INSERT INTO tickets(project_id, num, title, body, status, created_at, updated_at) VALUES (1, 1, 'fixme', 'something something widget', 'open', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM tickets_fts WHERE tickets_fts MATCH 'widget'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("FTS hit count=%d, want 1", n)
	}
}

func mustExec(t *testing.T, s *store, sql string, args ...any) {
	t.Helper()
	if _, err := s.db.Exec(sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}
