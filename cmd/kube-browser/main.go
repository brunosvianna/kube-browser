package main

import (
        "context"
        "embed"
        "fmt"
        "log"
        "net/http"
        "os"
        "os/signal"
        "strconv"
        "syscall"
        "time"

        "kube-browser/pkg/browser"
        "kube-browser/pkg/handlers"
)

//go:embed all:static
var staticFiles embed.FS

//go:embed all:templates
var templateFiles embed.FS

func envDuration(key string, defaultSec int) time.Duration {
        if v := os.Getenv(key); v != "" {
                if n, err := strconv.Atoi(v); err == nil && n > 0 {
                        return time.Duration(n) * time.Second
                }
        }
        return time.Duration(defaultSec) * time.Second
}

func main() {
        port := os.Getenv("PORT")
        if port == "" {
                port = "5000"
        }

        host := os.Getenv("HOST")
        if host == "" {
                host = "127.0.0.1"
        }

        readTimeout := envDuration("READ_TIMEOUT", 15)
        writeTimeout := envDuration("WRITE_TIMEOUT", 60)
        idleTimeout := envDuration("IDLE_TIMEOUT", 120)
        shutdownTimeout := envDuration("SHUTDOWN_TIMEOUT", 10)

        h := handlers.New(staticFiles, templateFiles)

        mux := http.NewServeMux()

        mux.HandleFunc("/", h.IndexHandler)
        mux.HandleFunc("/api/status", h.StatusHandler)
        mux.HandleFunc("/api/kubeconfig", h.LoadKubeconfigHandler)
        mux.HandleFunc("/api/kubeconfig-upload", h.LoadKubeconfigUploadHandler)
        mux.HandleFunc("/api/connect", h.ConnectHandler)
        mux.HandleFunc("/api/disconnect", h.DisconnectHandler)
        mux.HandleFunc("/api/namespaces", h.ListNamespacesHandler)
        mux.HandleFunc("/api/pvcs", h.ListPVCsHandler)
        mux.HandleFunc("/api/files", h.ListFilesHandler)
        mux.HandleFunc("/api/download", h.DownloadFileHandler)
        mux.HandleFunc("/api/upload", h.UploadFileHandler)
        mux.Handle("/api/browse", h.LocalhostOnly(http.HandlerFunc(h.BrowseLocalHandler)))
        mux.Handle("/static/", http.FileServer(http.FS(staticFiles)))

        addr := host + ":" + port
        srv := &http.Server{
                Addr:         addr,
                Handler:      mux,
                ReadTimeout:  readTimeout,
                WriteTimeout: writeTimeout,
                IdleTimeout:  idleTimeout,
        }

        url := fmt.Sprintf("http://localhost:%s", port)
        fmt.Printf("KubeBrowser started on %s\n", url)

        go func() {
                time.Sleep(500 * time.Millisecond)
                if err := browser.Open(url); err != nil {
                        log.Printf("Could not open browser automatically: %v", err)
                        fmt.Printf("Open %s in your browser\n", url)
                }
        }()

        quit := make(chan os.Signal, 1)
        signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

        go func() {
                if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
                        log.Fatalf("Server error: %v", err)
                }
        }()

        <-quit
        log.Println("Shutting down...")

        ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
        defer cancel()
        if err := srv.Shutdown(ctx); err != nil {
                log.Printf("Forced shutdown: %v", err)
        } else {
                log.Println("Server stopped cleanly")
        }
}
