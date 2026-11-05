package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/52funny/polish/internal/config"
)

func TestListModels(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Error("incorrect discovery request")
		}
		w.Write([]byte(`{"data":[{"id":"zeta"},{"id":"alpha"},{"id":"zeta"},{"id":" "}]}`))
	}))
	defer s.Close()
	ids, err := New().ListModels(context.Background(), config.Config{BaseURL: s.URL + "/v1/", APIKey: "secret"})
	if err != nil || !slices.Equal(ids, []string{"alpha", "zeta"}) {
		t.Fatalf("models: %v %v", ids, err)
	}
}

func TestOpenCodeModelCatalogue(t *testing.T) {
	for _, prefix := range []string{"/inference", "/inference/go"} {
		c := New()
		base := "https://opencode.ai" + prefix + "/openai/v1"
		c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() != "https://opencode.ai"+prefix+"/v1/models" {
				t.Fatalf("wrong catalogue: %s", r.URL)
			}
			if prefix == "/inference/go" && r.Header.Get("x-opencode-session") == "" {
				t.Fatal("session ID missing")
			}
			body := `{"data":[{"id":"kimi-k3","api":{"chat-completions":{"base_url":"` + base + `"}}},{"id":"responses-only","api":{"responses":{"base_url":"` + base + `"}}},{"id":"other-endpoint","api":{"chat-completions":{"base_url":"https://elsewhere.example/v1"}}}]}`
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})
		ids, err := c.ListModels(context.Background(), config.Config{BaseURL: base, APIKey: "key"})
		if err != nil || !slices.Equal(ids, []string{"kimi-k3"}) {
			t.Fatalf("filter: %v %v", ids, err)
		}
	}
}

func TestModelListFailures(t *testing.T) {
	for _, tc := range []struct {
		status     int
		body, want string
	}{
		{401, "secret", "authentication"}, {404, "secret", "manually"}, {500, "secret", "500"},
		{200, "not JSON secret", "format"}, {200, `{"data":[]}`, "No models"},
	} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) }))
		_, err := New().ListModels(context.Background(), config.Config{BaseURL: s.URL, APIKey: "key"})
		s.Close()
		if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "secret") {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := New().ListModels(ctx, config.Config{BaseURL: "https://example.org/v1", APIKey: "key"})
	if err == nil {
		t.Fatal("cancellation ignored")
	}
}

// Opt-in read-only verification with the user's configured service. Never logs keys.
func TestLiveModelList(t *testing.T) {
	if os.Getenv("POLISH_LIVE_MODELS") != "1" {
		t.Skip("set POLISH_LIVE_MODELS=1 to query the configured service")
	}
	path, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := New().ListModels(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Received %d candidate models; configured model included: %t", len(ids), slices.Contains(ids, cfg.Model))
}
