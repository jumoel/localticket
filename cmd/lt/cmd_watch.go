package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	watchDefaultInterval = 2 * time.Second
	watchMinInterval     = 500 * time.Millisecond
)

type watchEvent struct {
	ObservedAt string `json:"observed_at"`
	Action     string `json:"action"`
	Project    string `json:"project"`
	ID         int64  `json:"id"`
	Title      string `json:"title,omitempty"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
	Label      string `json:"label,omitempty"`
	LinkType   string `json:"link_type,omitempty"`
	LinkTarget int64  `json:"link_target,omitempty"`
	Body       string `json:"body,omitempty"`
}

func runWatchImpl(args []string, stdout io.Writer, mode outMode) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	project := projectFlag(fs)
	since := fs.String("since", "", "RFC3339 cursor; default = now")
	intervalStr := fs.String("interval", watchDefaultInterval.String(), "poll interval")
	once := fs.Bool("once", false, "emit pending events and exit (for tests)")
	if err := parseArgs(fs, args); err != nil {
		return userErr("bad_flags", err.Error())
	}
	if fs.NArg() != 0 {
		return userErr("usage", "usage: lt watch [-p <project>] [--since RFC3339] [--interval 2s] [--once]")
	}
	var pf *string
	if *project != "" {
		if err := validateProjectName(*project); err != nil {
			return err
		}
		pf = project
	}
	interval, err := time.ParseDuration(*intervalStr)
	if err != nil {
		return userErr("bad_interval", fmt.Sprintf("invalid --interval: %v", err))
	}
	if interval < watchMinInterval {
		interval = watchMinInterval
	}

	cursorTime := nowUTC()
	if *since != "" {
		t, err := time.Parse(time.RFC3339, *since)
		if err != nil {
			return userErr("bad_since", fmt.Sprintf("invalid --since: %v", err))
		}
		cursorTime = t.UTC().Format(time.RFC3339)
	}
	cur := cursor{UpdatedAt: cursorTime, ID: 0}

	s, err := openDefaultStore()
	if err != nil {
		return err
	}
	defer s.Close()

	cache, err := s.snapshotTickets(pf, cursorTime)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if !*once {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigCh
			cancel()
		}()
	}

	emit := func(events []watchEvent) error {
		for _, e := range events {
			if mode == modeJSON {
				if err := writeJSON(stdout, e); err != nil {
					return err
				}
				continue
			}
			fmt.Fprintln(stdout, formatWatchEvent(e))
		}
		return nil
	}

	tick := func() error {
		snaps, order, err := s.changedTicketsSince(cur, pf)
		if err != nil {
			return err
		}
		now := nowUTC()
		var events []watchEvent
		for _, id := range order {
			snap := snaps[id]
			prior, hadPrior := cache[id]
			ev := diffSnapshot(prior, hadPrior, snap, now)
			events = append(events, ev...)
			cache[id] = snap
			cur = cursor{UpdatedAt: snap.UpdatedAt, ID: id}
		}
		return emit(events)
	}

	if *once {
		return tick()
	}

	if err := tick(); err != nil {
		return err
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			if err := tick(); err != nil {
				return err
			}
		}
	}
}

// diffSnapshot returns the events that explain the transition from prior to
// next. If the ticket was never seen, hadPrior is false; emit a single
// `created` (or `updated` if the ticket pre-dates the watcher).
func diffSnapshot(prior *snapshot, hadPrior bool, next *snapshot, observedAt string) []watchEvent {
	base := watchEvent{
		ObservedAt: observedAt,
		Project:    next.Project,
		ID:         next.Num,
		Title:      next.Title,
	}
	if !hadPrior {
		ev := base
		if next.CreatedAt == next.UpdatedAt {
			ev.Action = "created"
		} else {
			ev.Action = "updated"
		}
		return []watchEvent{ev}
	}

	var out []watchEvent
	if prior.Status != next.Status {
		ev := base
		ev.From = prior.Status
		ev.To = next.Status
		switch {
		case next.Status == "closed":
			ev.Action = "closed"
		case prior.Status == "closed":
			ev.Action = "reopened"
		default:
			ev.Action = "status_changed"
		}
		out = append(out, ev)
	}
	if prior.Title != next.Title {
		ev := base
		ev.Action = "title_changed"
		ev.From = prior.Title
		ev.To = next.Title
		out = append(out, ev)
	}
	if prior.Body != next.Body {
		ev := base
		ev.Action = "body_changed"
		ev.Body = next.Body
		out = append(out, ev)
	}
	for l := range next.Labels {
		if !prior.Labels[l] {
			ev := base
			ev.Action = "label_added"
			ev.Label = l
			out = append(out, ev)
		}
	}
	for l := range prior.Labels {
		if !next.Labels[l] {
			ev := base
			ev.Action = "label_removed"
			ev.Label = l
			out = append(out, ev)
		}
	}
	for k := range next.Links {
		if !prior.Links[k] {
			ev := base
			ev.Action = "link_added"
			ev.LinkType = k.Type
			ev.LinkTarget = k.Target
			out = append(out, ev)
		}
	}
	for k := range prior.Links {
		if !next.Links[k] {
			ev := base
			ev.Action = "link_removed"
			ev.LinkType = k.Type
			ev.LinkTarget = k.Target
			out = append(out, ev)
		}
	}
	if len(out) == 0 {
		// updated_at advanced but nothing observable changed (rare; e.g. a
		// noop edit of the same body). Emit a generic updated marker so the
		// watcher doesn't look frozen.
		ev := base
		ev.Action = "updated"
		out = append(out, ev)
	}
	sort.SliceStable(out, func(i, j int) bool { return watchActionOrder(out[i].Action) < watchActionOrder(out[j].Action) })
	return out
}

// watchActionOrder controls the order of multiple events emitted in the same
// tick for one ticket, so a viewer reads them top-to-bottom in natural order:
// status flips first, then content, then tag/link adjustments.
func watchActionOrder(action string) int {
	switch action {
	case "created":
		return 0
	case "reopened":
		return 1
	case "status_changed":
		return 2
	case "closed":
		return 3
	case "title_changed":
		return 4
	case "body_changed":
		return 5
	case "label_added":
		return 6
	case "label_removed":
		return 7
	case "link_added":
		return 8
	case "link_removed":
		return 9
	default:
		return 10
	}
}

func formatWatchEvent(e watchEvent) string {
	t := e.ObservedAt
	if parsed, err := time.Parse(time.RFC3339, e.ObservedAt); err == nil {
		t = parsed.Local().Format("15:04:05")
	}
	tag := fmt.Sprintf("%s#%d", e.Project, e.ID)
	switch e.Action {
	case "label_added":
		return fmt.Sprintf("%s  %s  label +%s", t, tag, e.Label)
	case "label_removed":
		return fmt.Sprintf("%s  %s  label -%s", t, tag, e.Label)
	case "link_added":
		return fmt.Sprintf("%s  %s  link +%s #%d", t, tag, e.LinkType, e.LinkTarget)
	case "link_removed":
		return fmt.Sprintf("%s  %s  link -%s #%d", t, tag, e.LinkType, e.LinkTarget)
	case "status_changed":
		return fmt.Sprintf("%s  %s  status %s -> %s", t, tag, e.From, e.To)
	case "title_changed":
		return fmt.Sprintf("%s  %s  title %q -> %q", t, tag, e.From, e.To)
	case "body_changed":
		return fmt.Sprintf("%s  %s  body changed (%d chars)", t, tag, len(e.Body))
	default:
		title := e.Title
		if title != "" {
			title = "  " + truncateRunes(strings.ReplaceAll(title, "\n", " "), 60)
		}
		return fmt.Sprintf("%s  %s  %s%s", t, tag, e.Action, title)
	}
}
