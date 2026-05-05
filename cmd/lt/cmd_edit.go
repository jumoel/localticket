package main

import (
	"flag"
	"io"
	"strings"
)

func runEditImpl(args []string, stdin io.Reader, stdinTTY bool, stdout io.Writer, mode outMode) error {
	fs := flag.NewFlagSet("edit", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := projectFlag(fs)
	var title string
	titleSet := false
	fs.StringVar(&title, "title", "", "new title")
	body := &bodyFlags{}
	body.bind(fs)
	if err := parseArgs(fs, args); err != nil {
		return userErr("bad_flags", err.Error())
	}
	body.markSeen(fs)
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "title" {
			titleSet = true
		}
	})
	if err := requireProject(*project); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return userErr("usage", "usage: lt edit -p <project> <id> [--title T] [--body ...|--body-file ...|--body -]")
	}
	id, err := parseTicketID(fs.Arg(0))
	if err != nil {
		return err
	}

	s, err := openDefaultStore()
	if err != nil {
		return err
	}
	defer s.Close()

	current, err := s.getTicket(*project, id)
	if err != nil {
		return err
	}

	var newTitle *string
	if titleSet {
		trimmed := strings.TrimSpace(title)
		if trimmed == "" {
			return userErr("empty_title", "title must be non-empty")
		}
		newTitle = &trimmed
	}

	var newBody *string
	bodyFlagPresent := body.bodySet || body.bodyFile != ""
	if bodyFlagPresent {
		text, _, err := body.resolve(stdin, stdinTTY, editorForEdit, current.Body)
		if err != nil {
			return err
		}
		newBody = &text
	} else if !titleSet {
		text, _, err := body.resolve(stdin, stdinTTY, editorForEdit, current.Body)
		if err != nil {
			return err
		}
		if text == current.Body {
			return renderTicket(stdout, mode, current, "")
		}
		if strings.TrimSpace(text) == "" {
			return userErr("empty_body", "edit aborted: body empty; pass --body \"\" to force an empty body")
		}
		newBody = &text
	}

	if _, err := s.updateTicket(*project, id, newTitle, newBody); err != nil {
		return err
	}
	t, err := s.getTicket(*project, id)
	if err != nil {
		return err
	}
	return renderTicket(stdout, mode, t, "")
}

func runStatusImpl(args []string, stdout io.Writer, mode outMode) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := projectFlag(fs)
	if err := parseArgs(fs, args); err != nil {
		return userErr("bad_flags", err.Error())
	}
	if err := requireProject(*project); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return userErr("usage", "usage: lt status -p <project> <id>... open|in-progress|closed")
	}
	newStatus := fs.Arg(fs.NArg() - 1)
	idArgs := fs.Args()[:fs.NArg()-1]
	ids, err := parsePositionalIDs(idArgs)
	if err != nil {
		return err
	}
	return applyStatus(*project, ids, newStatus, stdout, mode)
}

func runCloseImpl(args []string, stdout io.Writer, mode outMode) error {
	fs := flag.NewFlagSet("close", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := projectFlag(fs)
	if err := parseArgs(fs, args); err != nil {
		return userErr("bad_flags", err.Error())
	}
	if err := requireProject(*project); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return userErr("usage", "usage: lt close -p <project> <id>...")
	}
	ids, err := parsePositionalIDs(fs.Args())
	if err != nil {
		return err
	}
	return applyStatus(*project, ids, "closed", stdout, mode)
}

func runReopenImpl(args []string, stdout io.Writer, mode outMode) error {
	fs := flag.NewFlagSet("reopen", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := projectFlag(fs)
	if err := parseArgs(fs, args); err != nil {
		return userErr("bad_flags", err.Error())
	}
	if err := requireProject(*project); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return userErr("usage", "usage: lt reopen -p <project> <id>...")
	}
	ids, err := parsePositionalIDs(fs.Args())
	if err != nil {
		return err
	}
	return applyStatus(*project, ids, "open", stdout, mode)
}

func parsePositionalIDs(args []string) ([]int64, error) {
	if len(args) == 0 {
		return nil, userErr("usage", "no ticket ids given")
	}
	out := make([]int64, 0, len(args))
	for _, a := range args {
		id, err := parseTicketID(a)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

func applyStatus(project string, ids []int64, newStatus string, stdout io.Writer, mode outMode) error {
	verb := statusVerb(newStatus)
	op := func(s *store, id int64) (*ticket, error) {
		if _, err := s.setTicketStatus(project, id, newStatus); err != nil {
			return nil, err
		}
		return s.getTicket(project, id)
	}
	return applyBulk(ids, verb, op, stdout, mode)
}

func statusVerb(s string) string {
	switch s {
	case "closed":
		return "Closed"
	case "open":
		return "Reopened"
	case "in-progress":
		return "In progress"
	}
	return "Status set"
}
