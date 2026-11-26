package app

import (
	"fmt"

	"github.com/52funny/polish/internal/cli"
	"github.com/52funny/polish/internal/config"
	"github.com/52funny/polish/internal/i18n"
	"github.com/egoist/mygo"
	_ "github.com/egoist/mygo/locales/zh-Hans"
	"github.com/egoist/mygo/ui"
)

// Run starts the native application on the main goroutine.
func Run(name string, options cli.Options) error {
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
		next, err := cli.Parse(args)
		if err != nil {
			return // The entry point validates arguments before forwarding them.
		}
		mygo.App.WhenReady(func() {
			a.launch(next)
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
		a.close = func() { win.Update(win.Close) }
		a.languageChanged = func(locale string) {
			mygo.App.SetLocale(locale)
			win.SetTitle(a.tr("Polish · Writing and translation"))
			win.Invalidate()
		}
		a.reveal = func() { win.Show(); win.Restore(); win.Focus(); win.Invalidate() }
		win.OnClosed(func() { a.stop(); a.stopModelList() })
		a.launch(options)
	})
	return mygo.App.Run()
}
