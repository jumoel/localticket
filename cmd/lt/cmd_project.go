package main

import (
	"flag"
	"fmt"
	"io"
	"text/tabwriter"
)

func runProjectImpl(args []string, stdout io.Writer, mode outMode) error {
	if len(args) == 0 {
		return userErr("missing_subcommand", "usage: lt project <create|list|delete> ...")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "create":
		return projectCreate(rest, stdout, mode)
	case "list", "ls":
		return projectList(rest, stdout, mode)
	case "delete", "rm":
		return projectDelete(rest, stdout, mode)
	default:
		return userErr("unknown_subcommand", fmt.Sprintf("unknown project subcommand: %q", sub))
	}
}

func projectCreate(args []string, stdout io.Writer, mode outMode) error {
	fs := flag.NewFlagSet("project create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := parseArgs(fs, args); err != nil {
		return userErr("bad_flags", err.Error())
	}
	if fs.NArg() != 1 {
		return userErr("usage", "usage: lt project create <name>")
	}
	name := fs.Arg(0)
	if err := validateProjectName(name); err != nil {
		return err
	}
	s, err := openDefaultStore()
	if err != nil {
		return err
	}
	defer s.Close()
	p, err := s.createProject(name)
	if err != nil {
		return err
	}
	if mode == modeJSON {
		return writeJSON(stdout, map[string]any{
			"name":       p.Name,
			"created_at": p.CreatedAt,
		})
	}
	fmt.Fprintf(stdout, "Created project %q\n", p.Name)
	return nil
}

func projectList(args []string, stdout io.Writer, mode outMode) error {
	fs := flag.NewFlagSet("project list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := parseArgs(fs, args); err != nil {
		return userErr("bad_flags", err.Error())
	}
	if fs.NArg() != 0 {
		return userErr("usage", "usage: lt project list")
	}
	s, err := openDefaultStore()
	if err != nil {
		return err
	}
	defer s.Close()
	ps, err := s.listProjects()
	if err != nil {
		return err
	}
	if mode == modeJSON {
		if ps == nil {
			ps = []projectSummary{}
		}
		return writeJSON(stdout, map[string]any{"projects": ps})
	}
	if len(ps) == 0 {
		fmt.Fprintln(stdout, "No projects.")
		return nil
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tOPEN\tIN-PROGRESS\tCLOSED\tCREATED")
	for _, p := range ps {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%s\n", p.Name, p.Tickets["open"], p.Tickets["in_progress"], p.Tickets["closed"], p.CreatedAt)
	}
	return tw.Flush()
}

func projectDelete(args []string, stdout io.Writer, mode outMode) error {
	fs := flag.NewFlagSet("project delete", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	force := fs.Bool("force", false, "delete even if non-closed tickets exist")
	if err := parseArgs(fs, args); err != nil {
		return userErr("bad_flags", err.Error())
	}
	if fs.NArg() != 1 {
		return userErr("usage", "usage: lt project delete <name> [--force]")
	}
	name := fs.Arg(0)
	if err := validateProjectName(name); err != nil {
		return err
	}
	s, err := openDefaultStore()
	if err != nil {
		return err
	}
	defer s.Close()
	if err := s.deleteProject(name, *force); err != nil {
		return err
	}
	if mode == modeJSON {
		return writeJSON(stdout, map[string]any{"deleted": name})
	}
	fmt.Fprintf(stdout, "Deleted project %q\n", name)
	return nil
}

func openDefaultStore() (*store, error) {
	path, err := defaultDBPath()
	if err != nil {
		return nil, internalErr(err)
	}
	s, err := openStore(path)
	if err != nil {
		return nil, internalErr(err)
	}
	return s, nil
}
