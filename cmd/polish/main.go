package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/52funny/polish/internal/app"
)

var applicationName = "Polish"

func main() {
	selection := flag.Bool("selection", false, "Read the Wayland PRIMARY selection before opening the window")
	flag.Parse()
	if runtime.GOOS != "linux" || os.Getenv("WAYLAND_DISPLAY") == "" {
		fmt.Fprintln(os.Stderr, "Polish requires a Linux Wayland desktop session")
		os.Exit(1)
	}
	if err := app.Run(applicationName, *selection); err != nil {
		fmt.Fprintln(os.Stderr, "Polish could not start. Check the desktop runtime dependencies:", err)
		os.Exit(1)
	}
}
