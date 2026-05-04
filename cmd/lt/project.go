package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type projectRow struct {
	ID            int64
	Name          string
	NextTicketNum int64
	CreatedAt     string
}

type projectSummary struct {
	Name      string         `json:"name"`
	CreatedAt string         `json:"created_at"`
	Tickets   map[string]int `json:"tickets"`
}

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

func (s *store) createProject(name string) (*projectRow, error) {
	now := nowUTC()
	res, err := s.db.Exec(`INSERT INTO projects(name, created_at) VALUES (?, ?)`, name, now)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, conflict("project_exists", fmt.Sprintf("project %q already exists", name))
		}
		return nil, internalErr(err)
	}
	id, _ := res.LastInsertId()
	return &projectRow{ID: id, Name: name, NextTicketNum: 1, CreatedAt: now}, nil
}

func (s *store) getProject(name string) (*projectRow, error) {
	var p projectRow
	err := s.db.QueryRow(`SELECT id, name, next_ticket_num, created_at FROM projects WHERE name = ?`, name).
		Scan(&p.ID, &p.Name, &p.NextTicketNum, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, notFound(fmt.Sprintf("project %q not found", name))
	}
	if err != nil {
		return nil, internalErr(err)
	}
	return &p, nil
}

func (s *store) listProjects() ([]projectSummary, error) {
	rows, err := s.db.Query(`
		SELECT p.name, p.created_at,
		       COALESCE(SUM(CASE WHEN t.status='open' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN t.status='in-progress' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN t.status='closed' THEN 1 ELSE 0 END), 0)
		FROM projects p
		LEFT JOIN tickets t ON t.project_id = p.id
		GROUP BY p.id
		ORDER BY p.name`)
	if err != nil {
		return nil, internalErr(err)
	}
	defer rows.Close()
	var out []projectSummary
	for rows.Next() {
		var ps projectSummary
		var open, inProg, closed int
		if err := rows.Scan(&ps.Name, &ps.CreatedAt, &open, &inProg, &closed); err != nil {
			return nil, internalErr(err)
		}
		ps.Tickets = map[string]int{"open": open, "in-progress": inProg, "closed": closed}
		out = append(out, ps)
	}
	return out, rows.Err()
}

func (s *store) deleteProject(name string, force bool) error {
	p, err := s.getProject(name)
	if err != nil {
		return err
	}
	if !force {
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM tickets WHERE project_id = ? AND status != 'closed'`, p.ID).Scan(&n); err != nil {
			return internalErr(err)
		}
		if n > 0 {
			return conflict("has_open_tickets", fmt.Sprintf("project %q has %d non-closed ticket(s); pass --force to delete anyway", name, n))
		}
	}
	if _, err := s.db.Exec(`DELETE FROM projects WHERE id = ?`, p.ID); err != nil {
		return internalErr(err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
