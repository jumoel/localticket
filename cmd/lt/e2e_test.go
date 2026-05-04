package main

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

type cliResult struct {
	exit   int
	stdout string
	stderr string
}

func runCLI(t *testing.T, args ...string) cliResult {
	t.Helper()
	return runCLIWithStdin(t, strings.NewReader(""), args...)
}

func runCLIWithStdin(t *testing.T, stdin io.Reader, args ...string) cliResult {
	t.Helper()
	var out, errBuf bytes.Buffer
	exit := run(args, stdin, false, &out, &errBuf)
	return cliResult{exit: exit, stdout: out.String(), stderr: errBuf.String()}
}

func setupHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

func mustJSON(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("invalid JSON: %v\n--- input ---\n%s", err, s)
	}
	return m
}

func TestE2E_ProjectLifecycle(t *testing.T) {
	setupHome(t)

	r := runCLI(t, "--json", "project", "create", "demo")
	if r.exit != 0 {
		t.Fatalf("create exit=%d stderr=%s", r.exit, r.stderr)
	}
	m := mustJSON(t, r.stdout)
	if m["name"] != "demo" {
		t.Errorf("created name=%v", m["name"])
	}

	r = runCLI(t, "--json", "project", "create", "demo")
	if r.exit != 3 {
		t.Errorf("dup create exit=%d, want 3", r.exit)
	}
	if !strings.Contains(r.stderr, "project_exists") {
		t.Errorf("dup stderr=%s", r.stderr)
	}

	r = runCLI(t, "--json", "project", "create", "Bad Name")
	if r.exit != 1 {
		t.Errorf("bad name exit=%d, want 1", r.exit)
	}

	r = runCLI(t, "--json", "project", "list")
	if r.exit != 0 {
		t.Fatalf("list exit=%d stderr=%s", r.exit, r.stderr)
	}
	m = mustJSON(t, r.stdout)
	projects, _ := m["projects"].([]any)
	if len(projects) != 1 {
		t.Fatalf("listed %d projects, want 1", len(projects))
	}

	r = runCLI(t, "--pretty", "project", "delete", "demo")
	if r.exit != 0 {
		t.Fatalf("delete exit=%d stderr=%s", r.exit, r.stderr)
	}
	if !strings.Contains(r.stdout, "Deleted") {
		t.Errorf("pretty delete stdout=%s", r.stdout)
	}

	r = runCLI(t, "--json", "project", "delete", "demo")
	if r.exit != 2 {
		t.Errorf("missing delete exit=%d, want 2", r.exit)
	}
}

func TestE2E_UnknownCommand(t *testing.T) {
	setupHome(t)
	r := runCLI(t, "--json", "frobnicate")
	if r.exit != 1 {
		t.Errorf("exit=%d, want 1", r.exit)
	}
	m := mustJSON(t, r.stderr)
	if m["code"] != "unknown_command" {
		t.Errorf("code=%v", m["code"])
	}
}

func TestE2E_HelpVersion(t *testing.T) {
	r := runCLI(t, "--help")
	if r.exit != 0 || !strings.Contains(r.stdout, "Usage:") {
		t.Errorf("help broken: exit=%d stdout=%q", r.exit, r.stdout)
	}
	r = runCLI(t, "--version")
	if r.exit != 0 || strings.TrimSpace(r.stdout) == "" {
		t.Errorf("version broken: exit=%d stdout=%q", r.exit, r.stdout)
	}
}
