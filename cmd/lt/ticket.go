package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type ticket struct {
	Project   string   `json:"project"`
	ID        int64    `json:"id"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Status    string   `json:"status"`
	Labels    []string `json:"labels"`
	Links     []link   `json:"links"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	ClosedAt  *string  `json:"closed_at"`

	internalID int64 `json:"-"`
}

type link struct {
	Type   string `json:"type"`
	Target int64  `json:"target"`
}

func (s *store) createTicket(projectName, title, body string) (*ticket, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, userErr("empty_title", "title must be non-empty")
	}
	p, err := s.getProject(projectName)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, internalErr(err)
	}
	defer tx.Rollback()

	var num int64
	err = tx.QueryRow(`UPDATE projects SET next_ticket_num = next_ticket_num + 1 WHERE id = ? RETURNING next_ticket_num - 1`, p.ID).Scan(&num)
	if err != nil {
		return nil, internalErr(fmt.Errorf("allocate ticket num: %w", err))
	}

	now := nowUTC()
	res, err := tx.Exec(`INSERT INTO tickets(project_id, num, title, body, status, created_at, updated_at) VALUES (?, ?, ?, ?, 'open', ?, ?)`,
		p.ID, num, title, body, now, now)
	if err != nil {
		return nil, internalErr(err)
	}
	id, _ := res.LastInsertId()
	if err := tx.Commit(); err != nil {
		return nil, internalErr(err)
	}
	return &ticket{
		Project:    p.Name,
		ID:         num,
		Title:      title,
		Body:       body,
		Status:     "open",
		Labels:     []string{},
		Links:      []link{},
		CreatedAt:  now,
		UpdatedAt:  now,
		internalID: id,
	}, nil
}

func (s *store) getTicket(projectName string, num int64) (*ticket, error) {
	p, err := s.getProject(projectName)
	if err != nil {
		return nil, err
	}
	t := &ticket{Project: p.Name, ID: num, Labels: []string{}, Links: []link{}}
	var closedAt sql.NullString
	err = s.db.QueryRow(`SELECT id, title, body, status, created_at, updated_at, closed_at FROM tickets WHERE project_id = ? AND num = ?`, p.ID, num).
		Scan(&t.internalID, &t.Title, &t.Body, &t.Status, &t.CreatedAt, &t.UpdatedAt, &closedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, notFound(fmt.Sprintf("ticket #%d not found in project %q", num, projectName))
	}
	if err != nil {
		return nil, internalErr(err)
	}
	if closedAt.Valid {
		t.ClosedAt = &closedAt.String
	}
	if err := s.loadLabels(t); err != nil {
		return nil, err
	}
	if err := s.loadLinks(t, p.ID); err != nil {
		return nil, err
	}
	return t, nil
}

// loadLabels populates t.Labels in alphabetical order.
func (s *store) loadLabels(t *ticket) error {
	rows, err := s.db.Query(`SELECT label FROM ticket_labels WHERE ticket_id = ? ORDER BY label`, t.internalID)
	if err != nil {
		return internalErr(err)
	}
	defer rows.Close()
	t.Labels = t.Labels[:0]
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			return internalErr(err)
		}
		t.Labels = append(t.Labels, l)
	}
	if t.Labels == nil {
		t.Labels = []string{}
	}
	return rows.Err()
}

// loadLinks populates t.Links with both directions: outgoing rows as-stored, plus
// virtual inverses for each row that points at this ticket. Inverse type mapping:
// blocks <-> blocked-by, parent <-> child, duplicate-of and related stay as-is.
func (s *store) loadLinks(t *ticket, projectID int64) error {
	t.Links = t.Links[:0]

	rows, err := s.db.Query(`
		SELECT l.type, other.num
		FROM ticket_links l
		JOIN tickets other ON other.id = l.to_id
		WHERE l.from_id = ? AND other.project_id = ?
		ORDER BY l.type, other.num`, t.internalID, projectID)
	if err != nil {
		return internalErr(err)
	}
	defer rows.Close()
	for rows.Next() {
		var typ string
		var num int64
		if err := rows.Scan(&typ, &num); err != nil {
			return internalErr(err)
		}
		t.Links = append(t.Links, link{Type: typ, Target: num})
	}
	if err := rows.Err(); err != nil {
		return internalErr(err)
	}

	rows2, err := s.db.Query(`
		SELECT l.type, other.num
		FROM ticket_links l
		JOIN tickets other ON other.id = l.from_id
		WHERE l.to_id = ? AND other.project_id = ?
		ORDER BY l.type, other.num`, t.internalID, projectID)
	if err != nil {
		return internalErr(err)
	}
	defer rows2.Close()
	for rows2.Next() {
		var typ string
		var num int64
		if err := rows2.Scan(&typ, &num); err != nil {
			return internalErr(err)
		}
		t.Links = append(t.Links, link{Type: inverseLinkType(typ), Target: num})
	}
	if t.Links == nil {
		t.Links = []link{}
	}
	return rows2.Err()
}

// inverseLinkType maps a stored canonical link type to the type a ticket on the
// receiving end should display. blocks-style relationships are directional;
// duplicate-of and related are symmetric.
func inverseLinkType(canonical string) string {
	switch canonical {
	case "blocks":
		return "blocked-by"
	case "parent":
		return "child"
	default:
		return canonical
	}
}
