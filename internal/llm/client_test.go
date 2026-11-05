package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/52funny/polish/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOpenCodeGoSessionHeaders(t *testing.T) {
	c := New()
	var sessions []string
	c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		defer r.Body.Close()
		if r.Header.Get("User-Agent") != "polish/0.1 (desktop text editor)" {
			t.Fatal("missing real application identity")
		}
		sessions = append(sessions, r.Header.Get("x-opencode-session"))
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"OK"},"finish_reason":"stop"}]}`)), Header: make(http.Header)}, nil
	})
	for _, base := range []string{
		"https://opencode.ai/inference/go/openai/v1", "https://opencode.ai/inference/go/openai/v1",
		"https://opencode.ai/inference/openai/v1", "https://other.example/inference/go/openai/v1",
		"https://opencode.ai.example/inference/go/openai/v1",
	} {
		_, err := c.Process(context.Background(), config.Config{BaseURL: base, Model: "kimi-k3", APIKey: "test-key"}, Request{Text: "test", Operation: Polish, Style: "natural"})
		if err != nil {
			t.Fatal(err)
		}
	}
	if sessions[0] == "" || sessions[0] != sessions[1] {
		t.Fatal("session must be nonempty and stable")
	}
	for _, s := range sessions[2:] {
		if s != "" {
			t.Fatal("provider session leaked to a different endpoint")
		}
	}
	if c.sessionID == New().sessionID {
		t.Fatal("new processing session must have a fresh ID")
	}
}

func TestProcessRequests(t *testing.T) {
	for _, r := range []Request{
		{Text: " 原文\n第二段 ", Operation: Polish, Style: "natural"},
		{Text: "text", Operation: Polish, Style: "formal"},
		{Text: "text", Operation: Polish, Style: "concise"},
		{Text: "text", Operation: Translate, Target: "zh-Hans"},
		{Text: "中文", Operation: Translate, Target: "en"},
	} {
		t.Run(string(r.Operation)+r.Style+r.Target, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.URL.Path != "/v1/chat/completions" || req.Method != "POST" || req.Header.Get("Authorization") != "Bearer secret" || req.Header.Get("Content-Type") != "application/json" {
					t.Error("incorrect request headers or endpoint")
				}
				var body struct {
					Model    string
					Stream   bool
					Messages []struct{ Role, Content string }
				}
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body.Model != "model" || body.Stream || len(body.Messages) != 2 {
					t.Error("incorrect request body")
					return
				}
				if body.Messages[0].Role != "system" || body.Messages[1].Role != "user" || body.Messages[1].Content != r.Text {
					t.Error("incorrect messages")
				}
				want := r.Style
				if r.Operation == Translate {
					want = map[string]string{"zh-Hans": "Simplified Chinese", "en": "English"}[r.Target]
				}
				if !strings.Contains(body.Messages[0].Content, want) {
					t.Error("missing instructions")
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"choices":[{"message":{"content":"结果\n第二段"},"finish_reason":"stop"}]}`))
			}))
			defer s.Close()
			got, err := New().Process(context.Background(), config.Config{BaseURL: s.URL + "/v1/", Model: "model", APIKey: "secret"}, r)
			if err != nil || got != "结果\n第二段" {
				t.Fatalf("got %q %v", got, err)
			}
		})
	}
}

func TestProcessFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"auth", 401, "private text and secret", "authentication"}, {"forbidden", 403, "private", "authentication"},
		{"quota", 429, "private", "quota"}, {"server", 503, "private", "503"},
		{"bad-json", 200, "private", "JSON"}, {"empty", 200, `{"choices":[]}`, "no text"},
		{"blank", 200, `{"choices":[{"message":{"content":" "}}]}`, "no text"},
		{"truncated", 200, `{"choices":[{"message":{"content":"partial"},"finish_reason":"length"}]}`, "incomplete"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) }))
			defer s.Close()
			_, err := New().Process(context.Background(), config.Config{BaseURL: s.URL, Model: "m", APIKey: "secret"}, Request{Text: "text", Operation: Polish, Style: "natural"})
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "secret") {
				t.Fatalf("unsafe/unexpected error: %v", err)
			}
		})
	}
}

func TestCancellationAndTimeout(t *testing.T) {
	for _, explicit := range []bool{true, false} {
		t.Run(map[bool]string{true: "cancel", false: "timeout"}[explicit], func(t *testing.T) {
			started := make(chan struct{})
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.Copy(io.Discard, r.Body)
				close(started)
				<-r.Context().Done()
			}))
			defer s.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c := New()
			if explicit {
				go func() { <-started; cancel() }()
			} else {
				c.HTTP.Timeout = 30 * time.Millisecond
			}
			_, err := c.Process(ctx, config.Config{BaseURL: s.URL, Model: "m", APIKey: "secret"}, Request{Text: "text", Operation: Polish, Style: "natural"})
			if err == nil {
				t.Fatal("expected error")
			}
			if explicit && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancel error: %v", err)
			}
			if !explicit && !strings.Contains(err.Error(), "timed out") {
				t.Fatalf("timeout error: %v", err)
			}
		})
	}
}

func TestNoRedirectAndNoEmptyInput(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/another", http.StatusTemporaryRedirect)
	}))
	defer s.Close()
	cfg := config.Config{BaseURL: s.URL, Model: "m", APIKey: "secret"}
	if _, err := New().Process(context.Background(), cfg, Request{Text: "text", Operation: Polish, Style: "natural"}); err == nil || !strings.Contains(err.Error(), "307") {
		t.Fatalf("redirect: %v", err)
	}
	if _, err := New().Process(context.Background(), cfg, Request{Text: " \n"}); err == nil {
		t.Fatal("blank input accepted")
	}
}
