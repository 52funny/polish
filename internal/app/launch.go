package app

import (
	"github.com/52funny/polish/internal/cli"
	"github.com/52funny/polish/internal/llm"
)

// launch applies both initial and forwarded launch requests on the UI thread.
func (a *app) launch(options cli.Options) {
	if options.Mode != "" {
		mode := 0
		if options.Mode == llm.Translate {
			mode = 1
		}
		if a.mode != mode {
			a.mode = mode
			a.changed()
		}
		a.stopModelList()
		a.settings = a.cfg.Validate() != nil
	}
	if options.Selection {
		a.readText(true, options.Mode != "")
	} else {
		a.reveal()
	}
}
