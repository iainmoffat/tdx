package auth

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openBrowser is a package-level function variable so tests can override
// it without launching a real browser. The default implementation invokes
// the OS-native open command and is best-effort: callers should treat a
// non-nil error as "browser did not launch, fall back to printing the URL"
// rather than as a fatal error.
//
// Implementation notes:
//   - macOS uses /usr/bin/open
//   - Windows uses rundll32 url.dll,FileProtocolHandler
//   - Other (linux/freebsd/etc.) uses xdg-open
//
// We Start() rather than Run() so the call returns immediately without
// blocking on the browser process. The browser's exit is irrelevant to
// the CLI's flow.
var openBrowser = func(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch browser via %s: %w", cmd.Path, err)
	}
	return nil
}
