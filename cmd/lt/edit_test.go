package main

import (
	"testing"
	"time"
)

func TestSetTicketStatus_OpenToInProgressToClosedAndReopen(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	tk, err := s.createTicket("p", "title", "body")
	if err != nil {
		t.Fatalf("createTicket: %v", err)
	}
	num := tk.ID

	// open -> in-progress: bumps updated_at, no closed_at.
	mustExec(t, s, `UPDATE tickets SET updated_at = ? WHERE id = ?`, "2020-01-01T00:00:00Z", tk.internalID)
	changed, err := s.setTicketStatus("p", num, "in-progress")
	if err != nil {
		t.Fatalf("setTicketStatus in-progress: %v", err)
	}
	if !changed {
		t.Fatal("expected change open->in-progress")
	}
	got, err := s.getTicket("p", num)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "in-progress" {
		t.Fatalf("status=%q, want in-progress", got.Status)
	}
	if got.ClosedAt != nil {
		t.Fatalf("closed_at=%v, want nil", got.ClosedAt)
	}
	if got.UpdatedAt == "2020-01-01T00:00:00Z" {
		t.Fatal("updated_at not bumped")
	}

	// in-progress -> closed: closed_at set.
	changed, err = s.setTicketStatus("p", num, "closed")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected change in-progress->closed")
	}
	got, err = s.getTicket("p", num)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "closed" || got.ClosedAt == nil {
		t.Fatalf("got status=%q closed_at=%v", got.Status, got.ClosedAt)
	}
	if _, err := time.Parse(time.RFC3339, *got.ClosedAt); err != nil {
		t.Fatalf("closed_at not RFC3339: %v", err)
	}

	// closed -> closed: no-op.
	prevUpdated := got.UpdatedAt
	prevClosed := *got.ClosedAt
	changed, err = s.setTicketStatus("p", num, "closed")
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("expected no-op for already-closed")
	}
	got, err = s.getTicket("p", num)
	if err != nil {
		t.Fatal(err)
	}
	if got.UpdatedAt != prevUpdated {
		t.Fatalf("updated_at changed on no-op: %q -> %q", prevUpdated, got.UpdatedAt)
	}
	if got.ClosedAt == nil || *got.ClosedAt != prevClosed {
		t.Fatalf("closed_at changed on no-op: %q -> %v", prevClosed, got.ClosedAt)
	}

	// closed -> open: clears closed_at.
	changed, err = s.setTicketStatus("p", num, "open")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected change closed->open")
	}
	got, err = s.getTicket("p", num)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "open" {
		t.Fatalf("status=%q, want open", got.Status)
	}
	if got.ClosedAt != nil {
		t.Fatalf("closed_at=%v, want nil after reopen", got.ClosedAt)
	}
}

func TestSetTicketStatus_InvalidStatus(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	tk, err := s.createTicket("p", "t", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.setTicketStatus("p", tk.ID, "bogus"); err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestUpdateTicket_TitleOnly(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	tk, err := s.createTicket("p", "old", "body content")
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, s, `UPDATE tickets SET updated_at = ? WHERE id = ?`, "2020-01-01T00:00:00Z", tk.internalID)
	newTitle := "new title"
	changed, err := s.updateTicket("p", tk.ID, &newTitle, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected change")
	}
	got, err := s.getTicket("p", tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "new title" {
		t.Fatalf("title=%q", got.Title)
	}
	if got.Body != "body content" {
		t.Fatalf("body=%q, expected unchanged", got.Body)
	}
	if got.UpdatedAt == "2020-01-01T00:00:00Z" {
		t.Fatal("updated_at not bumped")
	}
}

func TestUpdateTicket_BodyOnly(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	tk, err := s.createTicket("p", "title", "old body")
	if err != nil {
		t.Fatal(err)
	}
	newBody := "new body"
	changed, err := s.updateTicket("p", tk.ID, nil, &newBody)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected change")
	}
	got, err := s.getTicket("p", tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "title" {
		t.Fatalf("title=%q, want unchanged", got.Title)
	}
	if got.Body != "new body" {
		t.Fatalf("body=%q", got.Body)
	}
}

func TestUpdateTicket_NoOp(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	tk, err := s.createTicket("p", "title", "body")
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, s, `UPDATE tickets SET updated_at = ? WHERE id = ?`, "2020-01-01T00:00:00Z", tk.internalID)
	sameTitle := "title"
	changed, err := s.updateTicket("p", tk.ID, &sameTitle, nil)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("expected no-op when title equals current")
	}
	got, err := s.getTicket("p", tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.UpdatedAt != "2020-01-01T00:00:00Z" {
		t.Fatalf("updated_at bumped on no-op: %q", got.UpdatedAt)
	}
}

func TestUpdateTicket_EmptyTitleRejected(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	tk, err := s.createTicket("p", "title", "")
	if err != nil {
		t.Fatal(err)
	}
	empty := "   "
	if _, err := s.updateTicket("p", tk.ID, &empty, nil); err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestUpdateTicket_NotFound(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	title := "x"
	if _, err := s.updateTicket("p", 999, &title, nil); err == nil {
		t.Fatal("expected not_found error")
	}
}
