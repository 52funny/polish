// Package cli parses launch arguments shared by the entry point and instance forwarding.
package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/52funny/polish/internal/llm"
)

const Usage = `Usage: polish [--selection] [polish|translate] [--selection]

Commands:
  polish       Open the polishing editor
  translate    Open the translation editor

Options:
  --selection  Read the Wayland PRIMARY selection before focusing the window.
               With a subcommand, automatically polish or translate that text.
  -h, --help   Show this help

Without a subcommand, open the editor without automatically processing text.
Repeated launches reuse the existing window and its current mode unless specified.
`

type Options struct {
	// An empty mode leaves an existing window's mode unchanged.
	Mode      llm.Operation
	Selection bool
}

// Parse accepts options before or after a single optional subcommand.
// Help returns flag.ErrHelp; parsing never exits or writes output.
func Parse(args []string) (Options, error) {
	var options Options
	flags := flag.NewFlagSet("polish", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&options.Selection, "selection", false, "Read the Wayland PRIMARY selection")
	if err := flags.Parse(args); err != nil {
		return Options{}, err
	}
	if rest := flags.Args(); len(rest) > 0 {
		switch rest[0] {
		case string(llm.Polish):
			options.Mode = llm.Polish
		case string(llm.Translate):
			options.Mode = llm.Translate
		default:
			return Options{}, fmt.Errorf("unknown command %q (expected polish or translate)", rest[0])
		}
		if err := flags.Parse(rest[1:]); err != nil {
			return Options{}, err
		}
		if rest := flags.Args(); len(rest) > 0 {
			return Options{}, fmt.Errorf("unexpected argument %q", rest[0])
		}
	}
	return options, nil
}
