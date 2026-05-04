package main

import (
	"database/sql"
	"strings"
)

func (s *store) searchTickets(projectName, query string) ([]*ticket, error) {
	p, err := s.getProject(projectName)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`
		SELECT t.id, t.num, t.title, t.body, t.status, t.created_at, t.updated_at, t.closed_at
		FROM tickets_fts f
		JOIN tickets t ON t.id = f.rowid
		WHERE f.tickets_fts MATCH ? AND t.project_id = ?
		ORDER BY bm25(f.tickets_fts)`, query, p.ID)
	if err != nil {
		return nil, wrapFTSError(err)
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
		return nil, wrapFTSError(err)
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

func wrapFTSError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	low := strings.ToLower(msg)
	if strings.Contains(low, "fts") || strings.Contains(msg, "MATCH") || strings.Contains(low, "match") {
		return userErr("bad_query", msg)
	}
	return internalErr(err)
}
