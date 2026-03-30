package k8s

import (
	"context"
	"errors"
	"fmt"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestClassifyExecError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		stderr   string
		wantKind ErrorKind
	}{
		{
			name:     "context deadline exceeded → Timeout",
			err:      context.DeadlineExceeded,
			stderr:   "",
			wantKind: ErrKindTimeout,
		},
		{
			name:     "context canceled → Timeout",
			err:      context.Canceled,
			stderr:   "",
			wantKind: ErrKindTimeout,
		},
		{
			name:     "k8s forbidden error → RBAC",
			err:      apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "mypod", fmt.Errorf("not allowed")),
			stderr:   "",
			wantKind: ErrKindRBAC,
		},
		{
			name:     "error message contains 'forbidden' → RBAC",
			err:      fmt.Errorf("pods is forbidden: user cannot exec"),
			stderr:   "",
			wantKind: ErrKindRBAC,
		},
		{
			name:     "executable file not found in error → NoShell",
			err:      fmt.Errorf("exec: \"ls\": executable file not found in $PATH"),
			stderr:   "",
			wantKind: ErrKindNoShell,
		},
		{
			name:     "tool ls not found in stderr → NoShell",
			err:      fmt.Errorf("command terminated with exit code 127"),
			stderr:   "sh: ls: not found",
			wantKind: ErrKindNoShell,
		},
		{
			name:     "tool find not found in stderr → NoShell",
			err:      fmt.Errorf("command terminated with exit code 127"),
			stderr:   "sh: find: not found",
			wantKind: ErrKindNoShell,
		},
		{
			name:     "permission denied in stderr → PermDenied",
			err:      fmt.Errorf("command terminated with exit code 1"),
			stderr:   "ls: /secret: Permission denied",
			wantKind: ErrKindPermDenied,
		},
		{
			name:     "no such file or directory in stderr → PathNotFound",
			err:      fmt.Errorf("command terminated with exit code 2"),
			stderr:   "ls: /nonexistent: No such file or directory",
			wantKind: ErrKindPathNotFound,
		},
		{
			name:     "generic error → Unknown",
			err:      fmt.Errorf("some random error"),
			stderr:   "",
			wantKind: ErrKindUnknown,
		},
		{
			name:     "nil error → nil result",
			err:      nil,
			stderr:   "",
			wantKind: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyExecError(tt.err, tt.stderr)
			if tt.err == nil {
				if got != nil {
					t.Errorf("expected nil for nil error, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("classifyExecError returned nil for non-nil error")
			}
			if got.Kind != tt.wantKind {
				t.Errorf("Kind: got %q, want %q (message: %s)", got.Kind, tt.wantKind, got.Message)
			}
			if got.Message == "" {
				t.Errorf("Message should not be empty")
			}
		})
	}
}

func TestClassifyExecErrorWrapsOriginal(t *testing.T) {
	origErr := fmt.Errorf("original cause")
	got := classifyExecError(origErr, "")
	if !errors.Is(got, origErr) {
		t.Errorf("expected errors.Is to find original error via Unwrap chain")
	}
}

func TestClassifyPodError(t *testing.T) {
	tests := []struct {
		name     string
		phase    string
		reason   string
		wantKind ErrorKind
		wantMsg  string
	}{
		{
			name:     "pending no reason → HelperPending generic",
			phase:    "Pending",
			reason:   "",
			wantKind: ErrKindHelperPending,
			wantMsg:  "Pending",
		},
		{
			name:     "pending imagepullbackoff → HelperPending with ImagePull message",
			phase:    "Pending",
			reason:   "ImagePullBackOff",
			wantKind: ErrKindHelperPending,
			wantMsg:  "ImagePullBackOff",
		},
		{
			name:     "pending unschedulable → HelperPending with node message",
			phase:    "Pending",
			reason:   "Unschedulable",
			wantKind: ErrKindHelperPending,
			wantMsg:  "node",
		},
		{
			name:     "failed no reason → HelperPending",
			phase:    "Failed",
			reason:   "",
			wantKind: ErrKindHelperPending,
		},
		{
			name:     "failed imagepullbackoff → HelperPending",
			phase:    "Failed",
			reason:   "ErrImagePull",
			wantKind: ErrKindHelperPending,
			wantMsg:  "ImagePullBackOff",
		},
		{
			name:     "empty phase → HelperPending generic",
			phase:    "",
			reason:   "",
			wantKind: ErrKindHelperPending,
		},
		{
			name:     "unknown phase → Unknown kind",
			phase:    "Succeeded",
			reason:   "",
			wantKind: ErrKindUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyPodError(tt.phase, tt.reason)
			if got == nil {
				t.Fatal("classifyPodError returned nil")
			}
			if got.Kind != tt.wantKind {
				t.Errorf("Kind: got %q, want %q", got.Kind, tt.wantKind)
			}
			if tt.wantMsg != "" {
				if !containsInsensitive(got.Message, tt.wantMsg) {
					t.Errorf("Message %q does not contain %q", got.Message, tt.wantMsg)
				}
			}
		})
	}
}

func containsInsensitive(s, sub string) bool {
	sl := make([]byte, len(s))
	subl := make([]byte, len(sub))
	copy(sl, s)
	copy(subl, sub)
	for i := range sl {
		if sl[i] >= 'A' && sl[i] <= 'Z' {
			sl[i] += 32
		}
	}
	for i := range subl {
		if subl[i] >= 'A' && subl[i] <= 'Z' {
			subl[i] += 32
		}
	}
	return len(subl) == 0 || containsBytes(sl, subl)
}

func containsBytes(s, sub []byte) bool {
	if len(sub) == 0 {
		return true
	}
	if len(s) < len(sub) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		match := true
		for j := range sub {
			if s[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func TestMostActionableError(t *testing.T) {
	rbac := &K8sError{Kind: ErrKindRBAC, Message: "rbac"}
	noShell := &K8sError{Kind: ErrKindNoShell, Message: "noshell"}
	pathNotFound := &K8sError{Kind: ErrKindPathNotFound, Message: "pathnotfound"}
	unknown := &K8sError{Kind: ErrKindUnknown, Message: "unknown"}

	tests := []struct {
		name     string
		errs     []*K8sError
		wantKind ErrorKind
	}{
		{
			name:     "RBAC wins over NoShell",
			errs:     []*K8sError{noShell, rbac},
			wantKind: ErrKindRBAC,
		},
		{
			name:     "NoShell wins over Unknown",
			errs:     []*K8sError{unknown, noShell},
			wantKind: ErrKindNoShell,
		},
		{
			name:     "PathNotFound beats Unknown",
			errs:     []*K8sError{unknown, pathNotFound},
			wantKind: ErrKindPathNotFound,
		},
		{
			name:     "nil entries skipped",
			errs:     []*K8sError{nil, noShell, nil},
			wantKind: ErrKindNoShell,
		},
		{
			name:     "all nil returns nil",
			errs:     []*K8sError{nil, nil},
			wantKind: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mostActionableError(tt.errs...)
			if tt.wantKind == "" {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil K8sError")
			}
			if got.Kind != tt.wantKind {
				t.Errorf("Kind: got %q, want %q", got.Kind, tt.wantKind)
			}
		})
	}
}
