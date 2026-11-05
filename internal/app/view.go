package app

import (
	"strings"
	"unicode/utf8"

	"github.com/52funny/polish/internal/config"
	"github.com/52funny/polish/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func (a *app) view(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(24).Gap(16).Background(t.Background).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Column(c).Grow(1).Gap(4).Children(func() {
				ui.Text(c, "Polish").FontSize(28).Bold()
				ui.Text(c, a.tr("Clearer writing, across languages.")).FontSize(13).TextColor(t.TextMuted)
			})
			a.languagePicker(c)
			label := "Model settings"
			if a.settings {
				label = "Back to editor"
			}
			if ui.Button(c.Key("settings-toggle"), a.tr(label)).Clicked() {
				a.stopModelList()
				a.settings = !a.settings
				if a.settings {
					a.draft = a.cfg
					a.connectionChanged()
				}
			}
		})
		ui.Divider(c)
		if a.settings {
			a.settingsView(c)
			return
		}
		locked := a.busy == "read" || a.busy == "copy"
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
			if ui.Segmented(c.Key("mode"), &a.mode, a.tr("Polish text"), a.tr("Translate")).Disabled(locked).Changed() {
				a.changed()
			}
			if a.mode == 0 {
				ui.Text(c, a.tr("Style")).TextColor(t.TextMuted)
				if a.choice(c, "style", &a.style, []string{"natural", "formal", "concise"}, []string{"Natural", "Formal", "Concise"}, 110, locked).Changed() {
					a.changed()
				}
			} else {
				ui.Text(c, a.tr("Translate to")).TextColor(t.TextMuted)
				if a.choice(c, "target", &a.target, []string{"zh-Hans", "en"}, []string{"Simplified Chinese", "English"}, 180, locked).Changed() {
					a.changed()
				}
			}
		})
		ui.Row(c).FillWidth().Grow(1).Gap(16).Children(func() {
			ui.Column(c).Grow(1).Basis(0).FillHeight().Gap(10).Children(func() {
				ui.Row(c).FillWidth().Height(24).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, a.tr("Source")).Bold().Grow(1)
					replaceNewlines := a.cfg.ReplaceNewlines
					if ui.Checkbox(c.Key("replace-newlines"), &replaceNewlines, a.tr("Replace line breaks")).FontSize(12).Disabled(locked).Changed() {
						a.setReplaceNewlines(replaceNewlines)
					}
					ui.Text(c, a.tr("%d characters", utf8.RuneCountInString(a.source))).FontSize(12).TextColor(t.TextMuted)
				})
				if ui.TextArea(c.Key("source"), &a.source).FillWidth().Grow(1).MinHeight(120).
					Placeholder(a.tr("Select text in another app, or type / paste here…")).Label(a.tr("Source")).FontSize(16).ReadOnly(locked).Changed() {
					a.changed()
				}
			})
			ui.Column(c).Grow(1).Basis(0).FillHeight().Gap(10).Children(func() {
				ui.Row(c).Height(24).AlignItems(ui.Center).Children(func() { ui.Text(c, a.tr("Result")).Bold() })
				ui.TextArea(c.Key("result"), &a.result).FillWidth().Grow(1).MinHeight(120).
					Placeholder(a.tr("Polished or translated text will appear here.")).Label(a.tr("Result")).FontSize(16).ReadOnly(true)
			})
		})
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			if ui.Button(c.Key("paste"), a.tr("Read clipboard")).Disabled(locked).Clicked() {
				a.read(false)
			}
			ui.Spacer(c).Grow(1)
			if a.busy != "" {
				ui.Spinner(c)
				if ui.Button(c.Key("cancel"), a.tr("Cancel")).Clicked() {
					a.stop()
					a.message(i18n.Text("Canceled."), false)
				}
			}
			label := "Polish"
			if a.mode == 1 {
				label = "Translate"
			}
			if ui.PrimaryButton(c.Key("process"), a.tr(label)).Disabled(locked || a.busy == "process" || strings.TrimSpace(a.source) == "").Clicked() {
				a.process()
			}
			if ui.Button(c.Key("copy"), a.tr("Copy result")).Disabled(a.busy != "" || a.result == "").Clicked() {
				a.copy()
			}
		})
		color := t.TextMuted
		if a.failed {
			color = t.Danger
		}
		ui.Text(c, a.status.String(a.language())).FillWidth().MinHeight(38).FontSize(13).TextColor(color)
	})
}

func (a *app) settingsView(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c).FillWidth().Grow(1).Children(func() {
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			ui.Text(c, a.tr("Connect a model")).FontSize(20).Bold()
			ui.Text(c, a.tr("Enter your endpoint and API key, then fetch models.")).TextColor(t.TextMuted)
			ui.Text(c, a.tr("Base URL")).Bold()
			if ui.TextInput(c.Key("base-url"), &a.draft.BaseURL).FillWidth().Placeholder(a.tr("https://api.example.com/v1")).Label(a.tr("Base URL")).Changed() {
				a.connectionChanged()
			}
			ui.Text(c, a.tr("API Key")).Bold()
			if ui.TextInput(c.Key("api-key"), &a.draft.APIKey).FillWidth().Password().Placeholder(a.tr("Enter your API key")).Label(a.tr("API Key")).Changed() {
				a.connectionChanged()
			}
			a.modelPicker(c)
			ui.Text(c, a.tr("Your key is stored locally in plain text with user-only permissions. Text is sent only when you start processing. No history is saved.")).FillWidth().FontSize(13).TextColor(t.TextMuted)
			if a.settingsError.Source != "" {
				ui.Text(c, a.settingsError.String(a.language())).FillWidth().TextColor(t.Danger)
			}
			if ui.PrimaryButton(c.Key("save-settings"), a.tr("Save settings")).Clicked() {
				if err := config.Save(a.configPath, a.draft); err != nil {
					a.settingsError = i18n.FromError(err)
				} else {
					a.stopModelList()
					a.stop()
					a.cfg = a.draft.Normalized()
					a.settingsError, a.settings = i18n.Message{}, false
					a.message(i18n.Text("Settings saved. You can now process text."), false)
				}
			}
		})
	})
}
