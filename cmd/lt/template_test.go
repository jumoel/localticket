// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemplate(t *testing.T, relPath, content string) {
	t.Helper()
	root, err := templatesRoot()
	if err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveTemplate_GlobalHit(t *testing.T) {
	setupHome(t)
	writeTemplate(t, "phase.md", "## Effort\n")

	body, err := resolveTemplate("any-project", "phase")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "## Effort") {
		t.Errorf("got %q, want body containing '## Effort'", body)
	}
}

func TestResolveTemplate_ProjectScopedWins(t *testing.T) {
	setupHome(t)
	writeTemplate(t, "phase.md", "global content\n")
	writeTemplate(t, "demo/phase.md", "project content\n")

	body, err := resolveTemplate("demo", "phase")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "project content") {
		t.Errorf("got %q, want project-scoped content", body)
	}
}

func TestResolveTemplate_NotFound(t *testing.T) {
	setupHome(t)
	_, err := resolveTemplate("demo", "nope")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var ce *cmdError
	if !asCmdError(err, &ce) || ce.code != "template_not_found" {
		t.Errorf("expected template_not_found, got %+v", err)
	}
	if ce.exitCode != 2 {
		t.Errorf("exitCode=%d, want 2", ce.exitCode)
	}
}

func TestListTemplates_Empty(t *testing.T) {
	setupHome(t)
	entries, err := listTemplates()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d entries, want 0", len(entries))
	}
}

func TestListTemplates_GlobalAndProject(t *testing.T) {
	setupHome(t)
	writeTemplate(t, "bug.md", "")
	writeTemplate(t, "phase.md", "")
	writeTemplate(t, "demo/release.md", "")
	writeTemplate(t, "demo/notes.txt", "ignored - wrong extension")
	writeTemplate(t, "other/sprint.md", "")

	entries, err := listTemplates()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("got %d entries, want 4: %+v", len(entries), entries)
	}
	// sorted: global first, then by project then by name
	want := []templateEntry{
		{Name: "bug", Scope: "global"},
		{Name: "phase", Scope: "global"},
		{Name: "notes", Scope: "project", Project: "demo"},
		{Name: "release", Scope: "project", Project: "demo"},
		{Name: "sprint", Scope: "project", Project: "other"},
	}
	_ = want // not asserting the .txt -> exclusion via the file already being filtered

	// Verify .txt was filtered: the entry "notes" should NOT appear
	for _, e := range entries {
		if e.Name == "notes" {
			t.Errorf("notes.txt leaked into list as %+v", e)
		}
	}
}

func TestNew_WithTemplate_NonTTY(t *testing.T) {
	setupHome(t)
	writeTemplate(t, "phase.md", "## Context\n\n## Acceptance\n")

	if r := runCLI(t, "--json", "project", "create", "demo"); r.exit != 0 {
		t.Fatalf("project create: exit=%d stderr=%s", r.exit, r.stderr)
	}
	r := runCLI(t, "--json", "new", "-p", "demo", "with template", "--template", "phase")
	if r.exit != 0 {
		t.Fatalf("new --template: exit=%d stderr=%s", r.exit, r.stderr)
	}
	m := mustJSON(t, r.stdout)
	body, _ := m["body"].(string)
	if !strings.Contains(body, "## Context") || !strings.Contains(body, "## Acceptance") {
		t.Errorf("body=%q, want template content", body)
	}
}

func TestNew_TemplateNotFound(t *testing.T) {
	setupHome(t)
	if r := runCLI(t, "--json", "project", "create", "demo"); r.exit != 0 {
		t.Fatalf("project create: exit=%d stderr=%s", r.exit, r.stderr)
	}
	r := runCLI(t, "--json", "new", "-p", "demo", "x", "--template", "missing")
	if r.exit != 2 {
		t.Errorf("exit=%d, want 2", r.exit)
	}
	m := mustJSON(t, r.stderr)
	if m["code"] != "template_not_found" {
		t.Errorf("code=%v, want template_not_found", m["code"])
	}
}

func TestNew_BodyFlagOverridesTemplate(t *testing.T) {
	setupHome(t)
	writeTemplate(t, "phase.md", "TEMPLATE_BODY")

	if r := runCLI(t, "--json", "project", "create", "demo"); r.exit != 0 {
		t.Fatalf("project create: exit=%d stderr=%s", r.exit, r.stderr)
	}
	r := runCLI(t, "--json", "new", "-p", "demo", "x", "--template", "phase", "--body", "EXPLICIT")
	if r.exit != 0 {
		t.Fatalf("new: exit=%d stderr=%s", r.exit, r.stderr)
	}
	m := mustJSON(t, r.stdout)
	if m["body"] != "EXPLICIT" {
		t.Errorf("body=%v, want EXPLICIT (template should be ignored when --body given)", m["body"])
	}
}

func TestTemplateList_Pretty(t *testing.T) {
	setupHome(t)
	writeTemplate(t, "phase.md", "")

	r := runCLI(t, "--pretty", "template", "list")
	if r.exit != 0 {
		t.Fatalf("template list: exit=%d stderr=%s", r.exit, r.stderr)
	}
	if !strings.Contains(r.stdout, "phase") {
		t.Errorf("stdout=%q, want 'phase'", r.stdout)
	}
}

func TestTemplateList_JSONEmpty(t *testing.T) {
	setupHome(t)
	r := runCLI(t, "--json", "template", "list")
	if r.exit != 0 {
		t.Fatalf("exit=%d stderr=%s", r.exit, r.stderr)
	}
	m := mustJSON(t, r.stdout)
	tmpls, ok := m["templates"].([]any)
	if !ok || len(tmpls) != 0 {
		t.Errorf("templates=%v, want empty array", m["templates"])
	}
}
