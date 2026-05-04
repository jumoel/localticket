package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSummarize_EmptyStore(t *testing.T) {
	s := newTestStore(t)
	sum, err := s.summarize()
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Projects) != 0 || len(sum.Top) != 0 {
		t.Fatalf("expected empty summary, got %+v", sum)
	}
	if sum.Totals.Projects != 0 || sum.Totals.Open != 0 {
		t.Errorf("totals not zero: %+v", sum.Totals)
	}
}

func TestSummarize_OrdersProjectsByMostRecent(t *testing.T) {
	s := newTestStore(t)
	mustExec(t, s, `INSERT INTO projects(name, created_at) VALUES ('alpha', '2026-01-01T00:00:00Z'), ('beta', '2026-01-01T00:00:00Z'), ('gamma', '2026-01-01T00:00:00Z')`)
	mustExec(t, s, `INSERT INTO tickets(project_id, num, title, status, created_at, updated_at) VALUES
		(1, 1, 'a', 'open', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(2, 1, 'b', 'open', '2026-01-01T00:00:00Z', '2026-03-01T00:00:00Z')`)
	sum, err := s.summarize()
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Projects) != 3 {
		t.Fatalf("got %d projects", len(sum.Projects))
	}
	if sum.Projects[0].Name != "beta" {
		t.Errorf("expected beta first (most recent ticket), got %s", sum.Projects[0].Name)
	}
	if sum.Projects[2].Name != "gamma" {
		t.Errorf("expected gamma last (no tickets), got %s", sum.Projects[2].Name)
	}
}

func TestSummarize_TopExcludesClosed(t *testing.T) {
	s := newTestStore(t)
	mustExec(t, s, `INSERT INTO projects(name, created_at) VALUES ('p', '2026-01-01T00:00:00Z')`)
	mustExec(t, s, `INSERT INTO tickets(project_id, num, title, status, created_at, updated_at) VALUES
		(1, 1, 'open ticket',   'open',        '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(1, 2, 'closed ticket', 'closed',      '2026-01-01T00:00:00Z', '2026-02-01T00:00:00Z'),
		(1, 3, 'in progress',   'in-progress', '2026-01-01T00:00:00Z', '2026-01-15T00:00:00Z')`)
	sum, err := s.summarize()
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Top) != 2 {
		t.Fatalf("got %d top tickets, want 2 (closed excluded)", len(sum.Top))
	}
	if sum.Top[0].ID != 3 {
		t.Errorf("expected #3 (most recently updated open) first, got #%d", sum.Top[0].ID)
	}
}

func TestSwiftbarOutput(t *testing.T) {
	s := newTestStore(t)
	mustExec(t, s, `INSERT INTO projects(name, created_at) VALUES ('demo', '2026-01-01T00:00:00Z')`)
	mustExec(t, s, `INSERT INTO tickets(project_id, num, title, status, created_at, updated_at) VALUES
		(1, 1, 'pending work', 'open', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	sum, err := s.summarize()
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := renderSwiftbar(&buf, sum); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 5 {
		t.Fatalf("expected at least 5 lines, got %d:\n%s", len(lines), out)
	}
	if !strings.HasPrefix(lines[0], "1 |") {
		t.Errorf("title line want active count, got %q", lines[0])
	}
	if !strings.Contains(lines[0], "sfimage=list.bullet.clipboard") {
		t.Errorf("title missing sfimage param, got %q", lines[0])
	}
	if !strings.Contains(out, "demo: 1 open") {
		t.Errorf("project row missing, got:\n%s", out)
	}
	if !strings.Contains(out, "Refresh | refresh=true") {
		t.Errorf("missing refresh row")
	}
}

func TestSwiftbarOutput_EmptyStore(t *testing.T) {
	s := newTestStore(t)
	sum, err := s.summarize()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := renderSwiftbar(&buf, sum); err != nil {
		t.Fatal(err)
	}
	first := strings.SplitN(buf.String(), "\n", 2)[0]
	if !strings.HasPrefix(first, " | sfimage=") {
		t.Errorf("expected zero-count title with leading space, got %q", first)
	}
	if !strings.Contains(buf.String(), "No projects") {
		t.Errorf("expected 'No projects' in dropdown")
	}
}

func TestSwiftbarOutput_ProjectCap(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < swiftbarProjectCap+3; i++ {
		mustExec(t, s, `INSERT INTO projects(name, created_at) VALUES (?, '2026-01-01T00:00:00Z')`,
			"p"+string(rune('a'+i)))
	}
	sum, err := s.summarize()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := renderSwiftbar(&buf, sum); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "and 3 more") {
		t.Errorf("expected '... and 3 more' line, got:\n%s", buf.String())
	}
}

func TestSwiftbarSubmenu(t *testing.T) {
	s := newTestStore(t)
	mustExec(t, s, `INSERT INTO projects(name, created_at) VALUES ('demo', '2026-01-01T00:00:00Z')`)
	mustExec(t, s, `INSERT INTO tickets(project_id, num, title, body, status, created_at, updated_at) VALUES
		(1, 1, 'first',   'line one' || char(10) || 'line two', 'open',        '2026-01-01T00:00:00Z', '2026-02-01T00:00:00Z'),
		(1, 2, 'blocker', '',                                   'in-progress', '2026-01-01T00:00:00Z', '2026-01-15T00:00:00Z')`)
	mustExec(t, s, `INSERT INTO ticket_labels(ticket_id, label) VALUES (1, 'bug'), (1, 'refactor')`)
	mustExec(t, s, `INSERT INTO ticket_links(from_id, to_id, type) VALUES (2, 1, 'blocks')`)

	sum, err := s.summarize()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := renderSwiftbar(&buf, sum); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	for _, want := range []string{
		"demo#1  first | font=Menlo",
		"--Status: open | color=black colorDark=white",
		"--Labels: bug, refactor | color=black colorDark=white",
		"--Links: blocked-by #2 | color=black colorDark=white",
		"-----",
		"--line one | font=Menlo color=black colorDark=white",
		"--line two | font=Menlo color=black colorDark=white",
		"demo#2  blocker | font=Menlo",
		"--Links: blocks #1 | color=black colorDark=white",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
	// Empty body should NOT emit the inner separator.
	idx := strings.Index(out, "demo#2")
	if idx < 0 {
		t.Fatal("missing demo#2")
	}
	tail := out[idx:]
	if strings.Contains(tail[:strings.Index(tail, "Refresh")], "-----") {
		t.Errorf("blocker (empty body) should not emit body separator:\n%s", tail)
	}
}

func TestE2E_Summary_Swiftbar(t *testing.T) {
	setupHome(t)
	r := runCLI(t, "summary", "--swiftbar")
	if r.exit != 0 {
		t.Fatalf("exit=%d stderr=%s", r.exit, r.stderr)
	}
	if !strings.Contains(r.stdout, "sfimage=list.bullet.clipboard") {
		t.Errorf("missing sfimage in output:\n%s", r.stdout)
	}
}
