package app

import (
	"fmt"
	"slices"

	"github.com/52funny/polish/internal/config"
	"github.com/52funny/polish/internal/i18n"
	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/locales/zh-Hans"
	"github.com/egoist/mygo/ui"
)

// Run starts the native application on the main goroutine.
func Run(name string, selection bool) error {
	mygo.App.SetName(name)
	if !mygo.App.RequestSingleInstanceLock() {
		return nil
	}
	path, err := config.Path()
	if err != nil {
		return fmt.Errorf("Could not determine the settings directory: %w", err)
	}
	cfg, loadErr := config.Load(path)
	a := newApp(cfg, path)
	if loadErr != nil {
		a.settingsError = i18n.FromError(loadErr)
		a.settings = true
	}
	var win *mygo.Window
	mygo.App.OnSecondInstance(func(args []string, _ string) {
		mygo.App.WhenReady(func() {
			if slices.ContainsFunc(args, func(arg string) bool {
				return arg == "--selection" || arg == "-selection" || arg == "--selection=true" || arg == "-selection=true"
			}) {
				a.read(true)
			} else {
				a.reveal()
			}
		})
	})
	mygo.App.WhenReady(func() {
		mygo.App.SetLanguages("en", "zh-Hans")
		mygo.App.SetLocale(a.language())
		mygo.App.SetMenu(nil)
		win = mygo.NewWindow(mygo.WindowOptions{
			Title: a.tr("Polish · Writing and translation"), Width: 960, Height: 700,
			MinWidth: 720, MinHeight: 560, Hidden: true,
			Content: ui.View(a.view),
		})
		a.post = win.Update
		a.languageChanged = func(locale string) {
			mygo.App.SetLocale(locale)
			win.SetTitle(a.tr("Polish · Writing and translation"))
			win.Invalidate()
		}
		a.reveal = func() { win.Show(); win.Restore(); win.Focus(); win.Invalidate() }
		win.OnClosed(func() { a.stop(); a.stopModelList() })
		if selection {
			a.read(true)
		} else {
			a.reveal()
		}
	})
	return mygo.App.Run()
}
