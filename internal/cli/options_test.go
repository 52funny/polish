package cli

import (
	"errors"
	"flag"
	"testing"

	"github.com/52funny/polish/internal/llm"
)

func TestParseLaunchOptions(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want Options
	}{
		{"default", nil, Options{}},
		{"polish", []string{"polish"}, Options{Mode: llm.Polish}},
		{"translate", []string{"translate"}, Options{Mode: llm.Translate}},
		{"legacy selection", []string{"--selection"}, Options{Selection: true}},
		{"legacy single dash", []string{"-selection=true"}, Options{Selection: true}},
		{"polish selection", []string{"polish", "--selection"}, Options{Mode: llm.Polish, Selection: true}},
		{"translate selection", []string{"translate", "--selection"}, Options{Mode: llm.Translate, Selection: true}},
		{"selection before command", []string{"--selection", "translate"}, Options{Mode: llm.Translate, Selection: true}},
		{"explicit true", []string{"translate", "--selection=true"}, Options{Mode: llm.Translate, Selection: true}},
		{"single dash", []string{"translate", "-selection"}, Options{Mode: llm.Translate, Selection: true}},
		{"false", []string{"translate", "--selection=false"}, Options{Mode: llm.Translate}},
		{"last flag wins", []string{"--selection", "translate", "--selection=false"}, Options{Mode: llm.Translate}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.args)
			if err != nil || got != tc.want {
				t.Fatalf("Parse(%q) = %+v, %v; want %+v", tc.args, got, err, tc.want)
			}
		})
	}
}

func TestParseHelpAndErrors(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}, {"polish", "-h"}, {"translate", "--help"}} {
		if _, err := Parse(args); !errors.Is(err, flag.ErrHelp) {
			t.Errorf("Parse(%q) = %v; want help", args, err)
		}
	}
	for _, args := range [][]string{
		{"unknown"}, {"polish", "translate"}, {"translate", "extra"},
		{"--unknown"}, {"translate", "--unknown"}, {"--selection=invalid"},
		{"translate", "--selection=invalid"}, {"--selection", "false"},
	} {
		if _, err := Parse(args); err == nil || errors.Is(err, flag.ErrHelp) {
			t.Errorf("Parse(%q) = %v; want argument error", args, err)
		}
	}
}
