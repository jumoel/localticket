package main

import (
	"fmt"
	"regexp"
)

var nameRE = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

func validateProjectName(s string) error {
	if !nameRE.MatchString(s) {
		return userErr("invalid_name", fmt.Sprintf("project name %q must match [a-z0-9_-], 1-64 chars", s))
	}
	return nil
}

func validateLabel(s string) error {
	if !nameRE.MatchString(s) {
		return userErr("invalid_label", fmt.Sprintf("label %q must match [a-z0-9_-], 1-64 chars", s))
	}
	return nil
}
