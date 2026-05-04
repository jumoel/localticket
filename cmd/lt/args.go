package main

import (
	"flag"
	"strings"
)

// reorderArgs moves all known flags (with their values) ahead of positional
// arguments so that Go's flag package, which stops at the first positional,
// still sees them. Recognised flags come from fs. `--` ends flag scanning.
// Unknown flag-looking tokens are left in place (parser will reject).
func reorderArgs(fs *flag.FlagSet, args []string) []string {
	takesValue := map[string]bool{}
	fs.VisitAll(func(f *flag.Flag) {
		if bf, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && bf.IsBoolFlag() {
			takesValue[f.Name] = false
			return
		}
		takesValue[f.Name] = true
	})

	var flags, positionals []string
	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if name, ok := flagName(a); ok {
			hasValue := false
			if eq := strings.Index(name, "="); eq >= 0 {
				name = name[:eq]
				hasValue = true
			}
			needs, known := takesValue[name]
			if !known {
				flags = append(flags, a)
				i++
				continue
			}
			if needs && !hasValue {
				if i+1 >= len(args) {
					flags = append(flags, a)
					i++
					continue
				}
				flags = append(flags, a, args[i+1])
				i += 2
				continue
			}
			flags = append(flags, a)
			i++
			continue
		}
		positionals = append(positionals, a)
		i++
	}
	return append(flags, positionals...)
}

func parseArgs(fs *flag.FlagSet, args []string) error {
	return fs.Parse(reorderArgs(fs, args))
}

// flagName returns the bare flag name from a token like "--foo", "-p", or
// "--foo=bar", and reports whether the token looks like a flag at all. Tokens
// that are bare "-", "--", or numeric-looking ("-5", "-1.2") are NOT flags.
// Exactly one or two leading dashes are stripped; "---foo" is treated as a
// positional, since neither the flag package nor users mean anything by it.
func flagName(a string) (string, bool) {
	if len(a) < 2 || a[0] != '-' || a == "--" {
		return "", false
	}
	rest := a[1:]
	if rest[0] == '-' {
		if len(rest) < 2 || rest[1] == '-' {
			return "", false
		}
		return rest[1:], true
	}
	if rest[0] >= '0' && rest[0] <= '9' {
		return "", false
	}
	return rest, true
}
