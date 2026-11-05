package desktop

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

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
