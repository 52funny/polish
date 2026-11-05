package app

import (
	"github.com/52funny/polish/internal/config"
	"github.com/52funny/polish/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func (a *app) language() string {
	if a.cfg.Language == "" {
		return a.systemLanguage
	}
	return i18n.Resolve(a.cfg.Language)
}

func (a *app) tr(source string, args ...any) string { return i18n.T(a.language(), source, args...) }

func (a *app) setLanguage(language string) {
	if err := config.SaveLanguage(a.configPath, language); err != nil {
		a.message(i18n.Text("Could not save the language preference: %s", i18n.FromError(err)), true)
		return
	}
	a.cfg.Language, a.draft.Language = language, language
	a.languageChanged(a.language())
}

func (a *app) languagePicker(c *ui.Context) {
	preference := a.cfg.Language
	if a.choice(c, "language", &preference,
		[]string{"", "en", "zh-Hans"}, []string{"System default", "English (language name)", "Simplified Chinese (language name)"}, 155, false).Changed() {
		a.setLanguage(preference)
	}
}

// Keep stored option IDs independent from their translated display labels.
func (a *app) choice(c *ui.Context, key string, selected *string, ids, sources []string, width float32, disabled bool) ui.Element {
	labels := make([]string, len(ids))
	value := ""
	for i, id := range ids {
		labels[i] = a.tr(sources[i])
		if id == *selected {
			value = labels[i]
		}
	}
	label := map[string]string{"language": "Interface language", "style": "Style", "target": "Translate to"}[key]
	element := ui.Select(c.Key(key), &value, labels).Width(width).Disabled(disabled).Label(a.tr(label))
	if element.Changed() {
		for i, label := range labels {
			if value == label {
				*selected = ids[i]
				break
			}
		}
	}
	return element
}
