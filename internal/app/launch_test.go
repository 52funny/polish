package app

import (
	"context"
	"errors"
	"testing"

	"github.com/52funny/polish/internal/cli"
	"github.com/52funny/polish/internal/config"
	"github.com/52funny/polish/internal/llm"
)

func TestSelectionSubcommandsProcessAutomatically(t *testing.T) {
	for _, operation := range []llm.Operation{llm.Polish, llm.Translate} {
		t.Run(string(operation), func(t *testing.T) {
			a, q := configuredApp(t)
			a.cfg.ReplaceNewlines = true
			a.style, a.target = "formal", "en"
			a.source, a.result = "old source", "old result"
			a.clipboard = clipFunc(func(_ context.Context, primary bool) (string, error) {
				if !primary {
					t.Error("launch read the regular clipboard instead of PRIMARY")
				}
				return "selected\r\ntext", nil
			})
			requests := make(chan llm.Request, 2)
			a.model = modelFunc(func(_ context.Context, _ config.Config, r llm.Request) (string, error) {
				requests <- r
				return "new result", nil
			})
			reveals := 0
			a.reveal = func() {
				reveals++
				if a.source != "selected text" || a.busy != "process" {
					t.Error("window did not show the selected text and processing state")
				}
			}
			a.launch(cli.Options{Mode: operation, Selection: true})
			if reveals != 0 || a.source != "" || a.result != "" {
				t.Fatal("focused before capture or kept stale text")
			}
			deliver(t, q) // Selection read starts processing without a click.
			deliver(t, q)
			if a.result != "new result" || reveals != 1 || len(requests) != 1 {
				t.Fatal("automatic processing did not finish exactly once")
			}
			r := <-requests
			if r.Operation != operation || r.Text != "selected text" || r.Style != "formal" || r.Target != "en" {
				t.Fatalf("wrong automatic request: %+v", r)
			}
		})
	}
}

func TestLaunchModesAndManualCompatibility(t *testing.T) {
	a, q := configuredApp(t)
	a.source, a.result = "source", "result"
	a.model = modelFunc(func(context.Context, config.Config, llm.Request) (string, error) {
		t.Error("manual launch unexpectedly called the model")
		return "unexpected", nil
	})
	a.launch(cli.Options{Mode: llm.Polish})
	if a.source != "source" || a.result != "result" {
		t.Fatal("repeating current mode discarded text")
	}
	a.launch(cli.Options{Mode: llm.Translate})
	if a.mode != 1 || a.source != "source" || a.result != "" || a.busy != "" {
		t.Fatal("explicit mode did not switch without processing")
	}
	a.result = "translation"
	a.launch(cli.Options{})
	if a.mode != 1 || a.result != "translation" {
		t.Fatal("plain launch changed existing mode or result")
	}
	a.clipboard = clipFunc(func(context.Context, bool) (string, error) { return "new selection", nil })
	a.launch(cli.Options{Selection: true})
	deliver(t, q)
	if a.mode != 1 || a.source != "new selection" || a.result != "" || a.busy != "" {
		t.Fatal("legacy selection launch changed mode or processed automatically")
	}
	a.launch(cli.Options{Mode: llm.Polish})
	if a.mode != 0 || a.source != "new selection" || a.busy != "" {
		t.Fatal("could not switch back to polishing")
	}
}

func TestAutomaticProcessingStopsOnCaptureOrConfigurationFailure(t *testing.T) {
	for _, tc := range []struct {
		name         string
		text         string
		readErr      error
		unconfigured bool
	}{
		{name: "capture failure", readErr: errors.New("no selection")},
		{name: "capture timeout", readErr: context.DeadlineExceeded},
		{name: "empty selection"},
		{name: "whitespace selection", text: " \n "},
		{name: "missing configuration", text: "keep this source", unconfigured: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, q := configuredApp(t)
			if tc.unconfigured {
				a.cfg, a.draft = config.Config{}, config.Config{}
			}
			a.clipboard = clipFunc(func(context.Context, bool) (string, error) { return tc.text, tc.readErr })
			a.model = modelFunc(func(context.Context, config.Config, llm.Request) (string, error) {
				t.Error("model called despite capture or configuration failure")
				return "unexpected", nil
			})
			revealed := false
			a.reveal = func() { revealed = true }
			a.launch(cli.Options{Mode: llm.Translate, Selection: true})
			deliver(t, q)
			if !revealed || a.busy != "" || a.result != "" || a.source != tc.text {
				t.Fatal("failure left a task running, stale text, or hidden window")
			}
			if tc.unconfigured {
				if !a.settings || a.settingsError.Source == "" || a.mode != 1 {
					t.Fatal("missing configuration did not open settings in the requested mode")
				}
			} else if !a.failed {
				t.Fatal("capture failure was not surfaced")
			}
		})
	}
}

func TestLaunchLeavesSettingsWithoutSavingDraft(t *testing.T) {
	a, _ := configuredApp(t)
	a.settings = true
	a.draft.Model = "unsaved model"
	ctx, cancel := context.WithCancel(context.Background())
	a.modelsCancel, a.modelsLoading = cancel, true
	a.launch(cli.Options{Mode: llm.Translate})
	if a.settings || a.modelsLoading || ctx.Err() == nil || a.draft.Model != "unsaved model" || a.cfg.Model != "test-model" {
		t.Fatal("launch did not leave settings safely")
	}
	a.cfg = config.Config{}
	a.launch(cli.Options{Mode: llm.Polish})
	if !a.settings {
		t.Fatal("unconfigured launch bypassed settings")
	}
}

func TestNewLaunchDiscardsLateAutomaticRead(t *testing.T) {
	a, q := configuredApp(t)
	started := make(chan context.Context, 1)
	lateRead := make(chan struct{})
	a.clipboard = clipFunc(func(ctx context.Context, _ bool) (string, error) {
		started <- ctx
		<-lateRead // Deliberately ignore cancellation.
		return "stale selection", nil
	})
	a.model = modelFunc(func(context.Context, config.Config, llm.Request) (string, error) {
		t.Error("stale selection started a model request")
		return "unexpected", nil
	})
	a.launch(cli.Options{Mode: llm.Translate, Selection: true})
	ctx := <-started
	a.launch(cli.Options{Mode: llm.Polish})
	if ctx.Err() == nil {
		t.Fatal("mode switch did not cancel the pending capture")
	}
	a.source, a.result = "current source", "current result"
	close(lateRead)
	deliver(t, q)
	if a.source != "current source" || a.result != "current result" || a.busy != "" {
		t.Fatal("late capture overwrote the new launch")
	}
}

func TestNewLaunchDiscardsLateAutomaticResponse(t *testing.T) {
	a, q := configuredApp(t)
	a.clipboard = clipFunc(func(context.Context, bool) (string, error) { return "selected text", nil })
	started := make(chan context.Context, 1)
	lateResponse := make(chan struct{})
	a.model = modelFunc(func(ctx context.Context, _ config.Config, r llm.Request) (string, error) {
		if r.Operation == llm.Translate {
			started <- ctx
			<-lateResponse // Deliberately ignore cancellation.
			return "stale translation", nil
		}
		return "new polish", nil
	})
	a.launch(cli.Options{Mode: llm.Translate, Selection: true})
	deliver(t, q)
	ctx := <-started
	a.launch(cli.Options{Mode: llm.Polish, Selection: true})
	if ctx.Err() == nil {
		t.Fatal("new launch did not cancel the old model request")
	}
	deliver(t, q)
	deliver(t, q)
	close(lateResponse)
	deliver(t, q)
	if a.mode != 0 || a.result != "new polish" || a.busy != "" {
		t.Fatal("late response overwrote the new result")
	}
}
