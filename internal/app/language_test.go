package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/52funny/polish/internal/config"
	"github.com/52funny/polish/internal/i18n"
	"github.com/egoist/mygo/ui"
)

func TestSwitchLanguagePreservesWorkAndPersistsPreference(t *testing.T) {
	a, _ := configuredApp(t)
	if err := config.Save(a.configPath, a.cfg); err != nil {
		t.Fatal(err)
	}
	a.source, a.result = "中文\nsource", "existing\nresult"
	a.style, a.target = "formal", "en"
	a.draft.Model = "unsaved-model"
	a.message(i18n.Text("Result copied."), false)
	tt := ui.NewTester(a.view, 960, 700)
	tt.Frame()
	changed := ""
	a.languageChanged = func(locale string) { changed = locale }
	a.setLanguage("zh-Hans")
	tt.Frame()
	if changed != "zh-Hans" || !slices.Contains(tt.Texts(), "模型设置") || !slices.Contains(tt.Texts(), "正式") {
		t.Fatal("interface did not change to Chinese")
	}
	if !strings.Contains(a.status.String(a.language()), "已复制") {
		t.Fatal("existing status was not translated")
	}
	saved, err := config.Load(a.configPath)
	if err != nil || saved.Language != "zh-Hans" || saved.Model != "test-model" {
		t.Fatal("language preference changed saved API settings")
	}
	if a.draft.Model != "unsaved-model" {
		t.Fatal("unsaved API edits were lost")
	}
	a.setLanguage("en")
	tt.Frame()
	if a.source != "中文\nsource" || a.result != "existing\nresult" || a.style != "formal" || a.target != "en" {
		t.Fatal("language switch changed working text or processing options")
	}
}

func TestLanguagePreferenceBeforeAPISetup(t *testing.T) {
	a, _ := configuredApp(t)
	a.cfg, a.draft = config.Config{}, config.Config{}
	a.systemLanguage = "zh-Hans"
	if a.language() != "zh-Hans" {
		t.Fatal("default should follow the system")
	}
	a.setLanguage("en")
	saved, err := config.Load(a.configPath)
	if err != nil || saved.Language != "en" || a.language() != "en" {
		t.Fatal("preference was not saved before API setup")
	}
	a.setLanguage("")
	if a.language() != "zh-Hans" {
		t.Fatal("system preference was not restored")
	}
}
