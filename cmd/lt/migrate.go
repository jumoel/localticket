// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"database/sql"
	"fmt"
)

// migrations[v] brings the DB from version v-1 to v. Index 0 is unused (no v0).
// Index 1 is also unused since v1 is the initial schema applied by schemaSQL on
// a fresh DB, not a migration step.
//
// To add a migration: append a function at the new version's index, then bump
// currentSchemaVersion in store.go.
var migrations = []func(*sql.Tx) error{
	nil, // index 0, unused
	nil, // index 1, v1 = initial schemaSQL
}

// runMigrations advances the DB from `from` to `to`, applying each step in
// `migs` inside its own transaction. A failure rolls back that step and
// surfaces the error; earlier steps stay committed. Tests pass a synthetic
// slice; production code passes the package-level migrations.
func runMigrations(db *sql.DB, from, to int, migs []func(*sql.Tx) error) error {
	for v := from + 1; v <= to; v++ {
		if v >= len(migs) || migs[v] == nil {
			return fmt.Errorf("no migration registered for v%d", v)
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration to v%d: %w", v, err)
		}
		if err := migs[v](tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration to v%d: %w", v, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_version(version) VALUES (?)`, v); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration to v%d: %w", v, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration to v%d: %w", v, err)
		}
	}
	return nil
}
