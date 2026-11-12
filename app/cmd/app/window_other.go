//go:build !windows

package main

// App mode is Windows-only (see main.go); elsewhere the UI opens in the
// default browser and errors go to the log.

func openAppWindow(url string) error {
	openBrowser(url)
	return nil
}

func showMessage(title, text string, isError bool) {}
