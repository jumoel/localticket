package main

import (
	"database/sql"
	"errors"
	"fmt"
)

func (s *store) addLabels(projectName string, num int64, labels []string) (int, error) {
	p, err := s.getProject(projectName)
	if err != nil {
		return 0, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, internalErr(err)
	}
	defer tx.Rollback()

	var ticketID int64
	err = tx.QueryRow(`SELECT id FROM tickets WHERE project_id = ? AND num = ?`, p.ID, num).Scan(&ticketID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, notFound(fmt.Sprintf("ticket #%d not found in project %q", num, projectName))
	}
	if err != nil {
		return 0, internalErr(err)
	}

	added := 0
	for _, l := range labels {
		res, err := tx.Exec(`INSERT INTO ticket_labels(ticket_id, label) VALUES (?, ?) ON CONFLICT(ticket_id, label) DO NOTHING`, ticketID, l)
		if err != nil {
			return 0, internalErr(err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return 0, internalErr(err)
		}
		if n > 0 {
			added++
		}
	}

	if added > 0 {
		if _, err := tx.Exec(`UPDATE tickets SET updated_at = ? WHERE id = ?`, nowUTC(), ticketID); err != nil {
			return 0, internalErr(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, internalErr(err)
	}
	return added, nil
}

func (s *store) removeLabels(projectName string, num int64, labels []string) (int, error) {
	p, err := s.getProject(projectName)
	if err != nil {
		return 0, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, internalErr(err)
	}
	defer tx.Rollback()

	var ticketID int64
	err = tx.QueryRow(`SELECT id FROM tickets WHERE project_id = ? AND num = ?`, p.ID, num).Scan(&ticketID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, notFound(fmt.Sprintf("ticket #%d not found in project %q", num, projectName))
	}
	if err != nil {
		return 0, internalErr(err)
	}

	for _, l := range labels {
		var exists int
		err := tx.QueryRow(`SELECT 1 FROM ticket_labels WHERE ticket_id = ? AND label = ?`, ticketID, l).Scan(&exists)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, notFound(fmt.Sprintf("ticket #%d in project %q has no label %q", num, projectName, l))
		}
		if err != nil {
			return 0, internalErr(err)
		}
	}

	removed := 0
	for _, l := range labels {
		res, err := tx.Exec(`DELETE FROM ticket_labels WHERE ticket_id = ? AND label = ?`, ticketID, l)
		if err != nil {
			return 0, internalErr(err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return 0, internalErr(err)
		}
		removed += int(n)
	}

	if removed > 0 {
		if _, err := tx.Exec(`UPDATE tickets SET updated_at = ? WHERE id = ?`, nowUTC(), ticketID); err != nil {
			return 0, internalErr(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, internalErr(err)
	}
	return removed, nil
}
