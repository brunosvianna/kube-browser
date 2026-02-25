# KubeBrowser - PVC File Manager

## Overview
KubeBrowser is a Go application that provides a web-based IDE interface for managing files in Kubernetes Persistent Volume Claims (PVCs). It connects to a Kubernetes cluster via kubeconfig and allows browsing, downloading, and uploading files stored in PVCs.

## Architecture
- **Language**: Go 1.25
- **Frontend**: HTML/CSS/JS embedded in the Go binary via `go:embed`
- **Kubernetes**: Uses `client-go` library for cluster communication
- **File Operations**: Executes commands inside pods (via `kubectl exec` equivalent) to browse/download/upload files

## Project Structure
```
kube-browser/
├── cmd/kube-browser/        # Main entry point + embedded assets
│   ├── main.go              # HTTP server, routing, embed directives
│   ├── static/              # CSS and JS files (embedded)
│   │   ├── css/style.css
│   │   └── js/app.js
│   └── templates/           # HTML templates (embedded)
│       └── index.html
├── pkg/
│   ├── k8s/                 # Kubernetes client logic
│   │   └── client.go        # PVC listing, file operations, pod exec
│   └── handlers/            # HTTP request handlers
│       └── handlers.go      # API endpoints for namespaces, PVCs, files
├── go.mod
├── go.sum
└── .gitignore
```

## How It Works
1. Binary reads `KUBECONFIG` env var (or `~/.kube/config` by default)
2. Connects to Kubernetes cluster using `client-go`
3. Lists namespaces and PVCs in the sidebar
4. For file operations, finds the running pod that mounts the selected PVC
5. Executes `ls`, `cat`, and `tee` commands inside the pod to list/download/upload files

## API Endpoints
- `GET /api/status` - Connection status
- `GET /api/namespaces` - List namespaces
- `GET /api/pvcs?namespace=X` - List PVCs in namespace
- `GET /api/files?namespace=X&pvc=Y&path=Z` - List files in PVC
- `GET /api/download?namespace=X&pvc=Y&path=Z` - Download a file
- `POST /api/upload` - Upload a file (multipart form)

## Building
```bash
CGO_ENABLED=0 go build -o kube-browser ./cmd/kube-browser/
```

## Running
```bash
export KUBECONFIG=/path/to/your/kubeconfig
./kube-browser
```
The server starts on port 5000 by default (configurable via PORT env var).

## Dependencies
- `k8s.io/client-go` v0.31.0 - Kubernetes Go client
- `k8s.io/api` v0.31.0 - Kubernetes API types
- `k8s.io/apimachinery` v0.31.0 - Kubernetes API machinery
