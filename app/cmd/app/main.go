package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"telemetry-host/internal/api"
	"telemetry-host/internal/hub"
	"telemetry-host/internal/serial"
	"telemetry-host/internal/store"
)

//go:embed all:frontend
var frontendFS embed.FS

const appName = "FSAE Telemetry"

// appMode: on Windows the UI opens in its own window (see window_windows.go)
// and the program exits when that window is closed. Elsewhere, or with
// --no-open, it is a plain server that runs until killed.
var appMode bool

func main() {
	port := flag.Int("port", 8080, "HTTP server port")
	dbPath := flag.String("db", "telemetry.db", "SQLite database path")
	noOpen := flag.Bool("no-open", false, "Don't open a window or browser; run as a server only")
	flag.Parse()

	appMode = !*noOpen && runtime.GOOS == "windows"

	if appMode {
		// Started from a shortcut or by double-click: keep the data next to
		// the .exe instead of in whatever the working directory happens to be.
		if !flagSet("db") {
			*dbPath = filepath.Join(exeDir(), "telemetry.db")
		}
	}

	url := fmt.Sprintf("http://localhost:%d", *port)

	// The app only serves this machine, so listening on loopback also avoids
	// the Windows Firewall prompt. As a server (Docker) it listens on all interfaces.
	addr := fmt.Sprintf(":%d", *port)
	if appMode {
		addr = fmt.Sprintf("127.0.0.1:%d", *port)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if appMode && alreadyRunning(url) {
			// Opened a second time: show another window of the running app.
			openWindow(url)
			return
		}
		fatal("Could not listen on port %d (is another program using it?): %v", *port, err)
	}

	// The Windows build has no console, so log to a file next to the database.
	// Created only now so a second instance does not truncate the first one's log.
	if runtime.GOOS == "windows" {
		if f, err := os.Create(filepath.Join(filepath.Dir(*dbPath), "telemetry-host.log")); err == nil {
			log.SetOutput(f)
		}
	}

	st, err := store.Open(*dbPath)
	if err != nil {
		fatal("Could not open the database %s: %v", *dbPath, err)
	}

	sm := serial.NewManager()
	h := hub.New()
	srv := api.New(sm, h, st)

	go srv.ConsumePackets()
	go srv.BroadcastLinkStats()

	mux := http.NewServeMux()
	srv.Routes(mux)

	// Serve the embedded frontend
	sub, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		fatal("embed sub: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	log.Printf("telemetry host running at %s (db %s)", url, *dbPath)
	go func() {
		fatal("HTTP server stopped: %v", http.Serve(ln, mux))
	}()

	switch {
	case appMode:
		openWindow(url)
		waitForWindowsClosed(h, url)
		log.Printf("window closed, shutting down")
		srv.Shutdown()
	case !*noOpen:
		openBrowser(url)
		select {}
	default:
		select {}
	}
}

// openWindow opens the UI in an app window, or in the default browser if no
// supported browser is installed.
func openWindow(url string) {
	if err := openAppWindow(url); err != nil {
		log.Printf("app window: %v; opening the default browser", err)
		openBrowser(url)
	}
}

// waitForWindowsClosed returns once every UI window has been closed. Each
// open window keeps a WebSocket to the hub; a page reload reconnects within a
// second or two, well inside closeGrace.
func waitForWindowsClosed(h *hub.Hub, url string) {
	const (
		closeGrace   = 4 * time.Second
		startTimeout = 60 * time.Second
	)
	start := time.Now()
	seen := false
	var emptySince time.Time

	for now := range time.Tick(500 * time.Millisecond) {
		switch {
		case h.Clients() > 0:
			seen = true
			emptySince = time.Time{}
		case !seen && now.Sub(start) > startTimeout:
			// The window never connected. Keep serving until the user says stop.
			showMessage(appName, fmt.Sprintf(
				"%s is running at %s but its window did not open.\n\n"+
					"Open that address in a browser to use it. Press OK to quit.", appName, url), false)
			return
		case !seen:
		case emptySince.IsZero():
			emptySince = now
		case now.Sub(emptySince) > closeGrace:
			return
		}
	}
}

func alreadyRunning(url string) bool {
	c := http.Client{Timeout: time.Second}
	resp, err := c.Get(url + "/api/status")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// fatal logs the error and, in app mode where there is no console, also
// shows it in a message box before exiting.
func fatal(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Print(msg)
	if appMode {
		showMessage(appName, msg, true)
	}
	os.Exit(1)
}

func flagSet(name string) bool {
	set := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}
