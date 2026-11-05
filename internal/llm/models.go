package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/52funny/polish/internal/config"
	"github.com/52funny/polish/internal/i18n"
)

func modelsURL(base string) string {
	u, _ := url.Parse(base)
	if u.Scheme == "https" && strings.EqualFold(u.Hostname(), "opencode.ai") {
		switch u.Path {
		case "/inference/go/openai/v1":
			u.Path = "/inference/go/v1"
		case "/inference/openai/v1":
			u.Path = "/inference/v1"
		}
	}
	u.Path += "/models"
	return u.String()
}

// ListModels returns candidate IDs, not a guarantee of account quota or access.
func (c Client) ListModels(ctx context.Context, cfg config.Config) ([]string, error) {
	cfg = cfg.Normalized()
	if err := cfg.ValidateConnection(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL(cfg.BaseURL), nil)
	if err != nil {
		return nil, i18n.Text("Could not create a model list request. Check the endpoint.")
	}
	c.authorize(req, cfg)
	res, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, i18n.Text("Fetching models was canceled or timed out. Retry or enter a model manually.")
		}
		return nil, i18n.Text("Could not fetch models. Check your network or enter a model manually.")
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusMethodNotAllowed {
		return nil, i18n.Text("This service does not provide a model list. Enter a model name manually.")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, statusError(res.StatusCode)
	}
	const maxList = 8 << 20
	b, err := io.ReadAll(io.LimitReader(res.Body, maxList+1))
	if err != nil {
		return nil, i18n.Text("Could not read the model list. Retry or enter a model manually.")
	}
	if len(b) > maxList {
		return nil, i18n.Text("The model list is too large. Enter a model name manually.")
	}
	var list struct {
		Data []struct {
			ID  string `json:"id"`
			API map[string]struct {
				BaseURL string `json:"base_url"`
			} `json:"api"`
		} `json:"data"`
	}
	if json.Unmarshal(b, &list) != nil {
		return nil, i18n.Text("The model list format is invalid. Enter a model name manually.")
	}
	openCode := req.URL.Scheme == "https" && strings.EqualFold(req.URL.Hostname(), "opencode.ai")
	var ids []string
	for _, model := range list.Data {
		id := strings.TrimSpace(model.ID)
		if id == "" {
			continue
		}
		// OpenCode's catalogue mixes API families. Polish uses Chat Completions.
		if openCode && len(model.API) > 0 {
			api, ok := model.API["chat-completions"]
			if !ok || strings.TrimRight(api.BaseURL, "/") != cfg.BaseURL {
				continue
			}
		}
		ids = append(ids, id)
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if len(ids) == 0 {
		return nil, i18n.Text("No models were returned for this endpoint. Enter one manually or change the endpoint.")
	}
	return ids, nil
}
