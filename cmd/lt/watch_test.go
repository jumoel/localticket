package main

import (
	"strings"
	"testing"
)

func mkSnapshot(project string, num int64, status, title, body, createdAt, updatedAt string, labels []string, links []linkKey) *snapshot {
	s := &snapshot{
		Project: project, Num: num,
		Status: status, Title: title, Body: body,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
		Labels: map[string]bool{}, Links: map[linkKey]bool{},
	}
	for _, l := range labels {
		s.Labels[l] = true
	}
	for _, k := range links {
		s.Links[k] = true
	}
	return s
}

func actionsOf(events []watchEvent) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.Action
	}
	return out
}

func TestDiff_FirstSightingCreated(t *testing.T) {
	now := "2026-05-04T12:00:00Z"
	next := mkSnapshot("demo", 1, "open", "first", "x", now, now, nil, nil)
	got := diffSnapshot(nil, false, next, now)
	if want := []string{"created"}; !equalStrs(actionsOf(got), want) {
		t.Errorf("got %v, want %v", actionsOf(got), want)
	}
}

func TestDiff_FirstSightingPreExisting(t *testing.T) {
	next := mkSnapshot("demo", 1, "open", "first", "x", "2026-05-04T11:00:00Z", "2026-05-04T12:00:00Z", nil, nil)
	got := diffSnapshot(nil, false, next, next.UpdatedAt)
	if want := []string{"updated"}; !equalStrs(actionsOf(got), want) {
		t.Errorf("got %v, want %v", actionsOf(got), want)
	}
}

func TestDiff_StatusTransitions(t *testing.T) {
	cases := []struct {
		from, to, want string
	}{
		{"open", "in-progress", "status_changed"},
		{"open", "closed", "closed"},
		{"in-progress", "closed", "closed"},
		{"closed", "open", "reopened"},
		{"closed", "in-progress", "reopened"},
		{"in-progress", "open", "status_changed"},
	}
	for _, c := range cases {
		t.Run(c.from+"->"+c.to, func(t *testing.T) {
			prior := mkSnapshot("d", 1, c.from, "t", "b", "T", "T", nil, nil)
			next := mkSnapshot("d", 1, c.to, "t", "b", "T", "T2", nil, nil)
			got := diffSnapshot(prior, true, next, "now")
			if len(got) != 1 || got[0].Action != c.want {
				t.Errorf("got %v, want [%s]", actionsOf(got), c.want)
			}
		})
	}
}

func TestDiff_TitleAndBody(t *testing.T) {
	prior := mkSnapshot("d", 1, "open", "old title", "old body", "T", "T", nil, nil)
	next := mkSnapshot("d", 1, "open", "new title", "new body", "T", "T2", nil, nil)
	got := diffSnapshot(prior, true, next, "now")
	want := []string{"title_changed", "body_changed"}
	if !equalStrs(actionsOf(got), want) {
		t.Errorf("got %v, want %v", actionsOf(got), want)
	}
}

func TestDiff_LabelsAddedAndRemoved(t *testing.T) {
	prior := mkSnapshot("d", 1, "open", "t", "b", "T", "T", []string{"keep", "drop"}, nil)
	next := mkSnapshot("d", 1, "open", "t", "b", "T", "T2", []string{"keep", "new"}, nil)
	got := diffSnapshot(prior, true, next, "now")
	wantContains := map[string]bool{"label_added": false, "label_removed": false}
	for _, e := range got {
		if _, ok := wantContains[e.Action]; ok {
			wantContains[e.Action] = true
		}
		if e.Action == "label_added" && e.Label != "new" {
			t.Errorf("label_added has label=%q, want new", e.Label)
		}
		if e.Action == "label_removed" && e.Label != "drop" {
			t.Errorf("label_removed has label=%q, want drop", e.Label)
		}
	}
	for action, seen := range wantContains {
		if !seen {
			t.Errorf("missing %s in %v", action, actionsOf(got))
		}
	}
}

func TestDiff_LinksAddedAndRemoved(t *testing.T) {
	prior := mkSnapshot("d", 1, "open", "t", "b", "T", "T", nil, []linkKey{{Type: "blocks", Target: 2}})
	next := mkSnapshot("d", 1, "open", "t", "b", "T", "T2", nil, []linkKey{{Type: "blocks", Target: 3}})
	got := diffSnapshot(prior, true, next, "now")
	wantContains := map[string]bool{"link_added": false, "link_removed": false}
	for _, e := range got {
		if _, ok := wantContains[e.Action]; ok {
			wantContains[e.Action] = true
		}
	}
	for action, seen := range wantContains {
		if !seen {
			t.Errorf("missing %s in %v", action, actionsOf(got))
		}
	}
}

func TestDiff_NoObservableChange(t *testing.T) {
	prior := mkSnapshot("d", 1, "open", "t", "b", "T", "T", nil, nil)
	next := mkSnapshot("d", 1, "open", "t", "b", "T", "T2", nil, nil)
	got := diffSnapshot(prior, true, next, "now")
	if len(got) != 1 || got[0].Action != "updated" {
		t.Errorf("got %v, want [updated]", actionsOf(got))
	}
}

func TestE2E_Watch_OnceReplay(t *testing.T) {
	setupHome(t)
	if r := runCLI(t, "--json", "project", "create", "demo"); r.exit != 0 {
		t.Fatalf("project create: %s", r.stderr)
	}
	if r := runCLI(t, "--json", "new", "-p", "demo", "first", "--body", "x"); r.exit != 0 {
		t.Fatalf("new: %s", r.stderr)
	}
	if r := runCLI(t, "--json", "new", "-p", "demo", "second", "--body", "y"); r.exit != 0 {
		t.Fatalf("new: %s", r.stderr)
	}

	r := runCLI(t, "--json", "watch", "--once", "--since", "1970-01-01T00:00:00Z")
	if r.exit != 0 {
		t.Fatalf("watch exit=%d stderr=%s", r.exit, r.stderr)
	}
	if !strings.Contains(r.stdout, `"action": "created"`) {
		t.Errorf("expected created action in:\n%s", r.stdout)
	}
	if !strings.Contains(r.stdout, `"id": 1`) || !strings.Contains(r.stdout, `"id": 2`) {
		t.Errorf("expected both tickets in output:\n%s", r.stdout)
	}
}

func TestE2E_Watch_OnceProjectFilter(t *testing.T) {
	setupHome(t)
	for _, p := range []string{"a", "b"} {
		if r := runCLI(t, "--json", "project", "create", p); r.exit != 0 {
			t.Fatalf("project create %s: %s", p, r.stderr)
		}
		if r := runCLI(t, "--json", "new", "-p", p, "thing", "--body", "x"); r.exit != 0 {
			t.Fatalf("new in %s: %s", p, r.stderr)
		}
	}
	r := runCLI(t, "--json", "watch", "--once", "-p", "a", "--since", "1970-01-01T00:00:00Z")
	if r.exit != 0 {
		t.Fatalf("watch: %s", r.stderr)
	}
	if !strings.Contains(r.stdout, `"project": "a"`) {
		t.Errorf("expected project a in output: %s", r.stdout)
	}
	if strings.Contains(r.stdout, `"project": "b"`) {
		t.Errorf("project b should be filtered out: %s", r.stdout)
	}
}

func equalStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
