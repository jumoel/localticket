// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// TestMigrateV1ToV2_PreservesLinks confirms that a v1 DB containing existing
// links survives the migration to v2 with all rows intact and that the new
// CHECK constraint accepts the expanded set of types.
func TestMigrateV1ToV2_PreservesLinks(t *testing.T) {
	path := synthesizeDB(t, v1SchemaSQL, 1)

	raw, err := sql.Open("sqlite", buildDSN(path))
	if err != nil {
		t.Fatal(err)
	}
	must := func(stmt string) {
		t.Helper()
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatalf("seed: %s: %v", stmt, err)
		}
	}
	must(`INSERT INTO projects(name, created_at) VALUES ('p', '2026-01-01T00:00:00Z')`)
	must(`INSERT INTO tickets(project_id, num, title, status, created_at, updated_at) VALUES (1, 1, 'a', 'open', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	must(`INSERT INTO tickets(project_id, num, title, status, created_at, updated_at) VALUES (1, 2, 'b', 'open', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	must(`INSERT INTO ticket_links(from_id, to_id, type) VALUES (1, 2, 'blocks')`)
	must(`INSERT INTO ticket_links(from_id, to_id, type) VALUES (2, 1, 'related')`)
	raw.Close()

	s, err := openStore(path)
	if err != nil {
		t.Fatalf("re-open after migration: %v", err)
	}
	defer s.Close()

	var v int
	if err := s.db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != 2 {
		t.Fatalf("schema_version=%d, want 2", v)
	}

	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM ticket_links`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("link count after migration=%d, want 2", n)
	}

	// New types should now insert cleanly; under the v1 CHECK they would fail.
	if _, err := s.db.Exec(`INSERT INTO ticket_links(from_id, to_id, type) VALUES (1, 2, 'supersedes')`); err != nil {
		t.Errorf("supersedes should be valid in v2: %v", err)
	}
	if _, err := s.db.Exec(`INSERT INTO ticket_links(from_id, to_id, type) VALUES (1, 2, 'references')`); err != nil {
		t.Errorf("references should be valid in v2: %v", err)
	}
	if _, err := s.db.Exec(`INSERT INTO ticket_links(from_id, to_id, type) VALUES (1, 2, 'derived-from')`); err != nil {
		t.Errorf("derived-from should be valid in v2: %v", err)
	}

	// Old types still work.
	if _, err := s.db.Exec(`INSERT INTO ticket_links(from_id, to_id, type) VALUES (2, 1, 'parent')`); err != nil {
		t.Errorf("parent should still be valid: %v", err)
	}

	// Bogus types are still rejected.
	_, err = s.db.Exec(`INSERT INTO ticket_links(from_id, to_id, type) VALUES (1, 2, 'bogus')`)
	if err == nil {
		t.Error("bogus type should be rejected by CHECK")
	}
}

func TestNewLinkTypes_E2E(t *testing.T) {
	setupHome(t)
	if r := runCLI(t, "--json", "project", "create", "demo"); r.exit != 0 {
		t.Fatal(r.stderr)
	}
	if r := runCLI(t, "--json", "new", "-p", "demo", "a", "--body", "x"); r.exit != 0 {
		t.Fatal(r.stderr)
	}
	if r := runCLI(t, "--json", "new", "-p", "demo", "b", "--body", "y"); r.exit != 0 {
		t.Fatal(r.stderr)
	}

	// supersedes: #1 supersedes #2 → #2 sees superseded-by #1
	if r := runCLI(t, "--json", "link", "add", "-p", "demo", "1", "supersedes", "2"); r.exit != 0 {
		t.Fatalf("link add supersedes: %s", r.stderr)
	}
	r := runCLI(t, "--json", "show", "-p", "demo", "1")
	m := mustJSON(t, r.stdout)
	links, _ := m["links"].([]any)
	if len(links) != 1 {
		t.Fatalf("links=%v", links)
	}
	first, _ := links[0].(map[string]any)
	if first["type"] != "supersedes" {
		t.Errorf("type=%v, want supersedes", first["type"])
	}

	r = runCLI(t, "--json", "show", "-p", "demo", "2")
	m = mustJSON(t, r.stdout)
	links, _ = m["links"].([]any)
	first, _ = links[0].(map[string]any)
	if first["type"] != "superseded-by" {
		t.Errorf("inverse type=%v, want superseded-by", first["type"])
	}
}

func TestLinkList_Project(t *testing.T) {
	setupHome(t)
	if r := runCLI(t, "--json", "project", "create", "demo"); r.exit != 0 {
		t.Fatal(r.stderr)
	}
	for _, ttl := range []string{"a", "b", "c"} {
		if r := runCLI(t, "--json", "new", "-p", "demo", ttl, "--body", "x"); r.exit != 0 {
			t.Fatal(r.stderr)
		}
	}
	runCLI(t, "--json", "link", "add", "-p", "demo", "1", "blocks", "2")
	runCLI(t, "--json", "link", "add", "-p", "demo", "2", "references", "3")

	r := runCLI(t, "--json", "link", "list", "-p", "demo")
	if r.exit != 0 {
		t.Fatalf("link list: %s", r.stderr)
	}
	m := mustJSON(t, r.stdout)
	links, _ := m["links"].([]any)
	if len(links) != 2 {
		t.Errorf("links=%d, want 2: %v", len(links), links)
	}

	// Filter by ticket
	r = runCLI(t, "--json", "link", "list", "-p", "demo", "2")
	m = mustJSON(t, r.stdout)
	links, _ = m["links"].([]any)
	if len(links) != 2 {
		t.Errorf("ticket #2 links=%d, want 2 (one outgoing, one incoming)", len(links))
	}

	// Filter by type
	r = runCLI(t, "--json", "link", "list", "-p", "demo", "--type", "blocks")
	m = mustJSON(t, r.stdout)
	links, _ = m["links"].([]any)
	if len(links) != 1 {
		t.Fatalf("blocks links=%d, want 1", len(links))
	}
	first, _ := links[0].(map[string]any)
	if first["type"] != "blocks" {
		t.Errorf("type=%v", first["type"])
	}
}

func TestLinkList_BadType(t *testing.T) {
	setupHome(t)
	if r := runCLI(t, "--json", "project", "create", "demo"); r.exit != 0 {
		t.Fatal(r.stderr)
	}
	r := runCLI(t, "--json", "link", "list", "-p", "demo", "--type", "bogus")
	if r.exit != 1 {
		t.Errorf("exit=%d, want 1", r.exit)
	}
	m := mustJSON(t, r.stderr)
	if !strings.Contains(m["error"].(string), "invalid --type") {
		t.Errorf("error=%v", m["error"])
	}
}

func TestLinkList_ExcludeClosed(t *testing.T) {
	setupHome(t)
	if r := runCLI(t, "--json", "project", "create", "demo"); r.exit != 0 {
		t.Fatal(r.stderr)
	}
	for _, ttl := range []string{"a", "b"} {
		if r := runCLI(t, "--json", "new", "-p", "demo", ttl, "--body", "x"); r.exit != 0 {
			t.Fatal(r.stderr)
		}
	}
	runCLI(t, "--json", "link", "add", "-p", "demo", "1", "blocks", "2")
	runCLI(t, "--json", "close", "-p", "demo", "2")

	r := runCLI(t, "--json", "link", "list", "-p", "demo")
	m := mustJSON(t, r.stdout)
	links, _ := m["links"].([]any)
	if len(links) != 0 {
		t.Errorf("default-hide-closed links=%d, want 0", len(links))
	}

	r = runCLI(t, "--json", "link", "list", "-p", "demo", "--include-closed")
	m = mustJSON(t, r.stdout)
	links, _ = m["links"].([]any)
	if len(links) != 1 {
		t.Errorf("--include-closed links=%d, want 1", len(links))
	}
}
