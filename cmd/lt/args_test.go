package main

import (
	"flag"
	"reflect"
	"testing"
)

func TestReorderArgs(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			"flags before positionals stay put",
			[]string{"-p", "demo", "--body", "hi", "title here"},
			[]string{"-p", "demo", "--body", "hi", "title here"},
		},
		{
			"flag after positional moves before",
			[]string{"-p", "demo", "title", "--body", "hi"},
			[]string{"-p", "demo", "--body", "hi", "title"},
		},
		{
			"--flag=value form",
			[]string{"title", "--body=hi"},
			[]string{"--body=hi", "title"},
		},
		{
			"-- ends flag scanning",
			[]string{"--body", "hi", "--", "--not-a-flag"},
			[]string{"--body", "hi", "--not-a-flag"},
		},
		{
			"bare - is positional",
			[]string{"title", "--body", "-"},
			[]string{"--body", "-", "title"},
		},
		{
			"bool flag with no value",
			[]string{"name", "--force"},
			[]string{"--force", "name"},
		},
		{
			"triple-dash treated as positional",
			[]string{"---body", "stuff"},
			[]string{"---body", "stuff"},
		},
		{
			"numeric-leading-dash is positional",
			[]string{"--body", "x", "-5"},
			[]string{"--body", "x", "-5"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fs := flag.NewFlagSet("t", flag.ContinueOnError)
			var p, body string
			var force bool
			fs.StringVar(&p, "p", "", "")
			fs.StringVar(&body, "body", "", "")
			fs.BoolVar(&force, "force", false, "")
			got := reorderArgs(fs, c.in)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
