package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

const swiftbarSFSymbol = "list.bullet.clipboard"
const swiftbarProjectCap = 10

func runSummaryImpl(args []string, stdout io.Writer, mode outMode) error {
	fs := flag.NewFlagSet("summary", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	swiftbar := fs.Bool("swiftbar", false, "emit SwiftBar plugin format")
	if err := parseArgs(fs, args); err != nil {
		return userErr("bad_flags", err.Error())
	}
	if fs.NArg() != 0 {
		return userErr("usage", "usage: lt summary [--swiftbar]")
	}

	s, err := openDefaultStore()
	if err != nil {
		return err
	}
	defer s.Close()

	sum, err := s.summarize()
	if err != nil {
		return err
	}

	if *swiftbar {
		return renderSwiftbar(stdout, sum)
	}
	if mode == modeJSON {
		return writeJSON(stdout, sum)
	}
	return renderSummaryPretty(stdout, sum)
}

func renderSummaryPretty(w io.Writer, sum *summary) error {
	if sum.Totals.Projects == 0 {
		fmt.Fprintln(w, "No projects.")
		return nil
	}
	fmt.Fprintf(w, "%d open, %d in-progress, %d closed across %d project(s)\n\n",
		sum.Totals.Open, sum.Totals.InProgress, sum.Totals.Closed, sum.Totals.Projects)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PROJECT\tOPEN\tIN-PROGRESS\tCLOSED\tLAST UPDATED")
	for _, p := range sum.Projects {
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%s\n", p.Name, p.Open, p.InProgress, p.Closed, p.LastUpdated)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if len(sum.Top) > 0 {
		fmt.Fprintln(w, "\nRecent open work:")
		for _, t := range sum.Top {
			fmt.Fprintf(w, "  %s#%d  %s  %s\n", t.Project, t.ID, t.Status, truncateRunes(t.Title, 60))
		}
	}
	return nil
}

func renderSwiftbar(w io.Writer, sum *summary) error {
	totalActive := sum.Totals.Open + sum.Totals.InProgress
	if totalActive == 0 {
		fmt.Fprintf(w, " | sfimage=%s\n", swiftbarSFSymbol)
	} else {
		fmt.Fprintf(w, "%d | sfimage=%s\n", totalActive, swiftbarSFSymbol)
	}
	fmt.Fprintln(w, "---")

	if sum.Totals.Projects == 0 {
		fmt.Fprintln(w, "No projects | color=gray")
		fmt.Fprintln(w, "---")
		fmt.Fprintln(w, "Refresh | refresh=true")
		return nil
	}

	shown := sum.Projects
	hidden := 0
	if len(shown) > swiftbarProjectCap {
		hidden = len(shown) - swiftbarProjectCap
		shown = shown[:swiftbarProjectCap]
	}
	for _, p := range shown {
		active := p.Open + p.InProgress
		if active == 0 {
			fmt.Fprintf(w, "%s: idle | color=gray\n", p.Name)
			continue
		}
		fmt.Fprintf(w, "%s: %d open, %d in-progress\n", p.Name, p.Open, p.InProgress)
	}
	if hidden > 0 {
		fmt.Fprintf(w, "... and %d more | color=gray\n", hidden)
	}

	if len(sum.Top) > 0 {
		fmt.Fprintln(w, "---")
		fmt.Fprintln(w, "Recent | color=gray")
		for _, t := range sum.Top {
			renderSwiftbarTicket(w, t)
		}
	}

	fmt.Fprintln(w, "---")
	fmt.Fprintln(w, "Refresh | refresh=true")
	return nil
}

// swiftbarSafe scrubs characters that would break SwiftBar's line format.
// '|' starts a parameter section, and CR/LF split one menu item into many.
// SwiftBar has no escape syntax for these, so we replace them with neutrals.
func swiftbarSafe(s string) string {
	r := strings.NewReplacer("|", "¦", "\r", " ", "\n", " ")
	return r.Replace(s)
}

const swiftbarBodyLineLimit = 25

// SwiftBar renders submenu items with a muted default text color, so we
// explicitly set color/colorDark to system-label values to make body and
// metadata read at full contrast in both light and dark mode.
const swiftbarSubmenuColor = " color=black colorDark=white"
const swiftbarSubmenuMonoStyle = " font=Menlo color=black colorDark=white"

// renderSwiftbarTicket emits one Recent row plus a submenu (lines prefixed
// with `--`) showing status, labels, links, and the body. Body lines beyond
// swiftbarBodyLineLimit are dropped with a trailing "(... N more lines)".
func renderSwiftbarTicket(w io.Writer, t topTicket) {
	fmt.Fprintf(w, "%s#%d  %s | font=Menlo\n", t.Project, t.ID, swiftbarSafe(truncateRunes(t.Title, 60)))
	fmt.Fprintf(w, "--Status: %s |%s\n", t.Status, swiftbarSubmenuColor)
	if len(t.Labels) > 0 {
		fmt.Fprintf(w, "--Labels: %s |%s\n", swiftbarSafe(strings.Join(t.Labels, ", ")), swiftbarSubmenuColor)
	}
	if len(t.Links) > 0 {
		parts := make([]string, len(t.Links))
		for i, l := range t.Links {
			parts[i] = fmt.Sprintf("%s #%d", l.Type, l.Target)
		}
		fmt.Fprintf(w, "--Links: %s |%s\n", swiftbarSafe(strings.Join(parts, ", ")), swiftbarSubmenuColor)
	}
	fmt.Fprintf(w, "--Updated: %s |%s\n", t.UpdatedAt, swiftbarSubmenuColor)
	body := strings.TrimRight(t.Body, "\n")
	if body == "" {
		return
	}
	fmt.Fprintln(w, "-----")
	lines := strings.Split(body, "\n")
	shown := lines
	hidden := 0
	if len(shown) > swiftbarBodyLineLimit {
		hidden = len(shown) - swiftbarBodyLineLimit
		shown = shown[:swiftbarBodyLineLimit]
	}
	for _, line := range shown {
		fmt.Fprintf(w, "--%s |%s\n", swiftbarSafe(line), swiftbarSubmenuMonoStyle)
	}
	if hidden > 0 {
		fmt.Fprintf(w, "--(... %d more lines) | color=gray\n", hidden)
	}
}
