package handlers

import "testing"

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
