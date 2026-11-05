package llm

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/52funny/polish/internal/config"
	"github.com/52funny/polish/internal/i18n"
)

type Operation string

const (
	Polish    Operation = "polish"
	Translate Operation = "translate"
)

type Request struct {
	Text      string
	Operation Operation
	Style     string
	Target    string
}

type Client struct {
	HTTP *http.Client
	// One desktop processing session; reused for requests and manual retries.
	sessionID string
}

func New() Client {
	return Client{sessionID: rand.Text(), HTTP: &http.Client{
		Timeout: 60 * time.Second,
		// Never forward selected text or credentials to a redirected endpoint.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (c Client) Process(ctx context.Context, cfg config.Config, r Request) (string, error) {
	if strings.TrimSpace(r.Text) == "" {
		return "", i18n.Text("Enter some source text first.")
	}
	cfg = cfg.Normalized()
	if err := cfg.Validate(); err != nil {
		return "", err
	}
	system, err := prompt(r)
	if err != nil {
		return "", err
	}
	body, _ := json.Marshal(struct {
		Model    string              `json:"model"`
		Messages []map[string]string `json:"messages"`
		Stream   bool                `json:"stream"`
	}{cfg.Model, []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": r.Text}}, false})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", i18n.Text("Could not create a model request. Check the endpoint.")
	}
	req.Header.Set("Content-Type", "application/json")
	c.authorize(req, cfg)
	res, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		var timeout interface{ Timeout() bool }
		if errors.As(err, &timeout) && timeout.Timeout() {
			return "", i18n.Text("The model request timed out. Try again.")
		}
		return "", i18n.Text("Could not connect to the model service. Check the endpoint and network.")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", statusError(res.StatusCode)
	}
	const maxResponse = 8 << 20
	b, err := io.ReadAll(io.LimitReader(res.Body, maxResponse+1))
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", i18n.Text("Could not read the model response. Try again.")
	}
	if len(b) > maxResponse {
		return "", i18n.Text("The model response is too large. Shorten the source text and try again.")
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if json.Unmarshal(b, &result) != nil {
		return "", i18n.Text("The model service returned invalid JSON.")
	}
	if len(result.Choices) == 0 || strings.TrimSpace(result.Choices[0].Message.Content) == "" {
		return "", i18n.Text("The model returned no text. Check that it supports text chat.")
	}
	if reason := result.Choices[0].FinishReason; reason != "" && reason != "stop" {
		return "", i18n.Text("The model response is incomplete. Shorten the text or choose another model.")
	}
	return result.Choices[0].Message.Content, nil
}

func (c Client) authorize(req *http.Request, cfg config.Config) {
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("User-Agent", "polish/0.1 (desktop text editor)")
	// OpenCode Go requires a stable session ID for routing. Keep this
	// provider-specific header away from unrelated compatible services.
	if isOpenCodeGo(req.URL) {
		sessionID := c.sessionID
		if sessionID == "" {
			sessionID = rand.Text()
		}
		req.Header.Set("x-opencode-session", sessionID)
	}
}

func statusError(status int) error {
	switch status {
	case 401, 403:
		return i18n.Text("Model service authentication failed. Check your API key and access.")
	case 429:
		return i18n.Text("The model service rate limit or quota was reached. Try later or check your account.")
	default:
		return i18n.Text("The model service returned HTTP %d. Check the endpoint and model, or try later.", status)
	}
}

func isOpenCodeGo(u *url.URL) bool {
	return u.Scheme == "https" && strings.EqualFold(u.Hostname(), "opencode.ai") && strings.HasPrefix(u.Path, "/inference/go/")
}
