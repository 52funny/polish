package llm

import (
	"embed"
	"strings"
	"text/template"

	"github.com/52funny/polish/internal/i18n"
)

//go:embed prompts/*.md
var promptFiles embed.FS

var promptTemplates = template.Must(template.New("prompts").Option("missingkey=error").ParseFS(promptFiles, "prompts/*.md"))

func prompt(r Request) (string, error) {
	var name string
	switch r.Operation {
	case Polish:
		name = map[string]string{
			"natural": "polish-natural.md",
			"formal":  "polish-formal.md",
			"concise": "polish-concise.md",
		}[r.Style]
		if name == "" {
			return "", i18n.Text("Choose a valid polishing style.")
		}
	case Translate:
		if r.Target != "zh-Hans" && r.Target != "en" {
			return "", i18n.Text("Choose a valid target language.")
		}
		name = "translate.md"
		r.Target = map[string]string{"zh-Hans": "Simplified Chinese", "en": "English"}[r.Target]
	default:
		return "", i18n.Text("Unknown processing operation.")
	}
	var common, task strings.Builder
	if err := promptTemplates.ExecuteTemplate(&common, "system.md", nil); err != nil {
		return "", i18n.Text("Could not render the shared prompt. Check its template.")
	}
	if err := promptTemplates.ExecuteTemplate(&task, name, r); err != nil {
		return "", i18n.Text("Could not render the task prompt. Check its template.")
	}
	return strings.TrimSpace(common.String()) + "\n\n" + strings.TrimSpace(task.String()), nil
}
