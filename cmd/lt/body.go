package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type bodyFlags struct {
	body     string
	bodyFile string
	bodySet  bool
	flagName string // tracks which flag base ("body", "content", ...) was used
}

func (b *bodyFlags) bind(fs *flag.FlagSet) { b.bindAs(fs, "body") }

// bindAs registers the flags under a chosen base name, e.g. "body" or
// "content". The resolve / markSeen helpers track which flag was actually
// passed via a stored name so the same struct can power lt new --body and
// lt edit --section --content.
func (b *bodyFlags) bindAs(fs *flag.FlagSet, base string) {
	b.flagName = base
	fs.StringVar(&b.body, base, "", base+" (literal text; pass - to read from stdin)")
	fs.StringVar(&b.bodyFile, base+"-file", "", base+" from file path")
}

// markSeen records whether the literal flag was explicitly passed so we can
// distinguish e.g. `--body ""` (force empty) from no flag at all (fall through
// to stdin/editor).
func (b *bodyFlags) markSeen(fs *flag.FlagSet) {
	target := b.flagName
	if target == "" {
		target = "body"
	}
	fs.Visit(func(f *flag.Flag) {
		if f.Name == target {
			b.bodySet = true
		}
	})
}

type editorMode int

const (
	editorForNew  editorMode = iota // empty buffer = abort
	editorForEdit                   // pre-populated; unchanged = no-op handled by caller
)

// resolve returns body, abortRequested, error. abortRequested is true when the
// editor came back with no usable content for a `new` (empty buffer = user aborted).
//
// `fallback` is used in two places: as the editor's pre-populated buffer in
// TTY mode, and as the body itself when the caller is non-TTY with no other
// body source. A non-empty fallback suppresses the implicit greedy read of
// stdin so callers (templates, lt edit's existing-body case) can supply
// content without forcing the user to also redirect stdin.
func (b *bodyFlags) resolve(stdin io.Reader, stdinTTY bool, mode editorMode, fallback string) (string, bool, error) {
	if b.bodyFile != "" {
		raw, err := os.ReadFile(b.bodyFile)
		if err != nil {
			return "", false, userErr("body_file", fmt.Sprintf("read --body-file: %v", err))
		}
		return string(raw), false, nil
	}
	if b.bodySet && b.body == "-" {
		raw, err := io.ReadAll(stdin)
		if err != nil {
			return "", false, userErr("stdin", fmt.Sprintf("read stdin: %v", err))
		}
		return string(raw), false, nil
	}
	if b.bodySet {
		return b.body, false, nil
	}
	if !stdinTTY {
		// For `new`, a non-empty fallback (e.g. a template) wins over the
		// implicit greedy read of stdin so callers can supply content without
		// the user having to also redirect stdin. `edit` keeps its old
		// stdin-read behavior even when given a fallback (the existing body).
		if fallback != "" && mode == editorForNew {
			return fallback, false, nil
		}
		raw, err := io.ReadAll(stdin)
		if err != nil {
			return "", false, userErr("stdin", fmt.Sprintf("read stdin: %v", err))
		}
		return string(raw), false, nil
	}
	out, err := launchEditor(fallback)
	if err != nil {
		return "", false, err
	}
	if mode == editorForNew && strings.TrimSpace(out) == "" {
		return "", true, nil
	}
	return out, false, nil
}

func launchEditor(initial string) (string, error) {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	tmp, err := os.CreateTemp("", "lt-*.md")
	if err != nil {
		return "", internalErr(err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if initial != "" {
		if _, err := tmp.WriteString(initial); err != nil {
			tmp.Close()
			return "", internalErr(err)
		}
	}
	tmp.Close()

	cmd := exec.Command("sh", "-c", editor+" "+shellQuote(tmpPath))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", userErr("editor_failed", fmt.Sprintf("editor exited with error: %v", err))
	}
	raw, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", internalErr(err)
	}
	return string(raw), nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func stdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
