package app

import (
	"context"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/52funny/polish/internal/config"
	"github.com/52funny/polish/internal/llm"
	"github.com/egoist/mygo/ui"
)

type modelFunc func(context.Context, config.Config, llm.Request) (string, error)

func (f modelFunc) Process(ctx context.Context, c config.Config, r llm.Request) (string, error) {
	return f(ctx, c, r)
}

type clipFunc func(context.Context, bool) (string, error)

func (f clipFunc) Read(ctx context.Context, p bool) (string, error) { return f(ctx, p) }
func (f clipFunc) Copy(context.Context, string) error               { return nil }

func configuredApp(t *testing.T) (*app, chan func()) {
	t.Helper()
	a := newApp(config.Config{BaseURL: "http://localhost/v1", Model: "test-model", APIKey: "test-secret", Language: "en"}, filepath.Join(t.TempDir(), "config.json"))
	q := make(chan func(), 10)
	a.post = func(f func()) { q <- f }
	t.Cleanup(func() { a.stop(); a.stopModelList() })
	return a, q
}

func deliver(t *testing.T, q chan func()) {
	t.Helper()
	select {
	case f := <-q:
		f()
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not finish")
	}
}

func TestCanceledResponseCannotOverwriteNewTask(t *testing.T) {
	a, q := configuredApp(t)
	oldResponse := make(chan struct{})
	started := make(chan context.Context, 1)
	a.model = modelFunc(func(ctx context.Context, _ config.Config, r llm.Request) (string, error) {
		if r.Text == "old" {
			started <- ctx
			<-oldResponse
		}
		return r.Text + " result", nil // Deliberately ignore cancellation to model a late response.
	})
	a.source = "old"
	a.process()
	ctx := <-started
	a.source = "new"
	a.changed()
	if ctx.Err() == nil {
		t.Fatal("old request was not canceled")
	}
	a.process()
	deliver(t, q)
	if a.result != "new result" {
		t.Fatal("new result missing")
	}
	close(oldResponse)
	deliver(t, q)
	if a.result != "new result" {
		t.Fatal("old response overwrote current result")
	}
}

func TestSelectionFailureClearsTextBeforeReveal(t *testing.T) {
	a, q := configuredApp(t)
	a.source, a.result = "previous original", "previous result"
	a.clipboard = clipFunc(func(context.Context, bool) (string, error) { return "", errors.New("no selection") })
	revealed := false
	a.reveal = func() {
		revealed = true
		if a.source != "" || a.result != "" {
			t.Error("stale text displayed")
		}
	}
	a.read(true)
	if revealed {
		t.Fatal("focused window before reading selection")
	}
	deliver(t, q)
	if !revealed || !a.failed || a.busy != "" {
		t.Fatal("failed selection not surfaced")
	}
}

func TestFailedRequestPreservesSourceAndCancelDiscardsReply(t *testing.T) {
	a, q := configuredApp(t)
	a.model = modelFunc(func(context.Context, config.Config, llm.Request) (string, error) {
		return "", errors.New("network unavailable")
	})
	a.source = "原文\n第二段"
	a.process()
	deliver(t, q)
	if a.source != "原文\n第二段" || !a.failed || a.result != "" {
		t.Fatal("failure lost source or exposed result")
	}
	a.run("process", func(context.Context) (string, error) { return "late reply", nil }, func(s string, _ error) { a.result = s })
	a.stop()
	deliver(t, q)
	if a.result != "" || a.busy != "" {
		t.Fatal("canceled result was applied")
	}
}

func TestNativeViews(t *testing.T) {
	a, _ := configuredApp(t)
	for _, language := range []string{"en", "zh-Hans"} {
		a.cfg.Language = language
		for _, size := range [][2]int{{960, 700}, {720, 560}} {
			for _, dark := range []bool{false, true} {
				for _, settings := range []bool{false, true} {
					a.settings = settings
					if settings {
						a.modelIDs = []string{"test-model", "other-model"}
					} else {
						a.modelIDs = nil
					}
					a.source = "这个工具可以帮助我把想法表达得更加清楚。\n\nPlease translate this paragraph."
					a.result = "这个工具能帮助我更清晰地表达想法。\n\n请翻译这一段。"
					tt := ui.NewTester(a.view, size[0], size[1])
					tt.SetDark(dark)
					tt.Frame()
					want := a.tr("Copy result")
					if settings {
						want = a.tr("Save settings")
					}
					if r, ok := tt.Find(want); !ok || r.Y+r.H > float32(size[1]) || r.X+r.W > float32(size[0]) {
						t.Fatalf("action %s offscreen: %+v", want, r)
					}
					if slices.Contains(tt.Texts(), "test-secret") {
						t.Fatal("key exposed in UI")
					}
					if dir := os.Getenv("POLISH_TEST_ARTIFACTS"); dir != "" && size[0] == 960 {
						name := "editor"
						if settings {
							name = "settings"
						}
						if dark {
							name += "-dark"
						}
						if err := os.MkdirAll(dir, 0700); err != nil {
							t.Fatal(err)
						}
						f, err := os.Create(filepath.Join(dir, name+"-"+language+".png"))
						if err != nil {
							t.Fatal(err)
						}
						if err := png.Encode(f, tt.Image()); err != nil {
							t.Fatal(err)
						}
						f.Close()
					}
				}
			}
		}
	}
}
