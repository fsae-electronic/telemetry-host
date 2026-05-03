package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"telemetry-host/internal/api"
	"telemetry-host/internal/hub"
	"telemetry-host/internal/serial"
	"telemetry-host/internal/store"
)

//go:embed all:frontend
var frontendFS embed.FS

func main() {
	port := flag.Int("port", 8080, "HTTP server port")
	dbPath := flag.String("db", "telemetry.db", "SQLite database path")
	noOpen := flag.Bool("no-open", false, "Don't auto-open the browser")
	flag.Parse()

	st, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}

	sm := serial.NewManager()
	h := hub.New()
	srv := api.New(sm, h, st)

	go srv.ConsumePackets()

	mux := http.NewServeMux()
	srv.Routes(mux)

	// Serve the embedded frontend
	sub, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		log.Fatalf("embed sub: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	addr := fmt.Sprintf(":%d", *port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen %s: %v", addr, err)
	}

	url := fmt.Sprintf("http://localhost:%d", *port)
	log.Printf("telemetry host running at %s", url)

	if !*noOpen {
		go func() {
			time.Sleep(300 * time.Millisecond)
			openBrowser(url)
		}()
	}

	log.Fatal(http.Serve(ln, mux))
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
