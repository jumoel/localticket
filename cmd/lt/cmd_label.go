package main

import (
	"flag"
	"fmt"
	"io"
)

func runLabelImpl(args []string, stdout io.Writer, mode outMode) error {
	if len(args) == 0 {
		return userErr("missing_subcommand", "usage: lt label <add|rm> -p <project> <id> <label>...")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "add":
		return labelAddOrRm(rest, stdout, mode, true)
	case "rm":
		return labelAddOrRm(rest, stdout, mode, false)
	default:
		return userErr("unknown_subcommand", fmt.Sprintf("unknown label subcommand: %q", sub))
	}
}

func labelAddOrRm(args []string, stdout io.Writer, mode outMode, add bool) error {
	name := "label rm"
	usage := "usage: lt label rm -p <project> <id> <label>..."
	if add {
		name = "label add"
		usage = "usage: lt label add -p <project> <id> <label>..."
	}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := projectFlag(fs)
	if err := parseArgs(fs, args); err != nil {
		return userErr("bad_flags", err.Error())
	}
	if err := requireProject(*project); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return userErr("usage", usage)
	}
	id, err := parseTicketID(fs.Arg(0))
	if err != nil {
		return err
	}
	labels := fs.Args()[1:]
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

	if add {
		if _, err := s.addLabels(*project, id, labels); err != nil {
			return err
		}
	} else {
		if _, err := s.removeLabels(*project, id, labels); err != nil {
			return err
		}
	}
	t, err := s.getTicket(*project, id)
	if err != nil {
		return err
	}
	return renderTicket(stdout, mode, t, "")
}
