package k8s

import "testing"

func TestBuildFilePath(t *testing.T) {
	tests := []struct {
		parent string
		name   string
		want   string
	}{
		{parent: "/data", name: "file.txt", want: "/data/file.txt"},
		{parent: "/data/sub", name: "nested.txt", want: "/data/sub/nested.txt"},
		{parent: "/", name: "root-file.txt", want: "root-file.txt"},
		{parent: "", name: "top-file.txt", want: "top-file.txt"},
		{parent: "/data", name: "file with spaces.txt", want: "/data/file with spaces.txt"},
	}

	for _, tt := range tests {
		got := buildFilePath(tt.parent, tt.name)
		if got != tt.want {
			t.Errorf("buildFilePath(%q, %q) = %q, want %q", tt.parent, tt.name, got, tt.want)
		}
	}
}

func TestFullPathConstruction(t *testing.T) {
	tests := []struct {
		mountPath string
		path      string
		want      string
	}{
		{mountPath: "/data", path: "/mydir", want: "/data//mydir"},
		{mountPath: "/mnt/pvc", path: "/subdir", want: "/mnt/pvc//subdir"},
		{mountPath: "/data", path: "", want: "/data/"},
	}

	for _, tt := range tests {
		got := tt.mountPath + "/" + tt.path
		if got != tt.want {
			t.Errorf("mountPath+path: got %q, want %q", got, tt.want)
		}
	}
}
