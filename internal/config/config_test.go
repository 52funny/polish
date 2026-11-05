package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadAndPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "polish", "config.json")
	if c, err := Load(path); err != nil || c != (Config{}) {
		t.Fatalf("missing: %v %v", c, err)
	}
	cfg := Config{BaseURL: " https://example.org/v1/ ", Model: " model ", APIKey: " secret "}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil || c != cfg.Normalized() {
		t.Fatalf("roundtrip failed: %v", err)
	}
	for p, mode := range map[string]os.FileMode{filepath.Dir(path): 0700, path: 0600} {
		st, err := os.Stat(p)
		if err != nil || st.Mode().Perm() != mode {
			t.Fatalf("permissions for %s: %v %v", p, st, err)
		}
	}
	if err := Save(path, Config{}); err == nil {
		t.Fatal("invalid config saved")
	}
	if c, _ := Load(path); c != cfg.Normalized() {
		t.Fatal("invalid save replaced config")
	}
	if err := os.WriteFile(path, []byte("{invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("invalid JSON accepted")
	}
}

func TestValidate(t *testing.T) {
	for _, base := range []string{"", "ftp://example.org", "https://", "https://u:p@example.org", "https://example.org?token=secret", "https://example.org/#fragment", "https://example.org/v1/chat/completions"} {
		if (Config{BaseURL: base, Model: "model", APIKey: "key"}).Validate() == nil {
			t.Errorf("accepted %q", base)
		}
	}
	for _, base := range []string{"https://example.org/v1", "http://127.0.0.1:1234/v1"} {
		if err := (Config{BaseURL: base, Model: "model", APIKey: "key"}).Validate(); err != nil {
			t.Error(err)
		}
	}
}

func TestConfigPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	p, err := Path()
	if err != nil || p != filepath.Join(dir, "polish", "config.json") {
		t.Fatalf("path: %q %v", p, err)
	}
}
