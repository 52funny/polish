package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/52funny/polish/internal/app"
	"github.com/52funny/polish/internal/cli"
)

var applicationName = "Polish"

func main() {
	options, err := cli.Parse(os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		fmt.Print(cli.Usage)
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprint(os.Stderr, cli.Usage)
		os.Exit(2)
	}
	if runtime.GOOS != "linux" || os.Getenv("WAYLAND_DISPLAY") == "" {
		fmt.Fprintln(os.Stderr, "Polish requires a Linux Wayland desktop session")
		os.Exit(1)
	}
	if err := app.Run(applicationName, options); err != nil {
		fmt.Fprintln(os.Stderr, "Polish could not start. Check the desktop runtime dependencies:", err)
		os.Exit(1)
	}
}
