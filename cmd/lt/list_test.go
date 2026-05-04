package main

import (
	"testing"
)

func seedProject(t *testing.T, s *store, name string) {
	t.Helper()
	if _, err := s.createProject(name); err != nil {
		t.Fatalf("createProject %q: %v", name, err)
	}
}

func seedTicket(t *testing.T, s *store, project, title, status, updatedAt string, labels ...string) int64 {
	t.Helper()
	tk, err := s.createTicket(project, title, "")
	if err != nil {
		t.Fatalf("createTicket: %v", err)
	}
	if status != "open" {
		mustExec(t, s, `UPDATE tickets SET status = ?, updated_at = ? WHERE id = ?`, status, updatedAt, tk.internalID)
	} else if updatedAt != "" {
		mustExec(t, s, `UPDATE tickets SET updated_at = ? WHERE id = ?`, updatedAt, tk.internalID)
	}
	for _, l := range labels {
		mustExec(t, s, `INSERT INTO ticket_labels(ticket_id, label) VALUES (?, ?)`, tk.internalID, l)
	}
	return tk.ID
}

func ticketNums(ts []*ticket) []int64 {
	out := make([]int64, len(ts))
	for i, t := range ts {
		out[i] = t.ID
	}
	return out
}

func TestListTickets_EmptyProject(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	got, err := s.listTickets("p", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d tickets, want 0", len(got))
	}
}

func TestListTickets_DefaultExcludesClosed(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	seedTicket(t, s, "p", "a", "open", "2026-01-01T00:00:00Z")
	seedTicket(t, s, "p", "b", "in-progress", "2026-01-02T00:00:00Z")
	seedTicket(t, s, "p", "c", "closed", "2026-01-03T00:00:00Z")

	got, err := s.listTickets("p", []string{"open", "in-progress"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
	for _, tk := range got {
		if tk.Status == "closed" {
			t.Errorf("default filter included closed ticket #%d", tk.ID)
		}
	}
}

func TestListTickets_StatusClosed(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	seedTicket(t, s, "p", "a", "open", "2026-01-01T00:00:00Z")
	seedTicket(t, s, "p", "b", "closed", "2026-01-02T00:00:00Z")
	seedTicket(t, s, "p", "c", "closed", "2026-01-03T00:00:00Z")

	got, err := s.listTickets("p", []string{"closed"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
	for _, tk := range got {
		if tk.Status != "closed" {
			t.Errorf("non-closed ticket leaked: #%d status=%s", tk.ID, tk.Status)
		}
	}
}

func TestListTickets_StatusAll(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	seedTicket(t, s, "p", "a", "open", "2026-01-01T00:00:00Z")
	seedTicket(t, s, "p", "b", "in-progress", "2026-01-02T00:00:00Z")
	seedTicket(t, s, "p", "c", "closed", "2026-01-03T00:00:00Z")

	got, err := s.listTickets("p", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d, want 3", len(got))
	}
}

func TestListTickets_LabelAND(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	seedTicket(t, s, "p", "only-foo", "open", "2026-01-01T00:00:00Z", "foo")
	seedTicket(t, s, "p", "only-bar", "open", "2026-01-02T00:00:00Z", "bar")
	bothID := seedTicket(t, s, "p", "both", "open", "2026-01-03T00:00:00Z", "foo", "bar")
	seedTicket(t, s, "p", "neither", "open", "2026-01-04T00:00:00Z")

	got, err := s.listTickets("p", []string{"open", "in-progress"}, []string{"foo", "bar"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d tickets, want 1: %+v", len(got), ticketNums(got))
	}
	if got[0].ID != bothID {
		t.Errorf("got ticket #%d, want #%d", got[0].ID, bothID)
	}
}

func TestListTickets_Ordering(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	id1 := seedTicket(t, s, "p", "first", "open", "2026-01-01T00:00:00Z")
	id2 := seedTicket(t, s, "p", "second", "open", "2026-01-03T00:00:00Z")
	id3 := seedTicket(t, s, "p", "third", "open", "2026-01-02T00:00:00Z")
	id4 := seedTicket(t, s, "p", "fourth", "open", "2026-01-03T00:00:00Z")

	got, err := s.listTickets("p", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []int64{id4, id2, id3, id1}
	gotIDs := ticketNums(got)
	if len(gotIDs) != len(want) {
		t.Fatalf("got %d tickets, want %d", len(gotIDs), len(want))
	}
	for i := range want {
		if gotIDs[i] != want[i] {
			t.Errorf("position %d: got #%d, want #%d (full order: %v)", i, gotIDs[i], want[i], gotIDs)
		}
	}
}
