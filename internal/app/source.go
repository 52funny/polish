package app

import (
	"strings"

	"github.com/52funny/polish/internal/config"
	"github.com/52funny/polish/internal/i18n"
)

// Treat CRLF as one line break. Leave other spaces and tabs untouched.
var newlineSpaces = strings.NewReplacer(
	"\r\n", " ", "\r", " ", "\n", " ",
	"\u0085", " ", "\u2028", " ", "\u2029", " ",
)

func (a *app) prepareSource(text string) string {
	if a.cfg.ReplaceNewlines {
		return newlineSpaces.Replace(text)
	}
	return text
}

func (a *app) setReplaceNewlines(enabled bool) {
	if err := config.SaveReplaceNewlines(a.configPath, enabled); err != nil {
		a.message(i18n.Text("Could not save the line-break preference: %s", i18n.FromError(err)), true)
		return
	}
	a.cfg.ReplaceNewlines, a.draft.ReplaceNewlines = enabled, enabled
	a.changed()
}
