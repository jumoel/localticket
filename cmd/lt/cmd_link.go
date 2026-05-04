package main

import (
	"flag"
	"fmt"
	"io"
)

func runLinkImpl(args []string, stdout io.Writer, mode outMode) error {
	if len(args) == 0 {
		return userErr("missing_subcommand", "usage: lt link <add|rm> ...")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "add":
		return linkAdd(rest, stdout, mode)
	case "rm":
		return linkRm(rest, stdout, mode)
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
