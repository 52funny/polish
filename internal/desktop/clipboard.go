package desktop

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/52funny/polish/internal/i18n"
)

const Timeout = 2 * time.Second

type Runner func(context.Context, string, []string, io.Reader) ([]byte, error)

type Clipboard struct{ Run Runner }

func New() Clipboard { return Clipboard{Run: run} }

func run(ctx context.Context, name string, args []string, input io.Reader) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = input
	// A helper retaining its output pipe must not stall the UI indefinitely.
	cmd.WaitDelay = 200 * time.Millisecond
	return cmd.Output()
}

func (c Clipboard) Read(ctx context.Context, primary bool) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	args := []string{"--no-newline", "--type", "text"}
	if primary {
		args = append(args, "--primary")
	}
	b, err := c.Run(ctx, "wl-paste", args, nil)
	if ctx.Err() != nil {
		return "", i18n.Text("Reading was canceled or timed out. Paste your text manually.")
	}
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", i18n.Text("wl-paste was not found. Install wl-clipboard.")
		}
		if primary {
			return "", i18n.Text("Could not read the selection. Copy your text and use Read clipboard, or enter it manually.")
		}
		return "", i18n.Text("Could not read clipboard text. Copy some text and try again.")
	}
	if !utf8.Valid(b) || strings.TrimSpace(string(b)) == "" {
		return "", i18n.Text("No valid text was found. Select, copy, or enter some text.")
	}
	return string(b), nil
}

func (c Clipboard) Copy(ctx context.Context, text string) error {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	_, err := c.Run(ctx, "wl-copy", []string{"--type", "text/plain;charset=utf-8"}, strings.NewReader(text))
	if err != nil || ctx.Err() != nil {
		return i18n.Text("Copy failed. Check wl-clipboard and your Wayland session.")
	}
	return nil
}
