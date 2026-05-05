// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"io"
)

// idList collects repeatable --id flags.
type idList []int64

func (l *idList) String() string { return "" }
func (l *idList) Set(v string) error {
	id, err := parseTicketID(v)
	if err != nil {
		return err
	}
	*l = append(*l, id)
	return nil
}

// bulkError mirrors cmdError but carries the offending ticket id so multi-ID
// callers can attribute partial failures.
type bulkError struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

// applyBulk runs op against each id. With a single id it preserves the legacy
// bare-ticket return shape; with more it emits a wrapper containing successes
// and per-ticket failures, then returns a cmdError with the first failure's
// exit code so the parent process gets a meaningful status.
//
// op is called with a fresh store handle for each invocation so callers don't
// have to thread one through. verb is used for the TTY summary line.
func applyBulk(ids []int64, verb string, op func(s *store, id int64) (*ticket, error), stdout io.Writer, mode outMode) error {
	s, err := openDefaultStore()
	if err != nil {
		return err
	}
	defer s.Close()

	if len(ids) == 1 {
		t, err := op(s, ids[0])
		if err != nil {
			return err
		}
		return renderTicket(stdout, mode, t, verb)
	}

	var results []*ticket
	var failures []bulkError
	var firstErr *cmdError
	for _, id := range ids {
		t, err := op(s, id)
		if err == nil {
			results = append(results, t)
			continue
		}
		ce := asCmdErrorOrInternal(err)
		failures = append(failures, bulkError{ID: id, Code: ce.code, Msg: ce.msg})
		if firstErr == nil {
			firstErr = ce
		}
	}

	if mode == modeJSON {
		if results == nil {
			results = []*ticket{}
		}
		if failures == nil {
			failures = []bulkError{}
		}
		if err := writeJSON(stdout, map[string]any{
			"tickets": results,
			"errors":  failures,
		}); err != nil {
			return err
		}
	} else {
		for _, t := range results {
			fmt.Fprintf(stdout, "%s %s#%d\n", verb, t.Project, t.ID)
		}
		for _, be := range failures {
			fmt.Fprintf(stdout, "FAILED #%d: %s (%s)\n", be.ID, be.Msg, be.Code)
		}
	}

	if firstErr != nil {
		return &cmdError{
			code:     "bulk",
			exitCode: firstErr.exitCode,
			msg:      fmt.Sprintf("%d of %d operations failed", len(failures), len(ids)),
		}
	}
	return nil
}

func asCmdErrorOrInternal(err error) *cmdError {
	var ce *cmdError
	if errors.As(err, &ce) {
		return ce
	}
	return &cmdError{code: "internal", exitCode: 4, msg: err.Error()}
}
