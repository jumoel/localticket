// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"strings"
	"testing"
)

func bulkSeed(t *testing.T) {
	t.Helper()
	setupHome(t)
	if r := runCLI(t, "--json", "project", "create", "demo"); r.exit != 0 {
		t.Fatalf("project create: %s", r.stderr)
	}
	for _, title := range []string{"first", "second", "third"} {
		if r := runCLI(t, "--json", "new", "-p", "demo", title, "--body", "x"); r.exit != 0 {
			t.Fatalf("new %q: %s", title, r.stderr)
		}
	}
}

func TestBulkClose_SingleStaysBare(t *testing.T) {
	bulkSeed(t)
	r := runCLI(t, "--json", "close", "-p", "demo", "1")
	if r.exit != 0 {
		t.Fatalf("close: exit=%d stderr=%s", r.exit, r.stderr)
	}
	m := mustJSON(t, r.stdout)
	if m["id"] == nil || m["status"] != "closed" {
		t.Errorf("single-ID close should return bare ticket, got %v", m)
	}
	if _, wrapped := m["tickets"]; wrapped {
		t.Errorf("single-ID close should not return wrapper")
	}
}

func TestBulkClose_MultiWraps(t *testing.T) {
	bulkSeed(t)
	r := runCLI(t, "--json", "close", "-p", "demo", "1", "2", "3")
	if r.exit != 0 {
		t.Fatalf("close: exit=%d stderr=%s", r.exit, r.stderr)
	}
	m := mustJSON(t, r.stdout)
	tickets, _ := m["tickets"].([]any)
	if len(tickets) != 3 {
		t.Errorf("got %d tickets, want 3: %v", len(tickets), m)
	}
	errs, _ := m["errors"].([]any)
	if len(errs) != 0 {
		t.Errorf("got %d errors, want 0: %v", len(errs), m)
	}
}

func TestBulkClose_PartialFailure(t *testing.T) {
	bulkSeed(t)
	// #99 doesn't exist; #1 and #2 do.
	r := runCLI(t, "--json", "close", "-p", "demo", "1", "99", "2")
	if r.exit != 2 {
		t.Errorf("exit=%d, want 2 (not_found from #99)", r.exit)
	}
	m := mustJSON(t, r.stdout)
	tickets, _ := m["tickets"].([]any)
	if len(tickets) != 2 {
		t.Errorf("got %d tickets, want 2", len(tickets))
	}
	errs, _ := m["errors"].([]any)
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1: %v", len(errs), m)
	}
	first, _ := errs[0].(map[string]any)
	if first["id"] != float64(99) {
		t.Errorf("error id=%v, want 99", first["id"])
	}
	if first["code"] != "not_found" {
		t.Errorf("error code=%v, want not_found", first["code"])
	}
}

func TestBulkStatus_LastArgIsStatus(t *testing.T) {
	bulkSeed(t)
	r := runCLI(t, "--json", "status", "-p", "demo", "1", "2", "in-progress")
	if r.exit != 0 {
		t.Fatalf("status: exit=%d stderr=%s", r.exit, r.stderr)
	}
	m := mustJSON(t, r.stdout)
	tickets, _ := m["tickets"].([]any)
	if len(tickets) != 2 {
		t.Fatalf("got %d tickets, want 2", len(tickets))
	}
	for _, t0 := range tickets {
		tm, _ := t0.(map[string]any)
		if tm["status"] != "in-progress" {
			t.Errorf("status=%v, want in-progress", tm["status"])
		}
	}
}

func TestBulkLabelAdd_ViaIdFlag(t *testing.T) {
	bulkSeed(t)
	r := runCLI(t, "--json", "label", "add", "-p", "demo", "--id", "1", "--id", "2", "needs-review", "bug")
	if r.exit != 0 {
		t.Fatalf("label add: exit=%d stderr=%s", r.exit, r.stderr)
	}
	m := mustJSON(t, r.stdout)
	tickets, _ := m["tickets"].([]any)
	if len(tickets) != 2 {
		t.Fatalf("got %d tickets, want 2", len(tickets))
	}
	for _, t0 := range tickets {
		tm, _ := t0.(map[string]any)
		labels, _ := tm["labels"].([]any)
		if len(labels) != 2 {
			t.Errorf("ticket #%v labels=%v, want 2", tm["id"], labels)
		}
	}
}

func TestBulkLabelAdd_LegacyPositional(t *testing.T) {
	bulkSeed(t)
	// Single-ID legacy form should still work and return a bare ticket.
	r := runCLI(t, "--json", "label", "add", "-p", "demo", "1", "shipit")
	if r.exit != 0 {
		t.Fatalf("label add: exit=%d stderr=%s", r.exit, r.stderr)
	}
	m := mustJSON(t, r.stdout)
	if m["id"] == nil {
		t.Errorf("expected bare ticket, got %v", m)
	}
	if _, wrapped := m["tickets"]; wrapped {
		t.Errorf("legacy single-ID label add should not wrap")
	}
	labels, _ := m["labels"].([]any)
	if len(labels) != 1 || labels[0] != "shipit" {
		t.Errorf("labels=%v, want [shipit]", labels)
	}
}

func TestBulkClose_AllFailureReportedOnStdout(t *testing.T) {
	setupHome(t)
	if r := runCLI(t, "--json", "project", "create", "empty"); r.exit != 0 {
		t.Fatalf("project create: %s", r.stderr)
	}
	r := runCLI(t, "--json", "close", "-p", "empty", "1", "2")
	if r.exit != 2 {
		t.Errorf("exit=%d, want 2", r.exit)
	}
	m := mustJSON(t, r.stdout)
	errs, _ := m["errors"].([]any)
	if len(errs) != 2 {
		t.Errorf("got %d errors, want 2", len(errs))
	}
	tickets, _ := m["tickets"].([]any)
	if len(tickets) != 0 {
		t.Errorf("got %d tickets, want 0", len(tickets))
	}
	// stderr should also have a summary in JSON mode
	if !strings.Contains(r.stderr, "bulk") && !strings.Contains(r.stderr, "operations failed") {
		t.Errorf("expected bulk summary in stderr, got %q", r.stderr)
	}
}
