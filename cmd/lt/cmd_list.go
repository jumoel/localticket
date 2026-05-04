package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

type labelList []string

func (l *labelList) String() string     { return strings.Join(*l, ",") }
func (l *labelList) Set(v string) error { *l = append(*l, v); return nil }

func runListImpl(args []string, stdout io.Writer, mode outMode) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := projectFlag(fs)
	status := fs.String("status", "", "filter by status: open|in-progress|closed|all")
	var labels labelList
	fs.Var(&labels, "label", "filter by label (repeatable; AND semantics)")
	if err := parseArgs(fs, args); err != nil {
		return userErr("bad_flags", err.Error())
	}
	if err := requireProject(*project); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return userErr("usage", "usage: lt list -p <project> [--status ...] [--label L]...")
	}

	var statuses []string
	switch *status {
	case "":
		statuses = []string{"open", "in-progress"}
	case "all":
		statuses = nil
	case "open", "in-progress", "closed":
		statuses = []string{*status}
	default:
		return userErr("bad_status", fmt.Sprintf("invalid status %q (want open|in-progress|closed|all)", *status))
	}

	for _, l := range labels {
		if err := validateLabel(l); err != nil {
			return err
		}
	}

	s, err := openDefaultStore()
	if err != nil {
		return err
	}
	defer s.Close()

	tickets, err := s.listTickets(*project, statuses, labels)
	if err != nil {
		return err
	}

	if mode == modeJSON {
		if tickets == nil {
			tickets = []*ticket{}
		}
		return writeJSON(stdout, map[string]any{"tickets": tickets})
	}

	if len(tickets) == 0 {
		fmt.Fprintln(stdout, "No tickets.")
		return nil
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSTATUS\tTITLE\tLABELS\tUPDATED")
	for _, t := range tickets {
		fmt.Fprintf(tw, "#%d\t%s\t%s\t%s\t%s\n",
			t.ID, t.Status, truncateRunes(t.Title, 60), strings.Join(t.Labels, ","), t.UpdatedAt)
	}
	return tw.Flush()
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
