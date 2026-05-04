package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestAddLabels_Single(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	tk, err := s.createTicket("p", "t", "")
	if err != nil {
		t.Fatal(err)
	}
	added, err := s.addLabels("p", tk.ID, []string{"bug"})
	if err != nil {
		t.Fatalf("addLabels: %v", err)
	}
	if added != 1 {
		t.Fatalf("added=%d, want 1", added)
	}
	got, err := s.getTicket("p", tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Labels, []string{"bug"}) {
		t.Fatalf("labels=%v, want [bug]", got.Labels)
	}
}

func TestAddLabels_Multiple(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	tk, err := s.createTicket("p", "t", "")
	if err != nil {
		t.Fatal(err)
	}
	added, err := s.addLabels("p", tk.ID, []string{"bug", "p1", "ui"})
	if err != nil {
		t.Fatal(err)
	}
	if added != 3 {
		t.Fatalf("added=%d, want 3", added)
	}
	got, err := s.getTicket("p", tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Labels, []string{"bug", "p1", "ui"}) {
		t.Fatalf("labels=%v, want [bug p1 ui]", got.Labels)
	}
}

func TestAddLabels_DuplicateIsNoop(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	tk, err := s.createTicket("p", "t", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.addLabels("p", tk.ID, []string{"bug"}); err != nil {
		t.Fatal(err)
	}
	added, err := s.addLabels("p", tk.ID, []string{"bug", "p1"})
	if err != nil {
		t.Fatalf("addLabels: %v", err)
	}
	if added != 1 {
		t.Fatalf("added=%d, want 1 (only p1 is new)", added)
	}
	got, err := s.getTicket("p", tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Labels, []string{"bug", "p1"}) {
		t.Fatalf("labels=%v, want [bug p1]", got.Labels)
	}
}

func TestAddLabels_BumpsUpdatedAtOnlyWhenAdded(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	tk, err := s.createTicket("p", "t", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.addLabels("p", tk.ID, []string{"bug"}); err != nil {
		t.Fatal(err)
	}
	mustExec(t, s, `UPDATE tickets SET updated_at = ? WHERE id = ?`, "2020-01-01T00:00:00Z", tk.internalID)

	// Adding only the existing label is a no-op; updated_at must not bump.
	added, err := s.addLabels("p", tk.ID, []string{"bug"})
	if err != nil {
		t.Fatal(err)
	}
	if added != 0 {
		t.Fatalf("added=%d, want 0", added)
	}
	got, err := s.getTicket("p", tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.UpdatedAt != "2020-01-01T00:00:00Z" {
		t.Fatalf("updated_at bumped on no-op: %q", got.UpdatedAt)
	}

	// Adding a new label bumps updated_at.
	added, err = s.addLabels("p", tk.ID, []string{"p1"})
	if err != nil {
		t.Fatal(err)
	}
	if added != 1 {
		t.Fatalf("added=%d, want 1", added)
	}
	got, err = s.getTicket("p", tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.UpdatedAt == "2020-01-01T00:00:00Z" {
		t.Fatal("updated_at not bumped after real add")
	}
}

func TestRemoveLabels_Existing(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	tk, err := s.createTicket("p", "t", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.addLabels("p", tk.ID, []string{"bug", "p1"}); err != nil {
		t.Fatal(err)
	}
	removed, err := s.removeLabels("p", tk.ID, []string{"bug"})
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed=%d, want 1", removed)
	}
	got, err := s.getTicket("p", tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Labels, []string{"p1"}) {
		t.Fatalf("labels=%v, want [p1]", got.Labels)
	}
}

func TestRemoveLabels_MissingIsNotFoundAndAtomic(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	tk, err := s.createTicket("p", "t", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.addLabels("p", tk.ID, []string{"bug", "p1"}); err != nil {
		t.Fatal(err)
	}
	_, err = s.removeLabels("p", tk.ID, []string{"bug", "nope"})
	if err == nil {
		t.Fatal("expected error for missing label")
	}
	var ce *cmdError
	if !errors.As(err, &ce) || ce.code != "not_found" {
		t.Fatalf("error code=%v, want not_found: %v", err, err)
	}
	got, err := s.getTicket("p", tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Labels, []string{"bug", "p1"}) {
		t.Fatalf("labels=%v, want untouched [bug p1]", got.Labels)
	}
}

func TestRemoveLabels_BumpsUpdatedAtOnlyWhenRemoved(t *testing.T) {
	s := newTestStore(t)
	seedProject(t, s, "p")
	tk, err := s.createTicket("p", "t", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.addLabels("p", tk.ID, []string{"bug"}); err != nil {
		t.Fatal(err)
	}
	mustExec(t, s, `UPDATE tickets SET updated_at = ? WHERE id = ?`, "2020-01-01T00:00:00Z", tk.internalID)

	// Failed remove (label missing) must not bump updated_at.
	if _, err := s.removeLabels("p", tk.ID, []string{"nope"}); err == nil {
		t.Fatal("expected not_found")
	}
	got, err := s.getTicket("p", tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.UpdatedAt != "2020-01-01T00:00:00Z" {
		t.Fatalf("updated_at bumped on failed remove: %q", got.UpdatedAt)
	}

	// Successful remove bumps updated_at.
	if _, err := s.removeLabels("p", tk.ID, []string{"bug"}); err != nil {
		t.Fatal(err)
	}
	got, err = s.getTicket("p", tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.UpdatedAt == "2020-01-01T00:00:00Z" {
		t.Fatal("updated_at not bumped after real remove")
	}
}

func TestValidateLabel_RejectsInvalid(t *testing.T) {
	if err := validateLabel("Bad Label"); err == nil {
		t.Fatal("expected validateLabel to reject 'Bad Label'")
	}
}
