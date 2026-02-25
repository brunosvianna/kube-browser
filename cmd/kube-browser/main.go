package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"

	"kube-browser/pkg/handlers"
	"kube-browser/pkg/k8s"
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

	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		home, _ := os.UserHomeDir()
		kubeconfigPath = home + "/.kube/config"
	}

	client, err := k8s.NewClient(kubeconfigPath)
	if err != nil {
		log.Printf("WARNING: Could not connect to Kubernetes cluster: %v", err)
		log.Printf("The application will start in demo mode. Set KUBECONFIG to connect to a real cluster.")
		client = nil
	}

	h := handlers.New(client, staticFiles, templateFiles)

	mux := http.NewServeMux()

	mux.HandleFunc("/", h.IndexHandler)
	mux.HandleFunc("/api/namespaces", h.ListNamespacesHandler)
	mux.HandleFunc("/api/pvcs", h.ListPVCsHandler)
	mux.HandleFunc("/api/files", h.ListFilesHandler)
	mux.HandleFunc("/api/download", h.DownloadFileHandler)
	mux.HandleFunc("/api/upload", h.UploadFileHandler)
	mux.HandleFunc("/api/status", h.StatusHandler)
	mux.Handle("/static/", http.FileServer(http.FS(staticFiles)))

	fmt.Printf("KubeBrowser started on http://0.0.0.0:%s\n", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, mux))
}
