package main

import (
	"database/sql"
	"errors"
	"fmt"
)

var allowedLinkTypes = map[string]bool{
	"blocks":        true,
	"blocked-by":    true,
	"parent":        true,
	"child":         true,
	"duplicate-of":  true,
	"duplicates":    true,
	"related":       true,
	"supersedes":    true,
	"superseded-by": true,
	"references":    true,
	"referenced-by": true,
	"derived-from":  true,
	"derived-to":    true,
}

// canonicalLinkTypes is the set of types stored in the schema. Inverse aliases
// like "blocked-by" are flipped to their canonical form on insert.
var canonicalLinkTypes = []string{
	"blocks", "parent", "duplicate-of", "related",
	"supersedes", "references", "derived-from",
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
	case "duplicates":
		return "duplicate-of", true, true
	case "related":
		return "related", false, true
	case "supersedes":
		return "supersedes", false, true
	case "superseded-by":
		return "supersedes", true, true
	case "references":
		return "references", false, true
	case "referenced-by":
		return "references", true, true
	case "derived-from":
		return "derived-from", false, true
	case "derived-to":
		return "derived-from", true, true
	default:
		return "", false, false
	}
}

func (s *store) addLink(projectName string, fromNum, toNum int64, displayType string) error {
	if !allowedLinkTypes[displayType] {
		return userErr("bad_type", fmt.Sprintf("invalid link type %q (want blocks|blocked-by|parent|child|duplicate-of|duplicates|related|supersedes|superseded-by|references|referenced-by|derived-from|derived-to)", displayType))
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

type projectLink struct {
	From int64  `json:"from"`
	To   int64  `json:"to"`
	Type string `json:"type"`
}

// listProjectLinks returns every link in the project, optionally filtered by a
// ticket (links touching that ticket on either side) and/or a canonical type.
// Closed-endpoint links are excluded unless includeClosed is true.
func (s *store) listProjectLinks(projectName string, ticketNum int64, linkType string, includeClosed bool) ([]projectLink, error) {
	p, err := s.getProject(projectName)
	if err != nil {
		return nil, err
	}

	q := `SELECT tf.num, tt.num, tl.type
	      FROM ticket_links tl
	      JOIN tickets tf ON tl.from_id = tf.id
	      JOIN tickets tt ON tl.to_id   = tt.id
	      WHERE tf.project_id = ? AND tt.project_id = ?`
	args := []any{p.ID, p.ID}

	if ticketNum > 0 {
		q += ` AND (tf.num = ? OR tt.num = ?)`
		args = append(args, ticketNum, ticketNum)
	}
	if linkType != "" {
		q += ` AND tl.type = ?`
		args = append(args, linkType)
	}
	if !includeClosed {
		q += ` AND tf.status != 'closed' AND tt.status != 'closed'`
	}
	q += ` ORDER BY tf.num, tt.num, tl.type`

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, internalErr(err)
	}
	defer rows.Close()
	var out []projectLink
	for rows.Next() {
		var l projectLink
		if err := rows.Scan(&l.From, &l.To, &l.Type); err != nil {
			return nil, internalErr(err)
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, internalErr(err)
	}
	return out, nil
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
