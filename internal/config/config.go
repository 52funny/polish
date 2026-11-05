package config

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/52funny/polish/internal/i18n"
)

type Config struct {
	BaseURL         string `json:"base_url"`
	Model           string `json:"model"`
	APIKey          string `json:"api_key"`
	ReplaceNewlines bool   `json:"replace_newlines"`
	Language        string `json:"language,omitempty"`
}

func (c Config) Normalized() Config {
	c.BaseURL = strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	c.Model = strings.TrimSpace(c.Model)
	c.APIKey = strings.TrimSpace(c.APIKey)
	if c.Language != "" && c.Language != "en" && c.Language != "zh-Hans" {
		c.Language = "en"
	}
	return c
}

func (c Config) Validate() error {
	if err := c.ValidateConnection(); err != nil {
		return err
	}
	if strings.TrimSpace(c.Model) == "" {
		return i18n.Text("Enter a model name.")
	}
	return nil
}

// ValidateConnection also supports discovering models before one is selected.
func (c Config) ValidateConnection() error {
	c = c.Normalized()
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return i18n.Text("Enter a valid HTTP(S) API base URL, such as https://api.example.com/v1.")
	}
	if strings.HasSuffix(u.Path, "/chat/completions") {
		return i18n.Text("Enter the API base URL without /chat/completions.")
	}
	if c.APIKey == "" || strings.ContainsAny(c.APIKey, "\r\n") {
		return i18n.Text("Enter a valid API key.")
	}
	return nil
}

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "polish", "config.json"), nil
}

func Load(path string) (Config, error) {
	var c Config
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, i18n.Text("Could not read settings. Check the file permissions.")
	}
	if json.Unmarshal(b, &c) != nil {
		return Config{}, i18n.Text("The settings file is invalid. Enter and save your settings again.")
	}
	return c.Normalized(), nil
}

func Save(path string, c Config) error {
	c = c.Normalized()
	if err := c.Validate(); err != nil {
		return err
	}
	return write(path, c)
}

// SaveReplaceNewlines can save this preference before an API is configured.
// Read the saved settings so that toggling it cannot overwrite API fields.
func SaveReplaceNewlines(path string, enabled bool) error {
	c, err := Load(path)
	if err != nil {
		return err
	}
	c.ReplaceNewlines = enabled
	return write(path, c)
}

// SaveLanguage preserves saved API settings and also works before API setup.
func SaveLanguage(path, language string) error {
	if language != "" && language != "en" && language != "zh-Hans" {
		return i18n.Text("Unsupported interface language.")
	}
	c, err := Load(path)
	if err != nil {
		return err
	}
	c.Language = language
	return write(path, c)
}

func write(path string, c Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return i18n.Text("Could not create the settings directory: %s", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return i18n.Text("Could not update settings permissions.")
	}
	f, err := os.CreateTemp(dir, ".config-*")
	if err != nil {
		return i18n.Text("Could not create the settings file.")
	}
	defer os.Remove(f.Name())
	b, _ := json.MarshalIndent(c, "", "  ")
	_, writeErr := f.Write(append(b, '\n'))
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		return i18n.Text("Could not save settings.")
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return i18n.Text("Could not replace the settings file.")
	}
	return nil
}
