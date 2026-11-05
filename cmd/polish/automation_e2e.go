//go:build e2e

package main

// Only opt-in test builds expose MyGo's local automation socket.
import _ "github.com/egoist/mygo/plugins/automation"

// Keep smoke tests separate from the user's running instance.
func init() { applicationName = "Polish E2E" }
