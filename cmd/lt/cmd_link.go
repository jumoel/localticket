package main

import (
	"flag"
	"fmt"
	"io"
	"text/tabwriter"
)

func runLinkImpl(args []string, stdout io.Writer, mode outMode) error {
	if len(args) == 0 {
		return userErr("missing_subcommand", "usage: lt link <add|rm|list> ...")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "add":
		return linkAdd(rest, stdout, mode)
	case "rm":
		return linkRm(rest, stdout, mode)
	case "list":
		return runLinkList(rest, stdout, mode)
	default:
		return userErr("unknown_subcommand", fmt.Sprintf("unknown link subcommand: %q", sub))
	}
}

func linkAdd(args []string, stdout io.Writer, mode outMode) error {
	fs := flag.NewFlagSet("link add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := projectFlag(fs)
	if err := parseArgs(fs, args); err != nil {
		return userErr("bad_flags", err.Error())
	}
	if err := requireProject(*project); err != nil {
		return err
	}
	if fs.NArg() != 3 {
		return userErr("usage", "usage: lt link add -p <project> <id> <type> <other-id>")
	}
	id, err := parseTicketID(fs.Arg(0))
	if err != nil {
		return err
	}
	linkType := fs.Arg(1)
	other, err := parseTicketID(fs.Arg(2))
	if err != nil {
		return err
	}

	s, err := openDefaultStore()
	if err != nil {
		return err
	}
	defer s.Close()
	if err := s.addLink(*project, id, other, linkType); err != nil {
		return err
	}
	t, err := s.getTicket(*project, id)
	if err != nil {
		return err
	}
	return renderTicket(stdout, mode, t, "")
}

func runLinkList(args []string, stdout io.Writer, mode outMode) error {
	fs := flag.NewFlagSet("link list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := projectFlag(fs)
	typeFilter := fs.String("type", "", "filter by canonical link type")
	includeClosed := fs.Bool("include-closed", false, "include links touching closed tickets")
	if err := parseArgs(fs, args); err != nil {
		return userErr("bad_flags", err.Error())
	}
	if err := requireProject(*project); err != nil {
		return err
	}

	var ticketNum int64
	switch fs.NArg() {
	case 0:
		// list all links in project
	case 1:
		id, err := parseTicketID(fs.Arg(0))
		if err != nil {
			return err
		}
		ticketNum = id
	default:
		return userErr("usage", "usage: lt link list -p <project> [<id>] [--type T] [--include-closed]")
	}

	if *typeFilter != "" {
		valid := false
		for _, c := range canonicalLinkTypes {
			if c == *typeFilter {
				valid = true
				break
			}
		}
		if !valid {
			return userErr("bad_type", fmt.Sprintf("invalid --type %q; canonical types: %v", *typeFilter, canonicalLinkTypes))
		}
	}

	s, err := openDefaultStore()
	if err != nil {
		return err
	}
	defer s.Close()

	links, err := s.listProjectLinks(*project, ticketNum, *typeFilter, *includeClosed)
	if err != nil {
		return err
	}

	if mode == modeJSON {
		if links == nil {
			links = []projectLink{}
		}
		return writeJSON(stdout, map[string]any{"links": links})
	}

	if len(links) == 0 {
		fmt.Fprintln(stdout, "No links.")
		return nil
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "FROM\tTYPE\tTO")
	for _, l := range links {
		fmt.Fprintf(tw, "#%d\t%s\t#%d\n", l.From, l.Type, l.To)
	}
	return tw.Flush()
}

func linkRm(args []string, stdout io.Writer, mode outMode) error {
	fs := flag.NewFlagSet("link rm", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := projectFlag(fs)
	if err := parseArgs(fs, args); err != nil {
		return userErr("bad_flags", err.Error())
	}
	if err := requireProject(*project); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return userErr("usage", "usage: lt link rm -p <project> <id> <other-id>")
	}
	id, err := parseTicketID(fs.Arg(0))
	if err != nil {
		return err
	}
	other, err := parseTicketID(fs.Arg(1))
	if err != nil {
		return err
	}

	s, err := openDefaultStore()
	if err != nil {
		return err
	}
	defer s.Close()
	if err := s.removeLink(*project, id, other); err != nil {
		return err
	}
	t, err := s.getTicket(*project, id)
	if err != nil {
		return err
	}
	return renderTicket(stdout, mode, t, "")
}
