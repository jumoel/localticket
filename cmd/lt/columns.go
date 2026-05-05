// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dustin/go-humanize"
)

// defaultColumns is the column set used when --columns is not passed.
var defaultColumns = []string{"id", "status", "title", "labels", "updated_at"}

// columnHeaders maps a column key to its TTY header label.
var columnHeaders = map[string]string{
	"id":         "ID",
	"title":      "TITLE",
	"status":     "STATUS",
	"labels":     "LABELS",
	"links":      "LINKS",
	"updated_at": "UPDATED",
	"created_at": "CREATED",
	"closed_at":  "CLOSED",
}

// parseColumns splits a comma-separated --columns value, trims whitespace,
// validates each token, and returns the resulting list. An empty input
// returns defaultColumns.
func parseColumns(spec string) ([]string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return defaultColumns, nil
	}
	parts := strings.Split(spec, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		c := strings.TrimSpace(p)
		if c == "" {
			continue
		}
		if _, ok := columnHeaders[c]; !ok {
			return nil, userErr("bad_column", fmt.Sprintf("unknown column %q (want id|title|status|labels|links|updated_at|created_at|closed_at)", c))
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return defaultColumns, nil
	}
	return out, nil
}

// renderTicketTable prints tickets as a TTY-friendly table, projecting only
// the requested columns. Long titles are truncated.
func renderTicketTable(w io.Writer, tickets []*ticket, columns []string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	headers := make([]string, len(columns))
	for i, c := range columns {
		headers[i] = columnHeaders[c]
	}
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, t := range tickets {
		cells := make([]string, len(columns))
		for i, c := range columns {
			cells[i] = ticketCell(t, c)
		}
		fmt.Fprintln(tw, strings.Join(cells, "\t"))
	}
	return tw.Flush()
}

func ticketCell(t *ticket, column string) string {
	switch column {
	case "id":
		return fmt.Sprintf("#%d", t.ID)
	case "title":
		return truncateRunes(t.Title, 60)
	case "status":
		return t.Status
	case "labels":
		return strings.Join(t.Labels, ",")
	case "links":
		if len(t.Links) == 0 {
			return ""
		}
		parts := make([]string, len(t.Links))
		for i, l := range t.Links {
			parts[i] = fmt.Sprintf("%s:#%d", l.Type, l.Target)
		}
		return strings.Join(parts, ",")
	case "updated_at":
		return relativeTime(t.UpdatedAt)
	case "created_at":
		return relativeTime(t.CreatedAt)
	case "closed_at":
		if t.ClosedAt == nil {
			return "-"
		}
		return relativeTime(*t.ClosedAt)
	}
	return ""
}

// relativeTime formats an RFC3339 timestamp as "2 hours ago" / "5 days ago".
// On parse failure it returns the raw input so the user still sees something.
func relativeTime(rfc3339 string) string {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return rfc3339
	}
	return humanize.Time(t)
}
