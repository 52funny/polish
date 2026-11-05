package app

import (
	"context"
	"errors"
	"testing"

	"github.com/52funny/polish/internal/config"
)

type listFunc func(context.Context, config.Config) ([]string, error)

func (f listFunc) ListModels(ctx context.Context, c config.Config) ([]string, error) {
	return f(ctx, c)
}

func TestFetchBeforeSelectingModel(t *testing.T) {
	a, q := configuredApp(t)
	a.draft.Model = ""
	a.modelLister = listFunc(func(_ context.Context, c config.Config) ([]string, error) {
		if c.APIKey != a.cfg.APIKey {
			t.Error("draft credentials not used")
		}
		return []string{"model-a", "model-b"}, nil
	})
	a.fetchModels()
	deliver(t, q)
	if a.modelsLoading || a.modelsFailed || len(a.modelIDs) != 2 || a.draft.Model != "model-a" {
		t.Fatal("model picker did not load")
	}
	if a.cfg.Model != "test-model" {
		t.Fatal("fetch modified saved configuration")
	}
}

func TestChangedConnectionDiscardsModelList(t *testing.T) {
	a, q := configuredApp(t)
	started := make(chan context.Context, 1)
	release := make(chan struct{})
	a.modelLister = listFunc(func(ctx context.Context, _ config.Config) ([]string, error) {
		started <- ctx
		<-release
		return []string{"old-provider-model"}, nil
	})
	a.fetchModels()
	ctx := <-started
	a.draft.APIKey = "new-key"
	a.connectionChanged()
	close(release)
	deliver(t, q)
	if ctx.Err() == nil || len(a.modelIDs) != 0 || a.modelsLoading {
		t.Fatal("old model list accepted")
	}
}

func TestModelListFailureKeepsManualValue(t *testing.T) {
	a, q := configuredApp(t)
	a.modelLister = listFunc(func(context.Context, config.Config) ([]string, error) {
		return nil, errors.New("discovery unavailable")
	})
	a.fetchModels()
	deliver(t, q)
	if !a.modelsFailed || a.draft.Model != "test-model" || len(a.modelIDs) != 0 {
		t.Fatal("failure broke manual fallback")
	}
}
