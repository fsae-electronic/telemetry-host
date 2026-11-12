package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// openAppWindow opens url in a chromeless Edge window ("app mode": no tabs or
// address bar), falling back to Chrome. The dedicated profile directory makes
// it a separate browser instance, so it never ends up as a tab of a browser
// the user already has open, and keeps its own window size and zoom.
func openAppWindow(url string) error {
	browser := findBrowser()
	if browser == "" {
		return errors.New("neither Microsoft Edge nor Google Chrome found")
	}
	profile := filepath.Join(os.Getenv("LOCALAPPDATA"), appName, "window")
	return exec.Command(browser,
		"--app="+url,
		"--user-data-dir="+profile,
		"--window-size=1440,900",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-extensions",
		"--disable-sync",
	).Start()
}

func findBrowser() string {
	bases := []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles"), os.Getenv("LOCALAPPDATA")}
	for _, rel := range []string{`Microsoft\Edge\Application\msedge.exe`, `Google\Chrome\Application\chrome.exe`} {
		for _, base := range bases {
			if base == "" {
				continue
			}
			p := filepath.Join(base, rel)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

// showMessage shows a modal message box and returns when the user closes it.
func showMessage(title, text string, isError bool) {
	icon := uint32(windows.MB_ICONINFORMATION)
	if isError {
		icon = windows.MB_ICONERROR
	}
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(text)
	windows.MessageBox(0, m, t, windows.MB_OK|icon|windows.MB_SETFOREGROUND)
}
