package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

var version = "0.1.0-dev"

const usageText = `lt - local ticket store

Usage:
  lt [--json|--pretty] <command> [args...]

Commands:
  project create <name>
  project list
  project delete <name> [--force]

  new    -p <project> <title> [--body TEXT|--body-file PATH|--body -] [--label L]... [--link TYPE:ID]...
  list   -p <project> [--status open|in-progress|closed|all] [--label L]...
  show   -p <project> <id>
  edit   -p <project> <id> [--title T] [--body TEXT|--body-file PATH|--body -]
  status -p <project> <id> open|in-progress|closed
  close  -p <project> <id>
  reopen -p <project> <id>
  label  add|rm -p <project> <id> <label>
  link   add    -p <project> <id> <type> <other-id>
  link   rm     -p <project> <id> <other-id>
  search -p <project> <query>

  summary [--swiftbar]

Global flags:
  --json     Force JSON output
  --pretty   Force human-readable output
  --help     Show this help
  --version  Show version

Output mode is JSON when stdout is not a TTY, pretty otherwise.
Storage lives at ~/.localticket/db.sqlite.
`

type cmdError struct {
	code     string
	exitCode int
	msg      string
}

func (e *cmdError) Error() string { return e.msg }

func userErr(code, msg string) error  { return &cmdError{code, 1, msg} }
func notFound(msg string) error       { return &cmdError{"not_found", 2, msg} }
func conflict(code, msg string) error { return &cmdError{code, 3, msg} }
func internalErr(err error) error {
	return &cmdError{"internal", 4, err.Error()}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, stdinIsTTY(), os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdinTTY bool, stdout, stderr io.Writer) int {
	mode := modeAuto
	rest := make([]string, 0, len(args))
	for _, a := range args {
		switch a {
		case "--help", "-h":
			fmt.Fprint(stdout, usageText)
			return 0
		case "--version":
			fmt.Fprintln(stdout, version)
			return 0
		case "--json":
			mode = modeJSON
		case "--pretty":
			mode = modePretty
		default:
			rest = append(rest, a)
		}
	}
	if mode == modeAuto {
		mode = detectMode(stdout)
	}
	if len(rest) == 0 {
		if mode == modeJSON {
			writeError(stderr, mode, "usage", "no command given (try `lt --help`)")
		} else {
			fmt.Fprint(stderr, usageText)
		}
		return 1
	}

	cmd, sub := rest[0], rest[1:]
	err := dispatch(cmd, sub, stdin, stdinTTY, stdout, stderr, mode)
	if err == nil {
		return 0
	}
	var ce *cmdError
	if errors.As(err, &ce) {
		writeError(stderr, mode, ce.code, ce.msg)
		return ce.exitCode
	}
	writeError(stderr, mode, "internal", err.Error())
	return 4
}

func dispatch(cmd string, args []string, stdin io.Reader, stdinTTY bool, stdout, stderr io.Writer, mode outMode) error {
	switch cmd {
	case "project":
		return runProjectImpl(args, stdout, stderr, mode)
	case "new":
		return runNewImpl(args, stdin, stdinTTY, stdout, mode)
	case "list":
		return runListImpl(args, stdout, mode)
	case "show":
		return runShowImpl(args, stdout, mode)
	case "edit":
		return runEditImpl(args, stdin, stdinTTY, stdout, mode)
	case "status":
		return runStatusImpl(args, stdout, mode)
	case "close":
		return runCloseImpl(args, stdout, mode)
	case "reopen":
		return runReopenImpl(args, stdout, mode)
	case "label":
		return runLabelImpl(args, stdout, mode)
	case "link":
		return runLinkImpl(args, stdout, mode)
	case "search":
		return runSearchImpl(args, stdout, mode)
	case "summary":
		return runSummaryImpl(args, stdout, mode)
	default:
		return userErr("unknown_command", fmt.Sprintf("unknown command: %q (try `lt --help`)", cmd))
	}
}
