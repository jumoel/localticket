package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type outMode int

const (
	modeAuto outMode = iota
	modeJSON
	modePretty
)

func outputMode(s string) outMode {
	switch s {
	case "json":
		return modeJSON
	case "pretty":
		return modePretty
	default:
		return modeAuto
	}
}

func detectMode(w io.Writer) outMode {
	f, ok := w.(*os.File)
	if !ok {
		return modeJSON
	}
	fi, err := f.Stat()
	if err != nil {
		return modeJSON
	}
	if (fi.Mode() & os.ModeCharDevice) != 0 {
		return modePretty
	}
	return modeJSON
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func writeError(w io.Writer, mode outMode, code, msg string) {
	if mode == modeJSON {
		_ = writeJSON(w, map[string]string{"error": msg, "code": code})
		return
	}
	fmt.Fprintf(w, "error: %s\n", msg)
}
