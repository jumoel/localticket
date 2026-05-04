package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
)

func runNewImpl(args []string, stdout io.Writer, mode outMode) error {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := projectFlag(fs)
	body := &bodyFlags{}
	body.bind(fs)
	var labels labelList
	var links linkList
	fs.Var(&labels, "label", "label to apply (repeatable)")
	fs.Var(&links, "link", "link to add as TYPE:ID (repeatable; e.g. blocks:3)")
	if err := parseArgs(fs, args); err != nil {
		return userErr("bad_flags", err.Error())
	}
	body.markSeen(fs)
	if err := requireProject(*project); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return userErr("usage", "usage: lt new -p <project> <title> [--label L]... [--link TYPE:ID]...")
	}
	title := strings.Join(fs.Args(), " ")

	for _, l := range labels {
		if err := validateLabel(l); err != nil {
			return err
		}
	}
	parsedLinks := make([]parsedLink, 0, len(links))
	for _, raw := range links {
		pl, err := parseLinkSpec(raw)
		if err != nil {
			return err
		}
		parsedLinks = append(parsedLinks, pl)
	}

	bodyText, aborted, err := body.resolve(os.Stdin, stdinIsTTY(), editorForNew, "")
	if err != nil {
		return err
	}
	if aborted {
		return userErr("aborted", "ticket creation aborted (empty body); pass --body \"\" to force an empty body")
	}
	s, err := openDefaultStore()
	if err != nil {
		return err
	}
	defer s.Close()
	t, err := s.createTicket(*project, title, bodyText)
	if err != nil {
		return err
	}
	if len(labels) > 0 {
		if _, err := s.addLabels(*project, t.ID, labels); err != nil {
			return err
		}
	}
	for _, pl := range parsedLinks {
		if err := s.addLink(*project, t.ID, pl.target, pl.typ); err != nil {
			return err
		}
	}
	if len(labels) > 0 || len(parsedLinks) > 0 {
		t, err = s.getTicket(*project, t.ID)
		if err != nil {
			return err
		}
	}
	return renderTicket(stdout, mode, t, "Created")
}

type linkList []string

func (l *linkList) String() string     { return strings.Join(*l, ",") }
func (l *linkList) Set(v string) error { *l = append(*l, v); return nil }

type parsedLink struct {
	typ    string
	target int64
}

func parseLinkSpec(s string) (parsedLink, error) {
	colon := strings.Index(s, ":")
	if colon <= 0 || colon == len(s)-1 {
		return parsedLink{}, userErr("bad_link", fmt.Sprintf("invalid --link %q (want TYPE:ID, e.g. blocks:3)", s))
	}
	id, err := parseTicketID(s[colon+1:])
	if err != nil {
		return parsedLink{}, err
	}
	return parsedLink{typ: s[:colon], target: id}, nil
}

func runShowImpl(args []string, stdout io.Writer, mode outMode) error {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := projectFlag(fs)
	if err := parseArgs(fs, args); err != nil {
		return userErr("bad_flags", err.Error())
	}
	if err := requireProject(*project); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return userErr("usage", "usage: lt show -p <project> <id>")
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
	t, err := s.getTicket(*project, id)
	if err != nil {
		return err
	}
	return renderTicket(stdout, mode, t, "")
}

func projectFlag(fs *flag.FlagSet) *string {
	var p string
	fs.StringVar(&p, "project", "", "project name (required)")
	fs.StringVar(&p, "p", "", "project name (shorthand)")
	return &p
}

func requireProject(name string) error {
	if name == "" {
		return userErr("missing_project", "missing required flag: -p/--project")
	}
	return validateProjectName(name)
}

func parseTicketID(s string) (int64, error) {
	s = strings.TrimPrefix(s, "#")
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 1 {
		return 0, userErr("bad_id", fmt.Sprintf("invalid ticket id: %q", s))
	}
	return n, nil
}

func renderTicket(w io.Writer, mode outMode, t *ticket, verb string) error {
	if mode == modeJSON {
		return writeJSON(w, t)
	}
	if verb != "" {
		fmt.Fprintf(w, "%s %s#%d\n\n", verb, t.Project, t.ID)
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "Project:\t%s\n", t.Project)
	fmt.Fprintf(tw, "ID:\t#%d\n", t.ID)
	fmt.Fprintf(tw, "Title:\t%s\n", t.Title)
	fmt.Fprintf(tw, "Status:\t%s\n", t.Status)
	if len(t.Labels) > 0 {
		fmt.Fprintf(tw, "Labels:\t%s\n", strings.Join(t.Labels, ", "))
	}
	if len(t.Links) > 0 {
		parts := make([]string, len(t.Links))
		for i, l := range t.Links {
			parts[i] = fmt.Sprintf("%s #%d", l.Type, l.Target)
		}
		fmt.Fprintf(tw, "Links:\t%s\n", strings.Join(parts, ", "))
	}
	fmt.Fprintf(tw, "Created:\t%s\n", t.CreatedAt)
	fmt.Fprintf(tw, "Updated:\t%s\n", t.UpdatedAt)
	if t.ClosedAt != nil {
		fmt.Fprintf(tw, "Closed:\t%s\n", *t.ClosedAt)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if t.Body != "" {
		fmt.Fprintf(w, "\n%s\n", strings.TrimRight(t.Body, "\n"))
	}
	return nil
}
