package main

import (
	"database/sql"
	"strings"
)

func (s *store) listTickets(projectName string, statuses []string, labels []string) ([]*ticket, error) {
	p, err := s.getProject(projectName)
	if err != nil {
		return nil, err
	}

	var sb strings.Builder
	sb.WriteString(`SELECT t.id, t.num, t.title, t.body, t.status, t.created_at, t.updated_at, t.closed_at
		FROM tickets t`)
	args := []any{}

	if len(labels) > 0 {
		sb.WriteString(` JOIN ticket_labels tl ON tl.ticket_id = t.id`)
	}

	sb.WriteString(` WHERE t.project_id = ?`)
	args = append(args, p.ID)

	if len(statuses) > 0 {
		sb.WriteString(` AND t.status IN (`)
		for i, st := range statuses {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("?")
			args = append(args, st)
		}
		sb.WriteString(`)`)
	}

	if len(labels) > 0 {
		sb.WriteString(` AND tl.label IN (`)
		for i, l := range labels {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("?")
			args = append(args, l)
		}
		sb.WriteString(`) GROUP BY t.id HAVING COUNT(DISTINCT tl.label) = ?`)
		args = append(args, len(labels))
	}

	sb.WriteString(` ORDER BY t.updated_at DESC, t.num DESC`)

	rows, err := s.db.Query(sb.String(), args...)
	if err != nil {
		return nil, internalErr(err)
	}
	defer rows.Close()

	var out []*ticket
	for rows.Next() {
		t := &ticket{Project: p.Name, Labels: []string{}, Links: []link{}}
		var closedAt sql.NullString
		if err := rows.Scan(&t.internalID, &t.ID, &t.Title, &t.Body, &t.Status, &t.CreatedAt, &t.UpdatedAt, &closedAt); err != nil {
			return nil, internalErr(err)
		}
		if closedAt.Valid {
			t.ClosedAt = &closedAt.String
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, internalErr(err)
	}

	for _, t := range out {
		if err := s.loadLabels(t); err != nil {
			return nil, err
		}
		if err := s.loadLinks(t, p.ID); err != nil {
			return nil, err
		}
	}
	return out, nil
}
