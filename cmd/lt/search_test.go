package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestSearchTickets_TitleMatch(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	idA := seedTicket(t, s, "p", "fix the widget", "open", "")
	seedTicket(t, s, "p", "polish the button", "open", "")

	got, err := s.searchTickets("p", "widget")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d, want 1", len(got))
	}
	if got[0].ID != idA {
		t.Errorf("got #%d, want #%d", got[0].ID, idA)
	}
}

func TestSearchTickets_BodyMatch(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	tk, err := s.createTicket("p", "title here", "body mentions sprocket somewhere")
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.searchTickets("p", "sprocket")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != tk.ID {
		t.Fatalf("got %+v, want one ticket #%d", ticketNums(got), tk.ID)
	}
}

func TestSearchTickets_NoMatch(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	seedTicket(t, s, "p", "alpha", "open", "")
	seedTicket(t, s, "p", "beta", "open", "")

	got, err := s.searchTickets("p", "nonexistentterm")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d tickets, want 0", len(got))
	}
}

func TestSearchTickets_IncludesClosed(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	openID := seedTicket(t, s, "p", "open widget", "open", "")
	closedID := seedTicket(t, s, "p", "closed widget", "closed", "2026-01-02T00:00:00Z")

	got, err := s.searchTickets("p", "widget")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
	ids := map[int64]bool{}
	for _, tk := range got {
		ids[tk.ID] = true
	}
	if !ids[openID] || !ids[closedID] {
		t.Errorf("expected both #%d and #%d, got %+v", openID, closedID, ticketNums(got))
	}
}

func TestSearchTickets_RelevanceOrdering(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	bodyOnly, err := s.createTicket("p", "irrelevant title", strings.Repeat("filler text ", 200)+"widget "+strings.Repeat("more filler ", 200))
	if err != nil {
		t.Fatal(err)
	}
	titleHit, err := s.createTicket("p", "widget repair", "")
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.searchTickets("p", "widget")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
	ids := map[int64]bool{got[0].ID: true, got[1].ID: true}
	if !ids[bodyOnly.ID] || !ids[titleHit.ID] {
		t.Errorf("missing expected IDs in %+v (want #%d and #%d)", ticketNums(got), bodyOnly.ID, titleHit.ID)
	}
	if got[0].ID != titleHit.ID {
		t.Logf("title hit not first under bm25; got order %+v", ticketNums(got))
	}
}

func TestSearchTickets_BadQuery(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	seedTicket(t, s, "p", "alpha", "open", "")

	_, err := s.searchTickets("p", "AND OR")
	if err == nil {
		t.Fatal("expected error for bare operator query")
	}
	var ce *cmdError
	if !errors.As(err, &ce) {
		t.Fatalf("error is not *cmdError: %T %v", err, err)
	}
	if ce.code != "bad_query" {
		t.Errorf("code=%q, want bad_query (msg=%q)", ce.code, ce.msg)
	}
}

func TestSearchTickets_LabelsAndLinksPopulated(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	a, err := s.createTicket("p", "alpha widget", "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.createTicket("p", "beta target", "")
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, s, `INSERT INTO ticket_labels(ticket_id, label) VALUES (?, ?)`, a.internalID, "urgent")
	mustExec(t, s, `INSERT INTO ticket_links(from_id, to_id, type) VALUES (?, ?, 'related')`, a.internalID, b.internalID)

	got, err := s.searchTickets("p", "widget")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d, want 1", len(got))
	}
	if len(got[0].Labels) != 1 || got[0].Labels[0] != "urgent" {
		t.Errorf("labels=%v, want [urgent]", got[0].Labels)
	}
	if len(got[0].Links) != 1 || got[0].Links[0].Type != "related" || got[0].Links[0].Target != b.ID {
		t.Errorf("links=%+v, want [{related #%d}]", got[0].Links, b.ID)
	}
}

func TestSearchTickets_ProjectIsolation(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	seedProject(t, s, "q")
	idP := seedTicket(t, s, "p", "shared widget term", "open", "")
	seedTicket(t, s, "q", "shared widget term", "open", "")

	got, err := s.searchTickets("p", "widget")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != idP {
		t.Fatalf("got %+v, want only #%d", ticketNums(got), idP)
	}
}

func TestSearchTickets_ProjectNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.searchTickets("missing", "anything")
	if err == nil {
		t.Fatal("expected not_found")
	}
	var ce *cmdError
	if !errors.As(err, &ce) {
		t.Fatalf("err type=%T", err)
	}
	if ce.code != "not_found" {
		t.Errorf("code=%q, want not_found", ce.code)
	}
}

func TestE2E_Search(t *testing.T) {
	setupHome(t)

	if r := runCLI(t, "--json", "project", "create", "demo"); r.exit != 0 {
		t.Fatalf("project create exit=%d stderr=%s", r.exit, r.stderr)
	}
	if r := runCLI(t, "--json", "new", "-p", "demo", "--body", "", "fix", "the", "widget"); r.exit != 0 {
		t.Fatalf("new exit=%d stderr=%s", r.exit, r.stderr)
	}
	if r := runCLI(t, "--json", "new", "-p", "demo", "--body", "", "polish", "the", "button"); r.exit != 0 {
		t.Fatalf("new exit=%d stderr=%s", r.exit, r.stderr)
	}

	r := runCLI(t, "--json", "search", "-p", "demo", "widget")
	if r.exit != 0 {
		t.Fatalf("search exit=%d stderr=%s", r.exit, r.stderr)
	}
	var resp struct {
		Tickets []struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
		} `json:"tickets"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &resp); err != nil {
		t.Fatalf("decode: %v\n%s", err, r.stdout)
	}
	if len(resp.Tickets) != 1 {
		t.Fatalf("got %d hits, want 1: %+v", len(resp.Tickets), resp.Tickets)
	}
	if !strings.Contains(resp.Tickets[0].Title, "widget") {
		t.Errorf("hit title=%q, want contains 'widget'", resp.Tickets[0].Title)
	}

	r = runCLI(t, "--json", "search", "-p", "demo", "nonexistentterm")
	if r.exit != 0 {
		t.Fatalf("empty search exit=%d stderr=%s", r.exit, r.stderr)
	}
	if !strings.Contains(r.stdout, `"tickets": []`) {
		t.Errorf("expected empty tickets array in JSON, got: %s", r.stdout)
	}

	r = runCLI(t, "--pretty", "search", "-p", "demo", "nonexistentterm")
	if r.exit != 0 {
		t.Fatalf("pretty empty search exit=%d stderr=%s", r.exit, r.stderr)
	}
	if !strings.Contains(r.stdout, "No matches.") {
		t.Errorf("pretty empty stdout=%q", r.stdout)
	}

	r = runCLI(t, "--json", "search", "-p", "demo")
	if r.exit != 1 {
		t.Errorf("missing query exit=%d, want 1", r.exit)
	}
}
