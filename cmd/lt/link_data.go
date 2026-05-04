package main

import (
	"database/sql"
	"errors"
	"fmt"
)

var allowedLinkTypes = map[string]bool{
	"blocks":       true,
	"blocked-by":   true,
	"parent":       true,
	"child":        true,
	"duplicate-of": true,
	"related":      true,
}

// canonicalizeLink maps a user-facing link type to the canonical type stored in
// the schema, plus a flag indicating whether the from/to endpoints must be
// swapped to express the inverse direction.
func canonicalizeLink(displayType string) (canonical string, swap bool, ok bool) {
	switch displayType {
	case "blocks":
		return "blocks", false, true
	case "blocked-by":
		return "blocks", true, true
	case "parent":
		return "parent", false, true
	case "child":
		return "parent", true, true
	case "duplicate-of":
		return "duplicate-of", false, true
	case "related":
		return "related", false, true
	default:
		return "", false, false
	}
}

func (s *store) addLink(projectName string, fromNum, toNum int64, displayType string) error {
	if !allowedLinkTypes[displayType] {
		return userErr("bad_type", fmt.Sprintf("invalid link type %q (want blocks|blocked-by|parent|child|duplicate-of|related)", displayType))
	}
	if fromNum == toNum {
		return userErr("self_link", "cannot link a ticket to itself")
	}
	canonical, swap, _ := canonicalizeLink(displayType)

	p, err := s.getProject(projectName)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return internalErr(err)
	}
	defer tx.Rollback()

	fromID, err := lookupTicketID(tx, p.ID, fromNum, projectName)
	if err != nil {
		return err
	}
	toID, err := lookupTicketID(tx, p.ID, toNum, projectName)
	if err != nil {
		return err
	}

	srcID, dstID := fromID, toID
	if swap {
		srcID, dstID = toID, fromID
	}

	if _, err := tx.Exec(`INSERT INTO ticket_links(from_id, to_id, type) VALUES (?, ?, ?)`, srcID, dstID, canonical); err != nil {
		if isUniqueViolation(err) {
			return conflict("link_exists", fmt.Sprintf("link already exists between #%d and #%d", fromNum, toNum))
		}
		return internalErr(err)
	}

	now := nowUTC()
	if _, err := tx.Exec(`UPDATE tickets SET updated_at = ? WHERE id IN (?, ?)`, now, fromID, toID); err != nil {
		return internalErr(err)
	}
	if err := tx.Commit(); err != nil {
		return internalErr(err)
	}
	return nil
}

func (s *store) removeLink(projectName string, aNum, bNum int64) error {
	if aNum == bNum {
		return userErr("self_link", "cannot link a ticket to itself")
	}
	p, err := s.getProject(projectName)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return internalErr(err)
	}
	defer tx.Rollback()

	aID, err := lookupTicketID(tx, p.ID, aNum, projectName)
	if err != nil {
		return err
	}
	bID, err := lookupTicketID(tx, p.ID, bNum, projectName)
	if err != nil {
		return err
	}

	res, err := tx.Exec(`DELETE FROM ticket_links WHERE (from_id = ? AND to_id = ?) OR (from_id = ? AND to_id = ?)`,
		aID, bID, bID, aID)
	if err != nil {
		return internalErr(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return internalErr(err)
	}
	if n == 0 {
		return notFound(fmt.Sprintf("no link between #%d and #%d", aNum, bNum))
	}

	if _, err := tx.Exec(`UPDATE tickets SET updated_at = ? WHERE id IN (?, ?)`, nowUTC(), aID, bID); err != nil {
		return internalErr(err)
	}
	if err := tx.Commit(); err != nil {
		return internalErr(err)
	}
	return nil
}

func lookupTicketID(tx *sql.Tx, projectID, num int64, projectName string) (int64, error) {
	var id int64
	err := tx.QueryRow(`SELECT id FROM tickets WHERE project_id = ? AND num = ?`, projectID, num).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, notFound(fmt.Sprintf("ticket #%d not found in project %q", num, projectName))
	}
	if err != nil {
		return 0, internalErr(err)
	}
	return id, nil
}
