package app

import (
	"context"
	"errors"
	"strings"

	"github.com/52funny/polish/internal/config"
	"github.com/52funny/polish/internal/desktop"
	"github.com/52funny/polish/internal/i18n"
	"github.com/52funny/polish/internal/llm"
)

type processor interface {
	Process(context.Context, config.Config, llm.Request) (string, error)
}
type modelLister interface {
	ListModels(context.Context, config.Config) ([]string, error)
}
type clipboard interface {
	Read(context.Context, bool) (string, error)
	Copy(context.Context, string) error
}

// All state is owned by the UI thread. Workers only publish through post.
type app struct {
	cfg, draft       config.Config
	configPath       string
	source, result   string
	mode             int
	style, target    string
	status           i18n.Message
	failed           bool
	settings         bool
	settingsError    i18n.Message
	busy             string
	generation       uint64
	cancel           context.CancelFunc
	model            processor
	clipboard        clipboard
	post             func(func())
	reveal           func()
	systemLanguage   string
	languageChanged  func(string)
	modelLister      modelLister
	modelIDs         []string
	modelsLoading    bool
	modelsNotice     i18n.Message
	modelsFailed     bool
	manualModel      bool
	modelsGeneration uint64
	modelsCancel     context.CancelFunc
}

func newApp(cfg config.Config, path string) *app {
	client := llm.New()
	return &app{
		cfg: cfg, draft: cfg, configPath: path,
		style: "natural", target: "zh-Hans",
		systemLanguage: i18n.SystemLanguage(), languageChanged: func(string) {},
		status:   i18n.Text("Select text and use your shortcut, or enter text here."),
		settings: cfg.Validate() != nil,
		model:    client, modelLister: client, clipboard: desktop.New(),
		reveal: func() {},
	}
}

func (a *app) stop() {
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	a.generation++
	a.busy = ""
}

func (a *app) run(kind string, work func(context.Context) (string, error), done func(string, error)) {
	a.stop()
	id := a.generation
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel, a.busy = cancel, kind
	go func() {
		text, err := work(ctx)
		a.post(func() {
			if id != a.generation {
				return
			}
			cancel()
			a.cancel, a.busy = nil, ""
			done(text, err)
		})
	}()
}

func (a *app) message(text i18n.Message, failed bool) { a.status, a.failed = text, failed }

func (a *app) changed() {
	a.source = a.prepareSource(a.source)
	a.stop()
	a.result = ""
	a.message(i18n.Text("Source or options changed. Start processing when ready."), false)
}

func (a *app) read(primary bool) {
	a.source, a.result = "", ""
	a.message(i18n.Text("Reading text…"), false)
	a.run("read", func(ctx context.Context) (string, error) {
		return a.clipboard.Read(ctx, primary)
	}, func(text string, err error) {
		if err != nil {
			a.message(i18n.FromError(err), true)
		} else {
			a.source = a.prepareSource(text)
			a.message(i18n.Text("Text loaded. Choose polishing or translation to continue."), false)
		}
		a.reveal()
	})
}

func (a *app) process() {
	a.source = a.prepareSource(a.source)
	if strings.TrimSpace(a.source) == "" {
		a.message(i18n.Text("Enter some source text first."), true)
		return
	}
	if err := a.cfg.Validate(); err != nil {
		a.settings = true
		a.settingsError = i18n.FromError(err)
		return
	}
	r := llm.Request{Text: a.source, Operation: llm.Polish, Style: a.style, Target: a.target}
	if a.mode == 1 {
		r.Operation = llm.Translate
	}
	cfg := a.cfg
	a.result = ""
	a.message(i18n.Text("Processing. You can cancel at any time…"), false)
	a.run("process", func(ctx context.Context) (string, error) {
		return a.model.Process(ctx, cfg, r)
	}, func(text string, err error) {
		if err != nil {
			if errors.Is(err, context.Canceled) {
				a.message(i18n.Text("Processing canceled."), false)
			} else {
				a.message(i18n.FromError(err), true)
			}
			return
		}
		a.result = text
		a.message(i18n.Text("Done. Review the text, then copy the result."), false)
	})
}

func (a *app) copy() {
	text := a.result
	a.message(i18n.Text("Copying…"), false)
	a.run("copy", func(ctx context.Context) (string, error) {
		return "", a.clipboard.Copy(ctx, text)
	}, func(_ string, err error) {
		if err != nil {
			a.message(i18n.FromError(err), true)
		} else {
			a.message(i18n.Text("Result copied."), false)
		}
	})
}
