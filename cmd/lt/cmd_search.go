package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

func runSearchImpl(args []string, stdout io.Writer, mode outMode) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := projectFlag(fs)
	columnsFlag := fs.String("columns", "", "comma-separated TTY columns (id,status,title,labels,links,updated_at,created_at,closed_at)")
	if err := parseArgs(fs, args); err != nil {
		return userErr("bad_flags", err.Error())
	}
	if err := requireProject(*project); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return userErr("usage", "usage: lt search -p <project> <query>... [--columns ...]")
	}
	query := strings.Join(fs.Args(), " ")
	columns, err := parseColumns(*columnsFlag)
	if err != nil {
		return err
	}

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
	return renderTicketTable(stdout, tickets, columns)
}
