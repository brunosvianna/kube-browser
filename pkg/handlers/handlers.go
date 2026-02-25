package handlers

import (
        "embed"
        "encoding/json"
        "fmt"
        "io"
        "log"
        "net/http"
        "path/filepath"
        "strings"
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
        client    *k8s.Client
        static    embed.FS
        templates embed.FS
}

func New(client *k8s.Client, static, templates embed.FS) *Handler {
        return &Handler{
                client:    client,
                static:    static,
                templates: templates,
        }
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

        connected := h.client != nil
        tmpl.Execute(w, map[string]interface{}{
                "Connected": connected,
        })
}

func (h *Handler) StatusHandler(w http.ResponseWriter, r *http.Request) {
        connected := h.client != nil
        h.jsonResponse(w, map[string]interface{}{
                "connected": connected,
                "message": func() string {
                        if connected {
                                return "Connected to Kubernetes cluster"
                        }
                        return "Not connected. Set KUBECONFIG environment variable."
                }(),
        })
}

func (h *Handler) ListNamespacesHandler(w http.ResponseWriter, r *http.Request) {
        if h.client == nil {
                h.jsonError(w, "Not connected to Kubernetes cluster", http.StatusServiceUnavailable)
                return
        }

        namespaces, err := h.client.ListNamespaces(r.Context())
        if err != nil {
                h.jsonError(w, fmt.Sprintf("Failed to list namespaces: %v", err), http.StatusInternalServerError)
                return
        }

        h.jsonResponse(w, map[string]interface{}{
                "namespaces": namespaces,
        })
}

func (h *Handler) ListPVCsHandler(w http.ResponseWriter, r *http.Request) {
        if h.client == nil {
                h.jsonError(w, "Not connected to Kubernetes cluster", http.StatusServiceUnavailable)
                return
        }

        namespace := r.URL.Query().Get("namespace")
        if namespace == "" {
                h.jsonError(w, "namespace parameter is required", http.StatusBadRequest)
                return
        }

        pvcs, err := h.client.ListPVCs(r.Context(), namespace)
        if err != nil {
                h.jsonError(w, fmt.Sprintf("Failed to list PVCs: %v", err), http.StatusInternalServerError)
                return
        }

        h.jsonResponse(w, map[string]interface{}{
                "pvcs": pvcs,
        })
}

func (h *Handler) ListFilesHandler(w http.ResponseWriter, r *http.Request) {
        if h.client == nil {
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

        files, err := h.client.ListFiles(r.Context(), namespace, pvc, path)
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
        if h.client == nil {
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

        reader, fileName, err := h.client.DownloadFile(r.Context(), namespace, pvc, filePath)
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

        if h.client == nil {
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

        err = h.client.UploadFile(r.Context(), namespace, pvc, destPath, file)
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
