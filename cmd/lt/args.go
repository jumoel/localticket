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
		if len(a) > 1 && a[0] == '-' && a != "-" {
			name := strings.TrimLeft(a, "-")
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
