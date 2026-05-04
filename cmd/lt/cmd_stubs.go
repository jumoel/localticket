package main

import "io"

func runProject(args []string, stdout, stderr io.Writer, mode outMode) error {
	return runProjectImpl(args, stdout, stderr, mode)
}
func runNew(args []string, stdout io.Writer, mode outMode) error {
	return userErr("not_implemented", "new not implemented yet")
}
func runList(args []string, stdout io.Writer, mode outMode) error {
	return userErr("not_implemented", "list not implemented yet")
}
func runShow(args []string, stdout io.Writer, mode outMode) error {
	return userErr("not_implemented", "show not implemented yet")
}
func runEdit(args []string, stdout io.Writer, mode outMode) error {
	return userErr("not_implemented", "edit not implemented yet")
}
func runStatus(args []string, stdout io.Writer, mode outMode) error {
	return userErr("not_implemented", "status not implemented yet")
}
func runClose(args []string, stdout io.Writer, mode outMode) error {
	return userErr("not_implemented", "close not implemented yet")
}
func runReopen(args []string, stdout io.Writer, mode outMode) error {
	return userErr("not_implemented", "reopen not implemented yet")
}
func runLabel(args []string, stdout io.Writer, mode outMode) error {
	return userErr("not_implemented", "label not implemented yet")
}
func runLink(args []string, stdout io.Writer, mode outMode) error {
	return userErr("not_implemented", "link not implemented yet")
}
func runSearch(args []string, stdout io.Writer, mode outMode) error {
	return userErr("not_implemented", "search not implemented yet")
}
