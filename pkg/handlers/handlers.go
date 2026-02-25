package handlers

import (
        "embed"
        "encoding/json"
        "fmt"
        "io"
        "log"
        "net/http"
        "os"
        "path/filepath"
        "runtime"
        "strings"
        "sync"
        "text/template"

        "kube-browser/pkg/k8s"
)

func sanitizePath(p string) string {
        cleaned := filepath.Clean("/" + p)
        if strings.Contains(cleaned, "..") {
                return "/"
        }
        return cleaned
}

type Handler struct {
        mu        sync.RWMutex
        client    *k8s.Client
        static    embed.FS
        templates embed.FS
}

func New(static, templates embed.FS) *Handler {
        return &Handler{
                static:    static,
                templates: templates,
        }
}

func (h *Handler) getClient() *k8s.Client {
        h.mu.RLock()
        defer h.mu.RUnlock()
        return h.client
}

func (h *Handler) setClient(c *k8s.Client) {
        h.mu.Lock()
        defer h.mu.Unlock()
        h.client = c
}

func (h *Handler) jsonResponse(w http.ResponseWriter, data interface{}) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, code int) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(code)
        json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func (h *Handler) IndexHandler(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/" {
                http.NotFound(w, r)
                return
        }

        tmpl, err := template.ParseFS(h.templates, "templates/index.html")
        if err != nil {
                http.Error(w, "Failed to load template", http.StatusInternalServerError)
                log.Printf("Template error: %v", err)
                return
        }

        client := h.getClient()
        tmpl.Execute(w, map[string]interface{}{
                "Connected":      client != nil,
                "DefaultKubeconfig": k8s.DefaultKubeconfigPath(),
        })
}

func (h *Handler) StatusHandler(w http.ResponseWriter, r *http.Request) {
        client := h.getClient()
        connected := client != nil
        resp := map[string]interface{}{
                "connected": connected,
        }
        if connected {
                resp["kubeconfigPath"] = client.KubeconfigPath
                resp["context"] = client.ContextName
                resp["message"] = "Connected to Kubernetes cluster"
        } else {
                resp["message"] = "Not connected"
                resp["defaultKubeconfig"] = k8s.DefaultKubeconfigPath()
        }
        h.jsonResponse(w, resp)
}

func (h *Handler) LoadKubeconfigHandler(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
                h.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
                return
        }

        var req struct {
                Path string `json:"path"`
        }
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                h.jsonError(w, "Invalid request body", http.StatusBadRequest)
                return
        }

        if req.Path == "" {
                req.Path = k8s.DefaultKubeconfigPath()
        }

        info, err := k8s.ReadKubeconfig(req.Path)
        if err != nil {
                h.jsonError(w, fmt.Sprintf("Failed to load kubeconfig: %v", err), http.StatusBadRequest)
                return
        }

        h.jsonResponse(w, map[string]interface{}{
                "path":     req.Path,
                "contexts": info.Contexts,
                "current":  info.CurrentContext,
        })
}

func (h *Handler) ConnectHandler(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
                h.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
                return
        }

        var req struct {
                KubeconfigPath string `json:"kubeconfigPath"`
                Context        string `json:"context"`
        }
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
                h.jsonError(w, "Invalid request body", http.StatusBadRequest)
                return
        }

        client, err := k8s.NewClientWithContext(req.KubeconfigPath, req.Context)
        if err != nil {
                h.jsonError(w, fmt.Sprintf("Failed to connect: %v", err), http.StatusBadRequest)
                return
        }

        namespaces, err := client.ListNamespaces(r.Context())
        if err != nil {
                h.jsonError(w, fmt.Sprintf("Connected but failed to list namespaces: %v", err), http.StatusInternalServerError)
                return
        }

        h.setClient(client)

        h.jsonResponse(w, map[string]interface{}{
                "connected":  true,
                "context":    req.Context,
                "namespaces": namespaces,
                "message":    "Connected successfully",
        })
}

func (h *Handler) DisconnectHandler(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
                h.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
                return
        }

        h.setClient(nil)

        h.jsonResponse(w, map[string]interface{}{
                "connected": false,
                "message":   "Disconnected",
        })
}

func (h *Handler) ListNamespacesHandler(w http.ResponseWriter, r *http.Request) {
        client := h.getClient()
        if client == nil {
                h.jsonError(w, "Not connected to Kubernetes cluster", http.StatusServiceUnavailable)
                return
        }

        namespaces, err := client.ListNamespaces(r.Context())
        if err != nil {
                h.jsonError(w, fmt.Sprintf("Failed to list namespaces: %v", err), http.StatusInternalServerError)
                return
        }

        h.jsonResponse(w, map[string]interface{}{
                "namespaces": namespaces,
        })
}

func (h *Handler) ListPVCsHandler(w http.ResponseWriter, r *http.Request) {
        client := h.getClient()
        if client == nil {
                h.jsonError(w, "Not connected to Kubernetes cluster", http.StatusServiceUnavailable)
                return
        }

        namespace := r.URL.Query().Get("namespace")
        if namespace == "" {
                h.jsonError(w, "namespace parameter is required", http.StatusBadRequest)
                return
        }

        pvcs, err := client.ListPVCs(r.Context(), namespace)
        if err != nil {
                h.jsonError(w, fmt.Sprintf("Failed to list PVCs: %v", err), http.StatusInternalServerError)
                return
        }

        h.jsonResponse(w, map[string]interface{}{
                "pvcs": pvcs,
        })
}

func (h *Handler) ListFilesHandler(w http.ResponseWriter, r *http.Request) {
        client := h.getClient()
        if client == nil {
                h.jsonError(w, "Not connected to Kubernetes cluster", http.StatusServiceUnavailable)
                return
        }

        namespace := r.URL.Query().Get("namespace")
        pvc := r.URL.Query().Get("pvc")
        path := r.URL.Query().Get("path")

        if namespace == "" || pvc == "" {
                h.jsonError(w, "namespace and pvc parameters are required", http.StatusBadRequest)
                return
        }

        if path == "" {
                path = "/"
        }
        path = sanitizePath(path)

        files, err := client.ListFiles(r.Context(), namespace, pvc, path)
        if err != nil {
                h.jsonError(w, fmt.Sprintf("Failed to list files: %v", err), http.StatusInternalServerError)
                return
        }

        h.jsonResponse(w, map[string]interface{}{
                "files": files,
                "path":  path,
        })
}

func (h *Handler) DownloadFileHandler(w http.ResponseWriter, r *http.Request) {
        client := h.getClient()
        if client == nil {
                h.jsonError(w, "Not connected to Kubernetes cluster", http.StatusServiceUnavailable)
                return
        }

        namespace := r.URL.Query().Get("namespace")
        pvc := r.URL.Query().Get("pvc")
        filePath := r.URL.Query().Get("path")

        if namespace == "" || pvc == "" || filePath == "" {
                h.jsonError(w, "namespace, pvc, and path parameters are required", http.StatusBadRequest)
                return
        }

        filePath = sanitizePath(filePath)

        reader, fileName, err := client.DownloadFile(r.Context(), namespace, pvc, filePath)
        if err != nil {
                h.jsonError(w, fmt.Sprintf("Failed to download file: %v", err), http.StatusInternalServerError)
                return
        }

        w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
        w.Header().Set("Content-Type", "application/octet-stream")
        io.Copy(w, reader)
}

func (h *Handler) UploadFileHandler(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
                h.jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
                return
        }

        client := h.getClient()
        if client == nil {
                h.jsonError(w, "Not connected to Kubernetes cluster", http.StatusServiceUnavailable)
                return
        }

        err := r.ParseMultipartForm(100 << 20)
        if err != nil {
                h.jsonError(w, "Failed to parse upload", http.StatusBadRequest)
                return
        }

        namespace := r.FormValue("namespace")
        pvc := r.FormValue("pvc")
        destPath := r.FormValue("path")

        if namespace == "" || pvc == "" {
                h.jsonError(w, "namespace and pvc are required", http.StatusBadRequest)
                return
        }

        file, header, err := r.FormFile("file")
        if err != nil {
                h.jsonError(w, "No file provided", http.StatusBadRequest)
                return
        }
        defer file.Close()

        destPath = sanitizePath(destPath)
        if destPath == "" || destPath == "/" {
                destPath = "/" + filepath.Base(header.Filename)
        } else {
                destPath = destPath + "/" + filepath.Base(header.Filename)
        }

        err = client.UploadFile(r.Context(), namespace, pvc, destPath, file)
        if err != nil {
                h.jsonError(w, fmt.Sprintf("Failed to upload file: %v", err), http.StatusInternalServerError)
                return
        }

        h.jsonResponse(w, map[string]interface{}{
                "success":  true,
                "message":  fmt.Sprintf("File %s uploaded successfully", header.Filename),
                "filename": header.Filename,
        })
}

func (h *Handler) BrowseLocalHandler(w http.ResponseWriter, r *http.Request) {
        dirPath := r.URL.Query().Get("path")

        if dirPath == "" {
                home, _ := os.UserHomeDir()
                dirPath = home
        }

        dirPath = filepath.Clean(dirPath)

        info, err := os.Stat(dirPath)
        if err != nil {
                h.jsonError(w, fmt.Sprintf("Path not found: %s", dirPath), http.StatusBadRequest)
                return
        }
        if !info.IsDir() {
                dirPath = filepath.Dir(dirPath)
        }

        entries, err := os.ReadDir(dirPath)
        if err != nil {
                h.jsonError(w, fmt.Sprintf("Cannot read directory: %v", err), http.StatusInternalServerError)
                return
        }

        type localEntry struct {
                Name  string `json:"name"`
                IsDir bool   `json:"isDir"`
                Path  string `json:"path"`
        }

        var items []localEntry

        parent := filepath.Dir(dirPath)
        if parent != dirPath {
                items = append(items, localEntry{
                        Name:  "..",
                        IsDir: true,
                        Path:  parent,
                })
        }

        for _, entry := range entries {
                if strings.HasPrefix(entry.Name(), ".") && entry.Name() != ".kube" {
                        continue
                }
                fullPath := filepath.Join(dirPath, entry.Name())
                items = append(items, localEntry{
                        Name:  entry.Name(),
                        IsDir: entry.IsDir(),
                        Path:  fullPath,
                })
        }

        var drives []string
        if runtime.GOOS == "windows" {
                for _, d := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
                        dp := string(d) + ":\\"
                        if _, err := os.Stat(dp); err == nil {
                                drives = append(drives, dp)
                        }
                }
        }

        h.jsonResponse(w, map[string]interface{}{
                "currentPath": dirPath,
                "entries":     items,
                "drives":      drives,
        })
}
