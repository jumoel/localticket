// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// templatesRoot returns ~/.localticket/templates. Honors the same HOME
// resolution that the store does.
func templatesRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".localticket", "templates"), nil
}

// resolveTemplate looks up a template by name. It checks the project-scoped
// directory first, then the global directory. Returns the file contents on
// hit or a not_found error otherwise.
func resolveTemplate(projectName, templateName string) (string, error) {
	root, err := templatesRoot()
	if err != nil {
		return "", internalErr(err)
	}

	candidates := []string{
		filepath.Join(root, projectName, templateName+".md"),
		filepath.Join(root, templateName+".md"),
	}
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err == nil {
			return string(data), nil
		}
		if !os.IsNotExist(err) {
			return "", userErr("template_read", fmt.Sprintf("read template %q: %v", p, err))
		}
	}
	return "", &cmdError{
		code:     "template_not_found",
		exitCode: 2,
		msg:      fmt.Sprintf("template %q not found (looked in %s)", templateName, root),
	}
}

type templateEntry struct {
	Name    string `json:"name"`
	Scope   string `json:"scope"`             // "global" or "project"
	Project string `json:"project,omitempty"` // only set when scope=="project"
}

// listTemplates walks the templates root and returns every .md file found,
// distinguishing global templates (top-level) from project-scoped ones (one
// directory deep). Returns an empty slice if the root doesn't exist.
func listTemplates() ([]templateEntry, error) {
	root, err := templatesRoot()
	if err != nil {
		return nil, internalErr(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []templateEntry{}, nil
		}
		return nil, internalErr(err)
	}

	var out []templateEntry
	for _, e := range entries {
		if e.IsDir() {
			projDir := filepath.Join(root, e.Name())
			children, err := os.ReadDir(projDir)
			if err != nil {
				return nil, internalErr(err)
			}
			for _, c := range children {
				if c.IsDir() || filepath.Ext(c.Name()) != ".md" {
					continue
				}
				name := c.Name()[:len(c.Name())-len(".md")]
				out = append(out, templateEntry{Name: name, Scope: "project", Project: e.Name()})
			}
			continue
		}
		if filepath.Ext(e.Name()) != ".md" {
			continue
		}
		name := e.Name()[:len(e.Name())-len(".md")]
		out = append(out, templateEntry{Name: name, Scope: "global"})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Scope != out[j].Scope {
			return out[i].Scope < out[j].Scope
		}
		if out[i].Project != out[j].Project {
			return out[i].Project < out[j].Project
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func runTemplateImpl(args []string, stdout io.Writer, mode outMode) error {
	if len(args) == 0 {
		return userErr("missing_subcommand", "usage: lt template list")
	}
	switch args[0] {
	case "list":
		return runTemplateList(args[1:], stdout, mode)
	default:
		return userErr("unknown_subcommand", fmt.Sprintf("unknown template subcommand: %q", args[0]))
	}
}

func runTemplateList(args []string, stdout io.Writer, mode outMode) error {
	fs := flag.NewFlagSet("template list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := parseArgs(fs, args); err != nil {
		return userErr("bad_flags", err.Error())
	}
	if fs.NArg() != 0 {
		return userErr("usage", "usage: lt template list")
	}

	entries, err := listTemplates()
	if err != nil {
		return err
	}

	if mode == modeJSON {
		return writeJSON(stdout, map[string]any{"templates": entries})
	}

	if len(entries) == 0 {
		fmt.Fprintln(stdout, "No templates.")
		return nil
	}
	for _, e := range entries {
		switch e.Scope {
		case "global":
			fmt.Fprintf(stdout, "%s\n", e.Name)
		case "project":
			fmt.Fprintf(stdout, "%s (project: %s)\n", e.Name, e.Project)
		}
	}
	return nil
}
