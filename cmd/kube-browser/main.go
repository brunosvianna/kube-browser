package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"

	"kube-browser/pkg/handlers"
)

//go:embed all:static
var staticFiles embed.FS

//go:embed all:templates
var templateFiles embed.FS

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	h := handlers.New(staticFiles, templateFiles)

	mux := http.NewServeMux()

	mux.HandleFunc("/", h.IndexHandler)
	mux.HandleFunc("/api/status", h.StatusHandler)
	mux.HandleFunc("/api/kubeconfig", h.LoadKubeconfigHandler)
	mux.HandleFunc("/api/connect", h.ConnectHandler)
	mux.HandleFunc("/api/disconnect", h.DisconnectHandler)
	mux.HandleFunc("/api/namespaces", h.ListNamespacesHandler)
	mux.HandleFunc("/api/pvcs", h.ListPVCsHandler)
	mux.HandleFunc("/api/files", h.ListFilesHandler)
	mux.HandleFunc("/api/download", h.DownloadFileHandler)
	mux.HandleFunc("/api/upload", h.UploadFileHandler)
	mux.Handle("/static/", http.FileServer(http.FS(staticFiles)))

	fmt.Printf("KubeBrowser started on http://0.0.0.0:%s\n", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, mux))
}
