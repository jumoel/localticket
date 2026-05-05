// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseColumns_Default(t *testing.T) {
	cols, err := parseColumns("")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(cols, ",") != strings.Join(defaultColumns, ",") {
		t.Errorf("default columns=%v, want %v", cols, defaultColumns)
	}
}

func TestParseColumns_CustomList(t *testing.T) {
	cols, err := parseColumns("id, title , labels")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"id", "title", "labels"}
	if strings.Join(cols, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", cols, want)
	}
}

func TestParseColumns_UnknownColumn(t *testing.T) {
	_, err := parseColumns("id,bogus")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var ce *cmdError
	if !asCmdError(err, &ce) || ce.code != "bad_column" {
		t.Errorf("expected bad_column, got %+v", err)
	}
}

func asCmdError(err error, target **cmdError) bool {
	if err == nil {
		return false
	}
	if ce, ok := err.(*cmdError); ok {
		*target = ce
		return true
	}
	return false
}

func TestRenderTicketTable_RespectsColumns(t *testing.T) {
	tickets := []*ticket{
		{ID: 1, Title: "first", Status: "open", Labels: []string{"bug"}, UpdatedAt: "2026-05-05T11:00:00Z"},
		{ID: 2, Title: "second", Status: "closed", Labels: nil, UpdatedAt: "2026-05-05T10:00:00Z"},
	}

	var buf bytes.Buffer
	if err := renderTicketTable(&buf, tickets, []string{"id", "title"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "ID") || !strings.Contains(out, "TITLE") {
		t.Errorf("missing headers: %q", out)
	}
	if strings.Contains(out, "STATUS") || strings.Contains(out, "LABELS") {
		t.Errorf("unexpected columns leaked: %q", out)
	}
	if !strings.Contains(out, "#1") || !strings.Contains(out, "#2") {
		t.Errorf("missing ids: %q", out)
	}
}

func TestRenderTicketTable_LinksAndClosed(t *testing.T) {
	closedAt := "2026-05-05T09:00:00Z"
	tickets := []*ticket{
		{
			ID:        3,
			Title:     "linked",
			Status:    "closed",
			Links:     []link{{Type: "blocks", Target: 4}},
			UpdatedAt: "2026-05-05T11:00:00Z",
			ClosedAt:  &closedAt,
		},
		{
			ID:        4,
			Title:     "still open",
			Status:    "open",
			UpdatedAt: "2026-05-05T11:00:00Z",
			ClosedAt:  nil,
		},
	}

	var buf bytes.Buffer
	if err := renderTicketTable(&buf, tickets, []string{"id", "links", "closed_at"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "blocks:#4") {
		t.Errorf("links not rendered: %q", out)
	}
	if !strings.Contains(out, "-") {
		t.Errorf("missing dash for nil closed_at: %q", out)
	}
}
