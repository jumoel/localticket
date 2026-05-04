package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

func runSearchImpl(args []string, stdout io.Writer, mode outMode) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := projectFlag(fs)
	if err := parseArgs(fs, args); err != nil {
		return userErr("bad_flags", err.Error())
	}
	if err := requireProject(*project); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return userErr("usage", "usage: lt search -p <project> <query>...")
	}
	query := strings.Join(fs.Args(), " ")

	s, err := openDefaultStore()
	if err != nil {
		return err
	}
	defer s.Close()

	tickets, err := s.searchTickets(*project, query)
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
		fmt.Fprintln(stdout, "No matches.")
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
