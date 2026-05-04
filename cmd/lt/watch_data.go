package main

import (
	"database/sql"
)

type linkKey struct {
	Type   string
	Target int64
}

type snapshot struct {
	Project   string
	Num       int64
	Status    string
	Title     string
	Body      string
	Labels    map[string]bool
	Links     map[linkKey]bool
	UpdatedAt string
	CreatedAt string
}

// snapshotTickets returns one snapshot per ticket whose updated_at is strictly
// before cursorTime, so any ticket modified at or after the cursor becomes a
// first-sighting in the diff and emits a real event rather than a no-op
// "updated" against its own final state.
func (s *store) snapshotTickets(project *string, cursorTime string) (map[int64]*snapshot, error) {
	args := []any{cursorTime}
	where := "WHERE t.updated_at < ?"
	if project != nil {
		where += " AND p.name = ?"
		args = append(args, *project)
	}
	q := `
		SELECT t.id, p.name, t.num, t.status, t.title, t.body, t.created_at, t.updated_at
		FROM tickets t
		JOIN projects p ON p.id = t.project_id
		` + where
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, internalErr(err)
	}
	defer rows.Close()
	out := map[int64]*snapshot{}
	for rows.Next() {
		var id int64
		var snap snapshot
		if err := rows.Scan(&id, &snap.Project, &snap.Num, &snap.Status, &snap.Title, &snap.Body, &snap.CreatedAt, &snap.UpdatedAt); err != nil {
			return nil, internalErr(err)
		}
		snap.Labels = map[string]bool{}
		snap.Links = map[linkKey]bool{}
		out[id] = &snap
	}
	if err := rows.Err(); err != nil {
		return nil, internalErr(err)
	}
	if err := s.fillLabels(out, project); err != nil {
		return nil, err
	}
	if err := s.fillLinks(out, project); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *store) fillLabels(snaps map[int64]*snapshot, project *string) error {
	args := []any{}
	where := ""
	if project != nil {
		where = "AND p.name = ?"
		args = append(args, *project)
	}
	q := `
		SELECT tl.ticket_id, tl.label
		FROM ticket_labels tl
		JOIN tickets t ON t.id = tl.ticket_id
		JOIN projects p ON p.id = t.project_id
		WHERE 1=1 ` + where
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return internalErr(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var label string
		if err := rows.Scan(&id, &label); err != nil {
			return internalErr(err)
		}
		if snap, ok := snaps[id]; ok {
			snap.Labels[label] = true
		}
	}
	return rows.Err()
}

func (s *store) fillLinks(snaps map[int64]*snapshot, project *string) error {
	args := []any{}
	where := ""
	if project != nil {
		where = "AND p.name = ?"
		args = append(args, *project)
	}
	// Outgoing links: from this ticket → other.
	qOut := `
		SELECT l.from_id, l.type, other.num
		FROM ticket_links l
		JOIN tickets other ON other.id = l.to_id
		JOIN tickets t ON t.id = l.from_id
		JOIN projects p ON p.id = t.project_id
		WHERE 1=1 ` + where
	if err := s.fillLinkRows(snaps, qOut, args, false); err != nil {
		return err
	}
	// Incoming links rendered as inverse types on the receiving ticket.
	qIn := `
		SELECT l.to_id, l.type, other.num
		FROM ticket_links l
		JOIN tickets other ON other.id = l.from_id
		JOIN tickets t ON t.id = l.to_id
		JOIN projects p ON p.id = t.project_id
		WHERE 1=1 ` + where
	return s.fillLinkRows(snaps, qIn, args, true)
}

func (s *store) fillLinkRows(snaps map[int64]*snapshot, q string, args []any, inverse bool) error {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return internalErr(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, num int64
		var typ string
		if err := rows.Scan(&id, &typ, &num); err != nil {
			return internalErr(err)
		}
		if inverse {
			typ = inverseLinkType(typ)
		}
		if snap, ok := snaps[id]; ok {
			snap.Links[linkKey{Type: typ, Target: num}] = true
		}
	}
	return rows.Err()
}

type cursor struct {
	UpdatedAt string
	ID        int64
}

// changedTicketsSince returns tickets whose updated_at advanced past the cursor,
// ordered so callers can update the cursor to the last row's (updated_at, id).
func (s *store) changedTicketsSince(cur cursor, project *string) (map[int64]*snapshot, []int64, error) {
	args := []any{cur.UpdatedAt, cur.UpdatedAt, cur.ID}
	pf := ""
	if project != nil {
		pf = " AND p.name = ?"
		args = append(args, *project)
	}
	q := `
		SELECT t.id, p.name, t.num, t.status, t.title, t.body, t.created_at, t.updated_at
		FROM tickets t
		JOIN projects p ON p.id = t.project_id
		WHERE (t.updated_at > ? OR (t.updated_at = ? AND t.id > ?))` + pf + `
		ORDER BY t.updated_at, t.id`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, nil, internalErr(err)
	}
	defer rows.Close()

	snaps := map[int64]*snapshot{}
	order := []int64{}
	for rows.Next() {
		var id int64
		var snap snapshot
		if err := rows.Scan(&id, &snap.Project, &snap.Num, &snap.Status, &snap.Title, &snap.Body, &snap.CreatedAt, &snap.UpdatedAt); err != nil {
			return nil, nil, internalErr(err)
		}
		snap.Labels = map[string]bool{}
		snap.Links = map[linkKey]bool{}
		snaps[id] = &snap
		order = append(order, id)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, internalErr(err)
	}
	if len(snaps) == 0 {
		return snaps, order, nil
	}
	if err := s.fillLabelsForIDs(snaps); err != nil {
		return nil, nil, err
	}
	if err := s.fillLinksForIDs(snaps); err != nil {
		return nil, nil, err
	}
	return snaps, order, nil
}

func (s *store) fillLabelsForIDs(snaps map[int64]*snapshot) error {
	for id, snap := range snaps {
		t := &ticket{internalID: id}
		if err := s.loadLabels(t); err != nil {
			return err
		}
		for _, l := range t.Labels {
			snap.Labels[l] = true
		}
	}
	return nil
}

func (s *store) fillLinksForIDs(snaps map[int64]*snapshot) error {
	for id, snap := range snaps {
		var projectID int64
		err := s.db.QueryRow(`SELECT project_id FROM tickets WHERE id = ?`, id).Scan(&projectID)
		if err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return internalErr(err)
		}
		t := &ticket{internalID: id}
		if err := s.loadLinks(t, projectID); err != nil {
			return err
		}
		for _, l := range t.Links {
			snap.Links[linkKey{Type: l.Type, Target: l.Target}] = true
		}
	}
	return nil
}
