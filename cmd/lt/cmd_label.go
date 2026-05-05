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
	usage := "usage: lt label rm -p <project> [--id <id>... | <id>] <label>..."
	if add {
		name = "label add"
		usage = "usage: lt label add -p <project> [--id <id>... | <id>] <label>..."
	}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := projectFlag(fs)
	var multiIDs idList
	fs.Var(&multiIDs, "id", "ticket id (repeatable; multi-ID mode)")
	if err := parseArgs(fs, args); err != nil {
		return userErr("bad_flags", err.Error())
	}
	if err := requireProject(*project); err != nil {
		return err
	}

	var ids []int64
	var labels []string
	if len(multiIDs) > 0 {
		// Multi-ID mode: every positional arg is a label.
		labels = fs.Args()
		ids = []int64(multiIDs)
	} else {
		// Legacy single-ID mode: first positional is the id, rest are labels.
		if fs.NArg() < 2 {
			return userErr("usage", usage)
		}
		id, err := parseTicketID(fs.Arg(0))
		if err != nil {
			return err
		}
		ids = []int64{id}
		labels = fs.Args()[1:]
	}
	if len(labels) == 0 {
		return userErr("usage", usage)
	}
	for _, l := range labels {
		if err := validateLabel(l); err != nil {
			return err
		}
	}

	verb := "Labels removed"
	if add {
		verb = "Labels added"
	}
	op := func(s *store, id int64) (*ticket, error) {
		if add {
			if _, err := s.addLabels(*project, id, labels); err != nil {
				return nil, err
			}
		} else {
			if _, err := s.removeLabels(*project, id, labels); err != nil {
				return nil, err
			}
		}
		return s.getTicket(*project, id)
	}
	return applyBulk(ids, verb, op, stdout, mode)
}
