package handlers

import (
        "embed"
        "encoding/json"
        "net/http"
        "net/http/httptest"
        "os"
        "strings"
        "testing"
)

func TestSanitizePath(t *testing.T) {
        tests := []struct {
                name  string
                input string
                want  string
        }{
                {
                        name:  "normal absolute path",
                        input: "/data/mydir",
                        want:  "/data/mydir",
                },
                {
                        name:  "normal nested path",
                        input: "/data/sub/nested",
                        want:  "/data/sub/nested",
                },
                {
                        name:  "root path",
                        input: "/",
                        want:  "/",
                },
                {
                        name:  "empty string becomes root",
                        input: "",
                        want:  "/",
                },
                {
                        name:  "path traversal with dotdot resolves relative to root",
                        input: "../../etc/passwd",
                        want:  "/etc/passwd",
                },
                {
                        name:  "dotdot inside path resolves cleanly",
                        input: "/data/../secret",
                        want:  "/secret",
                },
                {
                        name:  "Windows backslash converted",
                        input: "data\\subdir\\file.txt",
                        want:  "/data/subdir/file.txt",
                },
                {
                        name:  "mixed slashes",
                        input: "/data\\subdir/file.txt",
                        want:  "/data/subdir/file.txt",
                },
                {
                        name:  "relative path becomes absolute",
                        input: "data/mydir",
                        want:  "/data/mydir",
                },
                {
                        name:  "trailing slash is cleaned",
                        input: "/data/mydir/",
                        want:  "/data/mydir",
                },
        }

        for _, tt := range tests {
                t.Run(tt.name, func(t *testing.T) {
                        got := sanitizePath(tt.input)
                        if got != tt.want {
                                t.Errorf("sanitizePath(%q) = %q, want %q", tt.input, got, tt.want)
                        }
                })
        }
}

func TestReadOnlyModeUploadBlocked(t *testing.T) {
        t.Setenv("KUBE_BROWSER_READ_ONLY", "true")
        h := &Handler{readOnly: true}

        body := strings.NewReader("--boundary\r\nContent-Disposition: form-data; name=\"namespace\"\r\n\r\ndefault\r\n--boundary--\r\n")
        req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
        req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
        rr := httptest.NewRecorder()

        h.UploadFileHandler(rr, req)

        if rr.Code != http.StatusMethodNotAllowed {
                t.Errorf("expected 405, got %d", rr.Code)
        }

        var resp map[string]string
        if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
                t.Fatalf("failed to decode response: %v", err)
        }
        if resp["error"] != "read-only mode: write operations are disabled" {
                t.Errorf("unexpected error message: %q", resp["error"])
        }
}

func TestReadOnlyModeUploadAllowed(t *testing.T) {
        h := &Handler{readOnly: false}

        body := strings.NewReader("")
        req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
        req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
        rr := httptest.NewRecorder()

        h.UploadFileHandler(rr, req)

        if rr.Code == http.StatusMethodNotAllowed {
                var resp map[string]string
                json.NewDecoder(rr.Body).Decode(&resp)
                if resp["error"] == "read-only mode: write operations are disabled" {
                        t.Error("upload was blocked by read-only mode but readOnly is false")
                }
        }

        if rr.Code == http.StatusServiceUnavailable {
                var resp map[string]string
                json.NewDecoder(rr.Body).Decode(&resp)
                if resp["error"] == "read-only mode: write operations are disabled" {
                        t.Error("read-only error returned with unexpected status code")
                }
        }

        var resp map[string]string
        json.NewDecoder(rr.Body).Decode(&resp)
        if resp["error"] == "read-only mode: write operations are disabled" {
                t.Errorf("read-only guard fired with readOnly=false, got: %q", resp["error"])
        }
}

func TestParseReadOnlyEnv(t *testing.T) {
        tests := []struct {
                name     string
                envVal   string
                expected bool
        }{
                {"true string enables read-only", "true", true},
                {"1 enables read-only", "1", true},
                {"false disables read-only", "false", false},
                {"empty disables read-only", "", false},
                {"random value disables read-only", "yes", false},
        }

        for _, tt := range tests {
                t.Run(tt.name, func(t *testing.T) {
                        os.Setenv("KUBE_BROWSER_READ_ONLY", tt.envVal)
                        defer os.Unsetenv("KUBE_BROWSER_READ_ONLY")
                        got := parseReadOnlyEnv()
                        if got != tt.expected {
                                t.Errorf("parseReadOnlyEnv() with KUBE_BROWSER_READ_ONLY=%q = %v, want %v", tt.envVal, got, tt.expected)
                        }
                })
        }
}

func TestReadOnlyEnvVarParsing(t *testing.T) {
        tests := []struct {
                name     string
                envVal   string
                expected bool
        }{
                {"true string enables read-only", "true", true},
                {"1 enables read-only", "1", true},
                {"false disables read-only", "false", false},
                {"empty disables read-only", "", false},
                {"random value disables read-only", "yes", false},
        }

        for _, tt := range tests {
                t.Run(tt.name, func(t *testing.T) {
                        os.Setenv("KUBE_BROWSER_READ_ONLY", tt.envVal)
                        defer os.Unsetenv("KUBE_BROWSER_READ_ONLY")

                        var fs1, fs2 embed.FS
                        h := New(fs1, fs2)
                        if h.readOnly != tt.expected {
                                t.Errorf("KUBE_BROWSER_READ_ONLY=%q: readOnly=%v, want %v", tt.envVal, h.readOnly, tt.expected)
                        }
                })
        }
}

func TestStatusHandlerIncludesReadOnly(t *testing.T) {
        h := &Handler{readOnly: true}

        req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
        rr := httptest.NewRecorder()
        h.StatusHandler(rr, req)

        if rr.Code != http.StatusOK {
                t.Fatalf("expected 200, got %d", rr.Code)
        }

        var resp map[string]interface{}
        if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
                t.Fatalf("failed to decode response: %v", err)
        }

        readOnly, ok := resp["readOnly"]
        if !ok {
                t.Error("expected 'readOnly' field in status response")
        }
        if readOnly != true {
                t.Errorf("expected readOnly=true, got %v", readOnly)
        }
}

func TestStatusHandlerReadOnlyFalseByDefault(t *testing.T) {
        h := &Handler{readOnly: false}

        req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
        rr := httptest.NewRecorder()
        h.StatusHandler(rr, req)

        var resp map[string]interface{}
        json.NewDecoder(rr.Body).Decode(&resp)

        if resp["readOnly"] != false {
                t.Errorf("expected readOnly=false, got %v", resp["readOnly"])
        }
}
