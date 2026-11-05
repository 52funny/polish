package app

import (
	"context"
	"slices"
	"strings"

	"github.com/52funny/polish/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func (a *app) stopModelList() {
	if a.modelsCancel != nil {
		a.modelsCancel()
		a.modelsCancel = nil
	}
	a.modelsGeneration++
	a.modelsLoading = false
}

func (a *app) connectionChanged() {
	a.stopModelList()
	a.modelIDs = nil
	a.modelsNotice, a.modelsFailed = i18n.Message{}, false
	a.settingsError = i18n.Message{}
}

func (a *app) fetchModels() {
	a.stopModelList()
	a.modelIDs = nil
	cfg := a.draft.Normalized()
	if err := cfg.ValidateConnection(); err != nil {
		a.modelsNotice, a.modelsFailed = i18n.FromError(err), true
		return
	}
	id := a.modelsGeneration
	ctx, cancel := context.WithCancel(context.Background())
	a.modelsCancel, a.modelsLoading = cancel, true
	a.modelsNotice, a.modelsFailed = i18n.Text("Fetching model list…"), false
	go func() {
		ids, err := a.modelLister.ListModels(ctx, cfg)
		a.post(func() {
			current := a.draft.Normalized()
			if id != a.modelsGeneration || cfg.BaseURL != current.BaseURL || cfg.APIKey != current.APIKey {
				return
			}
			cancel()
			a.modelsLoading, a.modelsCancel = false, nil
			if err != nil {
				a.modelsNotice, a.modelsFailed = i18n.FromError(err), true
				return
			}
			a.modelIDs, a.manualModel = ids, false
			if strings.TrimSpace(a.draft.Model) == "" && len(ids) > 0 {
				a.draft.Model = ids[0]
			}
			a.modelsNotice = i18n.Text("Found %d candidate models. Access and quota are checked when used.", len(ids))
		})
	}()
}

func (a *app) modelPicker(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
		ui.Text(c, a.tr("Model name")).Bold().Grow(1)
		if len(a.modelIDs) > 0 {
			label := "Enter manually"
			if a.manualModel {
				label = "Choose from list"
			}
			if ui.Button(c.Key("model-manual"), a.tr(label)).Clicked() {
				a.manualModel = !a.manualModel
			}
		}
		label := "Fetch models"
		if a.modelsLoading {
			label = "Fetching…"
		} else if len(a.modelIDs) > 0 {
			label = "Refresh list"
		}
		if ui.Button(c.Key("fetch-models"), a.tr(label)).Disabled(a.modelsLoading || a.draft.ValidateConnection() != nil).Clicked() {
			a.fetchModels()
		}
	})
	if len(a.modelIDs) > 0 && !a.manualModel {
		ui.Select(c.Key("model-select"), &a.draft.Model, a.modelIDs).FillWidth().Label(a.tr("Choose a model"))
	} else {
		ui.TextInput(c.Key("model"), &a.draft.Model).FillWidth().Placeholder(a.tr("Fetch models or enter a model ID")).Label(a.tr("Model name"))
	}
	notice := a.modelsNotice
	if notice.Source == "" {
		notice = i18n.Text("Fetch models to choose from a list, or enter an ID manually.")
	}
	if len(a.modelIDs) > 0 && !a.manualModel && !slices.Contains(a.modelIDs, a.draft.Model) {
		notice = i18n.Text("The current model is not listed. Choose another or enter it manually.")
	}
	color := t.TextMuted
	if a.modelsFailed {
		color = t.Danger
	}
	ui.Text(c, notice.String(a.language())).FillWidth().FontSize(12).TextColor(color)
}
