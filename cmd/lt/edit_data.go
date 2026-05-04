package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var allowedStatuses = map[string]bool{
	"open":        true,
	"in-progress": true,
	"closed":      true,
}

func (s *store) setTicketStatus(projectName string, num int64, newStatus string) (bool, error) {
	if !allowedStatuses[newStatus] {
		return false, userErr("bad_status", fmt.Sprintf("invalid status %q (want open|in-progress|closed)", newStatus))
	}
	p, err := s.getProject(projectName)
	if err != nil {
		return false, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, internalErr(err)
	}
	defer tx.Rollback()

	var ticketID int64
	var current string
	err = tx.QueryRow(`SELECT id, status FROM tickets WHERE project_id = ? AND num = ?`, p.ID, num).
		Scan(&ticketID, &current)
	if errors.Is(err, sql.ErrNoRows) {
		return false, notFound(fmt.Sprintf("ticket #%d not found in project %q", num, projectName))
	}
	if err != nil {
		return false, internalErr(err)
	}

	if current == newStatus {
		return false, nil
	}

	now := nowUTC()
	var closedAt sql.NullString
	switch {
	case newStatus == "closed":
		closedAt = sql.NullString{String: now, Valid: true}
	case current == "closed":
		closedAt = sql.NullString{Valid: false}
	default:
		// neither side is closed; preserve existing closed_at (which should be NULL).
		var existing sql.NullString
		if err := tx.QueryRow(`SELECT closed_at FROM tickets WHERE id = ?`, ticketID).Scan(&existing); err != nil {
			return false, internalErr(err)
		}
		closedAt = existing
	}

	if _, err := tx.Exec(`UPDATE tickets SET status = ?, updated_at = ?, closed_at = ? WHERE id = ?`,
		newStatus, now, closedAt, ticketID); err != nil {
		return false, internalErr(err)
	}
	if err := tx.Commit(); err != nil {
		return false, internalErr(err)
	}
	return true, nil
}

func (s *store) updateTicket(projectName string, num int64, newTitle *string, newBody *string) (bool, error) {
	if newTitle == nil && newBody == nil {
		return false, nil
	}
	if newTitle != nil {
		trimmed := strings.TrimSpace(*newTitle)
		if trimmed == "" {
			return false, userErr("empty_title", "title must be non-empty")
		}
		t := trimmed
		newTitle = &t
	}
	p, err := s.getProject(projectName)
	if err != nil {
		return false, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, internalErr(err)
	}
	defer tx.Rollback()

	var ticketID int64
	var curTitle, curBody string
	err = tx.QueryRow(`SELECT id, title, body FROM tickets WHERE project_id = ? AND num = ?`, p.ID, num).
		Scan(&ticketID, &curTitle, &curBody)
	if errors.Is(err, sql.ErrNoRows) {
		return false, notFound(fmt.Sprintf("ticket #%d not found in project %q", num, projectName))
	}
	if err != nil {
		return false, internalErr(err)
	}

	titleChanges := newTitle != nil && *newTitle != curTitle
	bodyChanges := newBody != nil && *newBody != curBody
	if !titleChanges && !bodyChanges {
		return false, nil
	}

	var sets []string
	var args []any
	if titleChanges {
		sets = append(sets, "title = ?")
		args = append(args, *newTitle)
	}
	if bodyChanges {
		sets = append(sets, "body = ?")
		args = append(args, *newBody)
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, nowUTC())
	args = append(args, ticketID)

	q := "UPDATE tickets SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	if _, err := tx.Exec(q, args...); err != nil {
		return false, internalErr(err)
	}
	if err := tx.Commit(); err != nil {
		return false, internalErr(err)
	}
	return true, nil
}
