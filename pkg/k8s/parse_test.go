package k8s

import (
	"testing"
)

func TestParseGNUlsOutput(t *testing.T) {
	tests := []struct {
		name     string
		stdout   string
		path     string
		wantLen  int
		wantFile FileInfo
	}{
		{
			name: "regular file and directory",
			stdout: `total 8
-rw-r--r-- 1 root root  123 2024-01-15 10:30 myfile.txt
drwxr-xr-x 2 root root 4096 2024-01-15 10:30 mydir`,
			path:    "/data",
			wantLen: 2,
			wantFile: FileInfo{
				Name:    "myfile.txt",
				Size:    "123",
				ModTime: "2024-01-15 10:30",
				IsDir:   false,
				Path:    "/data/myfile.txt",
			},
		},
		{
			name: "file with spaces in name",
			stdout: `total 4
-rw-r--r-- 1 root root 42 2024-01-15 10:30 my file with spaces.txt`,
			path:    "/",
			wantLen: 1,
			wantFile: FileInfo{
				Name:    "my file with spaces.txt",
				Size:    "42",
				ModTime: "2024-01-15 10:30",
				IsDir:   false,
				Path:    "my file with spaces.txt",
			},
		},
		{
			name: "root path yields flat filePath",
			stdout: `total 4
drwxr-xr-x 2 root root 4096 2024-01-15 10:30 subdir`,
			path:    "",
			wantLen: 1,
			wantFile: FileInfo{
				Name:  "subdir",
				IsDir: true,
				Path:  "subdir",
			},
		},
		{
			name:    "empty output",
			stdout:  "",
			path:    "/data",
			wantLen: 0,
		},
		{
			name: "dot and dotdot are skipped",
			stdout: `total 8
drwxr-xr-x 2 root root 4096 2024-01-15 10:30 .
drwxr-xr-x 8 root root 4096 2024-01-15 10:30 ..
-rw-r--r-- 1 root root   10 2024-01-15 10:30 actual.txt`,
			path:    "/data",
			wantLen: 1,
		},
		{
			name: "malformed lines skipped",
			stdout: `total 4
this is a malformed line
-rw-r--r-- 1 root root 10 2024-01-15 10:30 ok.txt`,
			path:    "/data",
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGNUlsOutput(tt.stdout, tt.path)
			if len(got) != tt.wantLen {
				t.Errorf("got %d files, want %d", len(got), tt.wantLen)
			}
			if tt.wantLen > 0 && (tt.wantFile.Name != "" || tt.wantFile.Path != "") {
				f := got[0]
				if tt.wantFile.Name != "" && f.Name != tt.wantFile.Name {
					t.Errorf("Name: got %q, want %q", f.Name, tt.wantFile.Name)
				}
				if tt.wantFile.Size != "" && f.Size != tt.wantFile.Size {
					t.Errorf("Size: got %q, want %q", f.Size, tt.wantFile.Size)
				}
				if tt.wantFile.Path != "" && f.Path != tt.wantFile.Path {
					t.Errorf("Path: got %q, want %q", f.Path, tt.wantFile.Path)
				}
				if tt.wantFile.IsDir != f.IsDir {
					t.Errorf("IsDir: got %v, want %v", f.IsDir, tt.wantFile.IsDir)
				}
			}
		})
	}
}

func TestParseBusyboxOutput(t *testing.T) {
	tests := []struct {
		name     string
		stdout   string
		path     string
		wantLen  int
		wantFile *FileInfo
	}{
		{
			name: "standard busybox ls output (8 fields)",
			stdout: `total 8
-rw-r--r--    1 root     root           42 Jan 15 10:30 notes.txt
drwxr-xr-x    2 root     root         4096 Jan 15 10:30 configs`,
			path:    "/data",
			wantLen: 2,
			wantFile: &FileInfo{
				Name:  "notes.txt",
				Size:  "42",
				IsDir: false,
				Path:  "/data/notes.txt",
			},
		},
		{
			name: "file with spaces in name",
			stdout: `total 4
-rw-r--r--    1 root     root           10 Jan 15 10:30 my data file.csv`,
			path:    "/",
			wantLen: 1,
			wantFile: &FileInfo{
				Name:  "my data file.csv",
				Path:  "my data file.csv",
				IsDir: false,
			},
		},
		{
			name:    "empty",
			stdout:  "",
			path:    "/data",
			wantLen: 0,
		},
		{
			name: "dot entries skipped",
			stdout: `total 4
drwxr-xr-x    2 root     root         4096 Jan 15 10:30 .
drwxr-xr-x    8 root     root         4096 Jan 15 10:30 ..
-rw-r--r--    1 root     root           10 Jan 15 10:30 real.txt`,
			path:    "/data",
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseBusyboxOutput(tt.stdout, tt.path)
			if len(got) != tt.wantLen {
				t.Errorf("got %d files, want %d", len(got), tt.wantLen)
			}
			if tt.wantFile != nil && len(got) > 0 {
				f := got[0]
				if tt.wantFile.Name != "" && f.Name != tt.wantFile.Name {
					t.Errorf("Name: got %q, want %q", f.Name, tt.wantFile.Name)
				}
				if tt.wantFile.Size != "" && f.Size != tt.wantFile.Size {
					t.Errorf("Size: got %q, want %q", f.Size, tt.wantFile.Size)
				}
				if tt.wantFile.Path != "" && f.Path != tt.wantFile.Path {
					t.Errorf("Path: got %q, want %q", f.Path, tt.wantFile.Path)
				}
				if tt.wantFile.IsDir != f.IsDir {
					t.Errorf("IsDir: got %v, want %v", f.IsDir, tt.wantFile.IsDir)
				}
			}
		})
	}
}

func TestParseFindOutput(t *testing.T) {
	fullPath := "/data"

	tests := []struct {
		name     string
		stdout   string
		path     string
		wantLen  int
		wantFile *FileInfo
	}{
		{
			name:   "stat pipe format",
			stdout: "/data/myfile.txt|1234|1705310400|regular file\n/data/subdir|0|1705310400|directory\n",
			path:   "/data",
			wantLen: 2,
			wantFile: &FileInfo{
				Name:  "myfile.txt",
				Size:  "1234",
				IsDir: false,
				Path:  "/data/myfile.txt",
			},
		},
		{
			name:   "plain find output (no stat)",
			stdout: "/data/a.txt\n/data/b.txt\n",
			path:   "/data",
			wantLen: 2,
			wantFile: &FileInfo{
				Name:  "a.txt",
				Size:  "0",
				IsDir: false,
				Path:  "/data/a.txt",
			},
		},
		{
			name:    "empty output",
			stdout:  "",
			path:    "/data",
			wantLen: 0,
		},
		{
			name:   "directory entry",
			stdout: "/data/mydir|0|1705310400|directory\n",
			path:   "/",
			wantLen: 1,
			wantFile: &FileInfo{
				Name:  "mydir",
				IsDir: true,
				Path:  "mydir",
			},
		},
		{
			name:   "root path produces flat filePath",
			stdout: "/data/file.txt|100|1705310400|regular file\n",
			path:   "",
			wantLen: 1,
			wantFile: &FileInfo{
				Name: "file.txt",
				Path: "file.txt",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFindOutput(tt.stdout, fullPath, tt.path)
			if len(got) != tt.wantLen {
				t.Errorf("got %d files, want %d; entries: %v", len(got), tt.wantLen, got)
			}
			if tt.wantFile != nil && len(got) > 0 {
				f := got[0]
				if tt.wantFile.Name != "" && f.Name != tt.wantFile.Name {
					t.Errorf("Name: got %q, want %q", f.Name, tt.wantFile.Name)
				}
				if tt.wantFile.Size != "" && f.Size != tt.wantFile.Size {
					t.Errorf("Size: got %q, want %q", f.Size, tt.wantFile.Size)
				}
				if tt.wantFile.Path != "" && f.Path != tt.wantFile.Path {
					t.Errorf("Path: got %q, want %q", f.Path, tt.wantFile.Path)
				}
				if tt.wantFile.IsDir != f.IsDir {
					t.Errorf("IsDir: got %v, want %v", f.IsDir, tt.wantFile.IsDir)
				}
			}
		})
	}
}
