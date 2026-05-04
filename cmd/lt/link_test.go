package main

import (
	"errors"
	"testing"
)

func seedLinkTickets(t *testing.T, s *store, project string, n int) []int64 {
	t.Helper()
	seedProject(t, s, project)
	out := make([]int64, n)
	for i := 0; i < n; i++ {
		tk, err := s.createTicket(project, "title", "")
		if err != nil {
			t.Fatalf("createTicket: %v", err)
		}
		out[i] = tk.ID
	}
	return out
}

func linkRow(t *testing.T, s *store, fromNum, toNum int64) (string, bool) {
	t.Helper()
	var typ string
	err := s.db.QueryRow(`
		SELECT l.type
		FROM ticket_links l
		JOIN tickets f ON f.id = l.from_id
		JOIN tickets tt ON tt.id = l.to_id
		WHERE f.num = ? AND tt.num = ?`, fromNum, toNum).Scan(&typ)
	if err != nil {
		return "", false
	}
	return typ, true
}

func TestAddLink_Blocks(t *testing.T) {
	s := newTestStore(t)
	ids := seedLinkTickets(t, s, "p", 2)
	if err := s.addLink("p", ids[0], ids[1], "blocks"); err != nil {
		t.Fatalf("addLink: %v", err)
	}
	typ, ok := linkRow(t, s, ids[0], ids[1])
	if !ok {
		t.Fatal("expected link from #1 to #2")
	}
	if typ != "blocks" {
		t.Fatalf("type=%q want blocks", typ)
	}
}

func TestAddLink_BlockedBy_Swaps(t *testing.T) {
	s := newTestStore(t)
	ids := seedLinkTickets(t, s, "p", 2)
	if err := s.addLink("p", ids[0], ids[1], "blocked-by"); err != nil {
		t.Fatalf("addLink: %v", err)
	}
	if _, ok := linkRow(t, s, ids[0], ids[1]); ok {
		t.Fatal("expected stored row swapped, but found from=#1 to=#2")
	}
	typ, ok := linkRow(t, s, ids[1], ids[0])
	if !ok {
		t.Fatal("expected stored row from=#2 to=#1")
	}
	if typ != "blocks" {
		t.Fatalf("type=%q want blocks", typ)
	}
}

func TestAddLink_Parent(t *testing.T) {
	s := newTestStore(t)
	ids := seedLinkTickets(t, s, "p", 2)
	if err := s.addLink("p", ids[0], ids[1], "parent"); err != nil {
		t.Fatalf("addLink: %v", err)
	}
	typ, ok := linkRow(t, s, ids[0], ids[1])
	if !ok || typ != "parent" {
		t.Fatalf("got typ=%q ok=%v want parent/true", typ, ok)
	}
}

func TestAddLink_Child_Swaps(t *testing.T) {
	s := newTestStore(t)
	ids := seedLinkTickets(t, s, "p", 2)
	if err := s.addLink("p", ids[0], ids[1], "child"); err != nil {
		t.Fatalf("addLink: %v", err)
	}
	if _, ok := linkRow(t, s, ids[0], ids[1]); ok {
		t.Fatal("expected swapped storage")
	}
	typ, ok := linkRow(t, s, ids[1], ids[0])
	if !ok || typ != "parent" {
		t.Fatalf("got typ=%q ok=%v want parent/true", typ, ok)
	}
}

func TestAddLink_DuplicateOf(t *testing.T) {
	s := newTestStore(t)
	ids := seedLinkTickets(t, s, "p", 2)
	if err := s.addLink("p", ids[0], ids[1], "duplicate-of"); err != nil {
		t.Fatalf("addLink: %v", err)
	}
	typ, ok := linkRow(t, s, ids[0], ids[1])
	if !ok || typ != "duplicate-of" {
		t.Fatalf("got typ=%q ok=%v", typ, ok)
	}
}

func TestAddLink_Related(t *testing.T) {
	s := newTestStore(t)
	ids := seedLinkTickets(t, s, "p", 2)
	if err := s.addLink("p", ids[0], ids[1], "related"); err != nil {
		t.Fatalf("addLink: %v", err)
	}
	typ, ok := linkRow(t, s, ids[0], ids[1])
	if !ok || typ != "related" {
		t.Fatalf("got typ=%q ok=%v", typ, ok)
	}
}

func TestAddLink_BadType(t *testing.T) {
	s := newTestStore(t)
	ids := seedLinkTickets(t, s, "p", 2)
	err := s.addLink("p", ids[0], ids[1], "bogus")
	if err == nil {
		t.Fatal("expected bad_type error")
	}
	var ce *cmdError
	if !errors.As(err, &ce) || ce.code != "bad_type" {
		t.Fatalf("got %v code=%q want bad_type", err, codeOf(err))
	}
}

func TestAddLink_SelfLink(t *testing.T) {
	s := newTestStore(t)
	ids := seedLinkTickets(t, s, "p", 1)
	err := s.addLink("p", ids[0], ids[0], "blocks")
	if err == nil {
		t.Fatal("expected self_link error")
	}
	if codeOf(err) != "self_link" {
		t.Fatalf("got %v code=%q want self_link", err, codeOf(err))
	}
}

func TestAddLink_DuplicateConflict(t *testing.T) {
	s := newTestStore(t)
	ids := seedLinkTickets(t, s, "p", 2)
	if err := s.addLink("p", ids[0], ids[1], "blocks"); err != nil {
		t.Fatal(err)
	}
	err := s.addLink("p", ids[0], ids[1], "blocks")
	if err == nil {
		t.Fatal("expected link_exists conflict")
	}
	if codeOf(err) != "link_exists" {
		t.Fatalf("got %v code=%q want link_exists", err, codeOf(err))
	}
	// Adding via the inverse alias should also collide because canonical row matches.
	err2 := s.addLink("p", ids[1], ids[0], "blocked-by")
	if err2 == nil || codeOf(err2) != "link_exists" {
		t.Fatalf("inverse alias: got %v code=%q want link_exists", err2, codeOf(err2))
	}
}

func TestAddLink_BumpsBothUpdatedAt(t *testing.T) {
	s := newTestStore(t)
	ids := seedLinkTickets(t, s, "p", 2)
	mustExec(t, s, `UPDATE tickets SET updated_at = ?`, "2020-01-01T00:00:00Z")
	if err := s.addLink("p", ids[0], ids[1], "blocks"); err != nil {
		t.Fatal(err)
	}
	a, err := s.getTicket("p", ids[0])
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.getTicket("p", ids[1])
	if err != nil {
		t.Fatal(err)
	}
	if a.UpdatedAt == "2020-01-01T00:00:00Z" {
		t.Fatal("source updated_at not bumped")
	}
	if b.UpdatedAt == "2020-01-01T00:00:00Z" {
		t.Fatal("target updated_at not bumped")
	}
}

func TestAddLink_TicketNotFound(t *testing.T) {
	s := newTestStore(t)
	ids := seedLinkTickets(t, s, "p", 1)
	err := s.addLink("p", ids[0], 999, "blocks")
	if err == nil {
		t.Fatal("expected not_found")
	}
	if codeOf(err) != "not_found" {
		t.Fatalf("got code=%q want not_found", codeOf(err))
	}
}

func TestRemoveLink_EitherDirection(t *testing.T) {
	s := newTestStore(t)
	ids := seedLinkTickets(t, s, "p", 2)
	if err := s.addLink("p", ids[0], ids[1], "blocks"); err != nil {
		t.Fatal(err)
	}
	// Remove with arguments swapped relative to canonical storage direction.
	if err := s.removeLink("p", ids[1], ids[0]); err != nil {
		t.Fatalf("removeLink swapped: %v", err)
	}
	if _, ok := linkRow(t, s, ids[0], ids[1]); ok {
		t.Fatal("link still present after remove")
	}
}

func TestRemoveLink_NotFound(t *testing.T) {
	s := newTestStore(t)
	ids := seedLinkTickets(t, s, "p", 2)
	err := s.removeLink("p", ids[0], ids[1])
	if err == nil {
		t.Fatal("expected not_found")
	}
	if codeOf(err) != "not_found" {
		t.Fatalf("got code=%q want not_found", codeOf(err))
	}
}

func TestRemoveLink_BumpsBothUpdatedAt(t *testing.T) {
	s := newTestStore(t)
	ids := seedLinkTickets(t, s, "p", 2)
	if err := s.addLink("p", ids[0], ids[1], "blocks"); err != nil {
		t.Fatal(err)
	}
	mustExec(t, s, `UPDATE tickets SET updated_at = ?`, "2020-01-01T00:00:00Z")
	if err := s.removeLink("p", ids[0], ids[1]); err != nil {
		t.Fatal(err)
	}
	a, err := s.getTicket("p", ids[0])
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.getTicket("p", ids[1])
	if err != nil {
		t.Fatal(err)
	}
	if a.UpdatedAt == "2020-01-01T00:00:00Z" || b.UpdatedAt == "2020-01-01T00:00:00Z" {
		t.Fatalf("updated_at not bumped: a=%q b=%q", a.UpdatedAt, b.UpdatedAt)
	}
}

func TestLoadLinks_InverseDisplayOnReceiver(t *testing.T) {
	s := newTestStore(t)
	ids := seedLinkTickets(t, s, "p", 2)
	if err := s.addLink("p", ids[0], ids[1], "blocks"); err != nil {
		t.Fatal(err)
	}
	target, err := s.getTicket("p", ids[1])
	if err != nil {
		t.Fatal(err)
	}
	if len(target.Links) != 1 {
		t.Fatalf("links=%+v want 1", target.Links)
	}
	if target.Links[0].Type != "blocked-by" || target.Links[0].Target != ids[0] {
		t.Fatalf("got %+v want blocked-by #%d", target.Links[0], ids[0])
	}
	source, err := s.getTicket("p", ids[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(source.Links) != 1 || source.Links[0].Type != "blocks" || source.Links[0].Target != ids[1] {
		t.Fatalf("source links=%+v", source.Links)
	}
}

func codeOf(err error) string {
	var ce *cmdError
	if errors.As(err, &ce) {
		return ce.code
	}
	return ""
}
