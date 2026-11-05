package desktop

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCopyDaemonExitStatus(t *testing.T) {
	for _, tc := range []struct {
		name, script string
		wantError    bool
	}{
		{"background-owner", "#!/bin/sh\ncat >/dev/null\nsleep 1 >&2 &\nexit 0\n", false},
		{"helper-failure", "#!/bin/sh\ncat >/dev/null\nexit 7\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "wl-copy"), []byte(tc.script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			err := New().Copy(context.Background(), "multiline\ntext")
			if (err != nil) != tc.wantError {
				t.Fatalf("copy error = %v; want error: %t", err, tc.wantError)
			}
		})
	}
}

func TestReadSelectionPreservesTextAndDoesNotFallback(t *testing.T) {
	for _, primary := range []bool{true, false} {
		calls := 0
		c := Clipboard{Run: func(ctx context.Context, name string, args []string, _ io.Reader) ([]byte, error) {
			calls++
			if name != "wl-paste" || slices.Contains(args, "--primary") != primary || !slices.Contains(args, "--no-newline") {
				t.Fatalf("wrong command: %s %v", name, args)
			}
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("missing deadline")
			}
			return []byte("  中文\nSecond line\n"), nil
		}}
		s, err := c.Read(context.Background(), primary)
		if err != nil || s != "  中文\nSecond line\n" || calls != 1 {
			t.Fatalf("read: %q %v", s, err)
		}
	}
}

// Opt-in integration check uses PRIMARY so the normal clipboard is untouched.
func TestLiveClipboardCopy(t *testing.T) {
	if os.Getenv("POLISH_LIVE_CLIPBOARD") != "1" {
		t.Skip("set POLISH_LIVE_CLIPBOARD=1 in a Wayland session")
	}
	const sample = "Polish clipboard regression check.\n剪贴板验证"
	var helperErr error
	c := Clipboard{Run: func(ctx context.Context, name string, args []string, input io.Reader) ([]byte, error) {
		out, err := run(ctx, name, append([]string{"--primary"}, args...), input)
		if name == "wl-copy" {
			helperErr = err
		}
		return out, err
	}}
	err := c.Copy(context.Background(), sample)
	got, readErr := c.Read(context.Background(), false)
	if readErr == nil && got == sample {
		t.Cleanup(func() {
			// Do not clear a newer selection made by the user during this check.
			if current, readErr := c.Read(context.Background(), false); readErr == nil && current == sample {
				ctx, cancel := context.WithTimeout(context.Background(), Timeout)
				defer cancel()
				_, _ = run(ctx, "wl-copy", []string{"--primary", "--clear"}, nil)
			}
		})
	}
	if err != nil {
		t.Fatalf("wl-copy returned %v; clipboard payload matched: %t", helperErr, readErr == nil && got == sample)
	}
	if readErr != nil || got != sample {
		t.Fatal("clipboard round trip failed")
	}
}

func TestReadFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		out  []byte
		err  error
	}{
		{"empty", []byte(" \n"), nil}, {"binary", []byte{0xff}, nil},
		{"missing", nil, exec.ErrNotFound}, {"unavailable", nil, errors.New("no selection")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			c := Clipboard{Run: func(context.Context, string, []string, io.Reader) ([]byte, error) { calls++; return tc.out, tc.err }}
			if s, err := c.Read(context.Background(), true); err == nil || s != "" || calls != 1 {
				t.Fatalf("failure: %q %v calls=%d", s, err, calls)
			}
		})
	}
}

func TestReadDeadlineAndCopy(t *testing.T) {
	c := Clipboard{Run: func(ctx context.Context, _ string, _ []string, _ io.Reader) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Read(ctx, true); err == nil {
		t.Fatal("canceled read succeeded")
	}
	ctx, cancel = context.WithTimeout(context.Background(), 0)
	defer cancel()
	if _, err := c.Read(ctx, true); err == nil {
		t.Fatal("expired read succeeded")
	}
	text := "中文\ntext; $(do-not-execute)"
	c.Run = func(_ context.Context, name string, args []string, input io.Reader) ([]byte, error) {
		b, _ := io.ReadAll(input)
		if name != "wl-copy" || string(b) != text || !strings.Contains(strings.Join(args, " "), "text/plain") {
			t.Fatal("bad copy command")
		}
		return nil, nil
	}
	if err := c.Copy(context.Background(), text); err != nil {
		t.Fatal(err)
	}
}
