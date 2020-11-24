package goui

import "os/exec"

// LaunchBrowser opens the URL in the default browser.
func LaunchBrowser(url string) *exec.Cmd {
	return exec.Command("xdg-open", url)
}
