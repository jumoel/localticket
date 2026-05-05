// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"strings"
	"testing"
)

func TestFindSection_Exact(t *testing.T) {
	body := "# Top\n\n## Effort\n\n3 days\n\n## Notes\n\nlater\n"
	m, err := findSection(body, "Effort")
	if err != nil {
		t.Fatal(err)
	}
	if got := sectionContent(body, m); got != "\n3 days\n\n" {
		t.Errorf("content=%q", got)
	}
}

func TestFindSection_AcceptsHashPrefix(t *testing.T) {
	body := "## Effort\ntext\n"
	m, err := findSection(body, "## Effort")
	if err != nil {
		t.Fatal(err)
	}
	if m.heading.title != "Effort" {
		t.Errorf("title=%q", m.heading.title)
	}
}

func TestFindSection_CaseInsensitive(t *testing.T) {
	body := "## Acceptance Criteria\nstuff\n"
	if _, err := findSection(body, "acceptance criteria"); err != nil {
		t.Errorf("expected case-insensitive match, got %v", err)
	}
}

func TestFindSection_NotFound(t *testing.T) {
	body := "## Effort\nx\n"
	_, err := findSection(body, "Notes")
	if err == nil {
		t.Fatal("expected error")
	}
	var ce *cmdError
	if !asCmdError(err, &ce) || ce.code != "section_not_found" || ce.exitCode != 2 {
		t.Errorf("got %+v", ce)
	}
}

func TestFindSection_Ambiguous(t *testing.T) {
	body := "## Effort\nx\n## Effort\ny\n"
	_, err := findSection(body, "Effort")
	var ce *cmdError
	if !asCmdError(err, &ce) || ce.code != "section_ambiguous" {
		t.Errorf("expected section_ambiguous, got %+v", err)
	}
}

func TestFindSection_StopsAtSameLevel(t *testing.T) {
	body := "## A\nbody-A\n### A.1\nsub\n## B\nbody-B\n"
	m, err := findSection(body, "A")
	if err != nil {
		t.Fatal(err)
	}
	got := sectionContent(body, m)
	if !strings.Contains(got, "body-A") || !strings.Contains(got, "sub") {
		t.Errorf("missing inner content: %q", got)
	}
	if strings.Contains(got, "body-B") {
		t.Errorf("leaked into next section: %q", got)
	}
}

func TestFindSection_StopsAtHigherLevel(t *testing.T) {
	body := "### A\nbody-A\n## B\nbody-B\n"
	m, err := findSection(body, "A")
	if err != nil {
		t.Fatal(err)
	}
	got := sectionContent(body, m)
	if strings.Contains(got, "body-B") {
		t.Errorf("leaked into higher-level section: %q", got)
	}
}

func TestFindSection_AtEnd(t *testing.T) {
	body := "## Notes\nlast section\n"
	m, err := findSection(body, "Notes")
	if err != nil {
		t.Fatal(err)
	}
	if got := sectionContent(body, m); got != "last section\n" {
		t.Errorf("content=%q", got)
	}
}

func TestReplaceSection_PreservesHeading(t *testing.T) {
	body := "# Top\n\n## Effort\n\n3 days\n\n## Notes\n\nlater\n"
	m, err := findSection(body, "Effort")
	if err != nil {
		t.Fatal(err)
	}
	updated := replaceSection(body, m, "1 week\n")
	if !strings.Contains(updated, "## Effort\n1 week\n## Notes") {
		t.Errorf("unexpected:\n%s", updated)
	}
	if strings.Contains(updated, "3 days") {
		t.Error("old content not removed")
	}
}

func TestReplaceSection_AppendsTrailingNewline(t *testing.T) {
	body := "## A\nold\n## B\nb-body\n"
	m, _ := findSection(body, "A")
	updated := replaceSection(body, m, "new")
	if !strings.Contains(updated, "## A\nnew\n## B") {
		t.Errorf("missing newline before next heading:\n%s", updated)
	}
}

func TestEditSection_E2E(t *testing.T) {
	setupHome(t)
	if r := runCLI(t, "--json", "project", "create", "demo"); r.exit != 0 {
		t.Fatal(r.stderr)
	}
	body := "## Context\n\nold context\n\n## Effort\n\n2 days\n"
	if r := runCLI(t, "--json", "new", "-p", "demo", "task", "--body", body); r.exit != 0 {
		t.Fatal(r.stderr)
	}

	r := runCLI(t, "--json", "edit", "-p", "demo", "1", "--section", "Effort", "--content", "1 week\n")
	if r.exit != 0 {
		t.Fatalf("section edit: exit=%d stderr=%s", r.exit, r.stderr)
	}
	m := mustJSON(t, r.stdout)
	got, _ := m["body"].(string)
	if !strings.Contains(got, "## Effort\n1 week\n") {
		t.Errorf("body=%q", got)
	}
	if !strings.Contains(got, "old context") {
		t.Errorf("untouched section disappeared: %q", got)
	}

	r = runCLI(t, "--json", "show", "-p", "demo", "1", "--section", "Effort")
	if r.exit != 0 {
		t.Fatal(r.stderr)
	}
	m = mustJSON(t, r.stdout)
	if m["section"] != "Effort" || !strings.Contains(m["content"].(string), "1 week") {
		t.Errorf("show --section returned %v", m)
	}
}

func TestEditSection_NotFound(t *testing.T) {
	setupHome(t)
	if r := runCLI(t, "--json", "project", "create", "demo"); r.exit != 0 {
		t.Fatal(r.stderr)
	}
	if r := runCLI(t, "--json", "new", "-p", "demo", "x", "--body", "## A\ny\n"); r.exit != 0 {
		t.Fatal(r.stderr)
	}
	r := runCLI(t, "--json", "edit", "-p", "demo", "1", "--section", "Missing", "--content", "x")
	if r.exit != 2 {
		t.Errorf("exit=%d, want 2", r.exit)
	}
	m := mustJSON(t, r.stderr)
	if m["code"] != "section_not_found" {
		t.Errorf("code=%v", m["code"])
	}
}

func TestEditSection_RejectsBodyAlongside(t *testing.T) {
	setupHome(t)
	if r := runCLI(t, "--json", "project", "create", "demo"); r.exit != 0 {
		t.Fatal(r.stderr)
	}
	if r := runCLI(t, "--json", "new", "-p", "demo", "x", "--body", "## A\ny\n"); r.exit != 0 {
		t.Fatal(r.stderr)
	}
	r := runCLI(t, "--json", "edit", "-p", "demo", "1", "--section", "A", "--body", "wholesale")
	if r.exit != 1 {
		t.Errorf("exit=%d, want 1", r.exit)
	}
	m := mustJSON(t, r.stderr)
	if m["code"] != "conflicting_flags" {
		t.Errorf("code=%v", m["code"])
	}
}
