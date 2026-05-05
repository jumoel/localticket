// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"regexp"
	"strings"
)

// atxHeadingRE matches a markdown ATX heading line: one or more `#`, at least
// one space, the title text, optional trailing `#`s and whitespace.
var atxHeadingRE = regexp.MustCompile(`(?m)^(#+)[ \t]+(.+?)[ \t]*#*[ \t]*$`)

type heading struct {
	level     int
	title     string
	lineStart int // byte offset of the line's first char
	lineEnd   int // byte offset just past the trailing newline (or len(body) if last line has no newline)
}

func parseHeadings(body string) []heading {
	locs := atxHeadingRE.FindAllStringSubmatchIndex(body, -1)
	out := make([]heading, 0, len(locs))
	for _, loc := range locs {
		level := loc[3] - loc[2]
		title := body[loc[4]:loc[5]]
		lineStart := loc[0]
		lineEnd := loc[1]
		if lineEnd < len(body) && body[lineEnd] == '\n' {
			lineEnd++
		}
		out = append(out, heading{
			level:     level,
			title:     title,
			lineStart: lineStart,
			lineEnd:   lineEnd,
		})
	}
	return out
}

func normalizeHeading(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimLeft(s, "#")
	return strings.TrimSpace(strings.ToLower(s))
}

type sectionMatch struct {
	heading    heading
	contentEnd int // byte offset where the section's content ends (next same-or-higher heading, or end of body)
}

// findSection locates a unique section by heading text. Returns:
//   - section_not_found if no heading matches.
//   - section_ambiguous if more than one matches.
func findSection(body, target string) (*sectionMatch, error) {
	wantNorm := normalizeHeading(target)
	if wantNorm == "" {
		return nil, userErr("bad_section", "empty section heading")
	}

	headings := parseHeadings(body)
	var matches []int
	for i, h := range headings {
		if normalizeHeading(h.title) == wantNorm {
			matches = append(matches, i)
		}
	}
	if len(matches) == 0 {
		return nil, &cmdError{
			code:     "section_not_found",
			exitCode: 2,
			msg:      fmt.Sprintf("section %q not found", target),
		}
	}
	if len(matches) > 1 {
		return nil, &cmdError{
			code:     "section_ambiguous",
			exitCode: 1,
			msg:      fmt.Sprintf("section %q matches %d headings", target, len(matches)),
		}
	}

	idx := matches[0]
	h := headings[idx]
	contentEnd := len(body)
	for i := idx + 1; i < len(headings); i++ {
		if headings[i].level <= h.level {
			contentEnd = headings[i].lineStart
			break
		}
	}
	return &sectionMatch{heading: h, contentEnd: contentEnd}, nil
}

// sectionContent returns the body content between the heading line and the
// next same-or-higher heading.
func sectionContent(body string, m *sectionMatch) string {
	return body[m.heading.lineEnd:m.contentEnd]
}

// replaceSection returns a new body with the matched section's content swapped
// for newContent. The heading line is preserved unchanged. If newContent is
// non-empty and does not end with a newline, one is appended so the next
// heading stays on its own line.
func replaceSection(body string, m *sectionMatch, newContent string) string {
	if newContent != "" && !strings.HasSuffix(newContent, "\n") && m.contentEnd < len(body) {
		newContent += "\n"
	}
	return body[:m.heading.lineEnd] + newContent + body[m.contentEnd:]
}
