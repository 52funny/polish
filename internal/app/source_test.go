package app

import (
	"context"
	"testing"

	"github.com/52funny/polish/internal/config"
	"github.com/52funny/polish/internal/llm"
)

func TestReplaceNewlinesImportProcessingAndPersistence(t *testing.T) {
	a, q := configuredApp(t)
	if err := config.Save(a.configPath, a.cfg); err != nil {
		t.Fatal(err)
	}
	raw := "中文\r\nEnglish\nnext\rline\u2028tail\t  spaces"
	want := "中文 English next line tail\t  spaces"
	a.source = raw
	a.setReplaceNewlines(true)
	if a.source != want {
		t.Fatalf("current source not normalized: %q", a.source)
	}
	saved, err := config.Load(a.configPath)
	if err != nil || saved != a.cfg || !saved.ReplaceNewlines || saved.Model != "test-model" {
		t.Fatal("preference not saved or API configuration changed")
	}
	reloaded := newApp(saved, a.configPath)
	if reloaded.prepareSource(raw) != want {
		t.Fatal("preference lost after restart")
	}
	a.clipboard = clipFunc(func(context.Context, bool) (string, error) { return raw, nil })
	for _, primary := range []bool{true, false} {
		a.read(primary)
		deliver(t, q)
		if a.source != want {
			t.Fatalf("import not normalized (primary=%t): %q", primary, a.source)
		}
	}
	a.source = "pasted\ntext"
	a.changed()
	a.model = modelFunc(func(_ context.Context, _ config.Config, r llm.Request) (string, error) {
		if r.Text != "pasted text" {
			t.Errorf("model received unnormalized input: %q", r.Text)
		}
		return "结果\n段落", nil
	})
	a.process()
	deliver(t, q)
	if a.result != "结果\n段落" {
		t.Fatal("result formatting was changed")
	}
	a.setReplaceNewlines(false)
	a.read(false)
	deliver(t, q)
	if a.source != raw {
		t.Fatal("disabled option changed imported text")
	}
}

func TestReplaceNewlinesBeforeAPISetup(t *testing.T) {
	a, _ := configuredApp(t)
	a.cfg = config.Config{}
	a.draft = a.cfg
	a.setReplaceNewlines(true)
	saved, err := config.Load(a.configPath)
	if err != nil || !saved.ReplaceNewlines || !a.draft.ReplaceNewlines {
		t.Fatal("preference needs to work before API setup")
	}
}
