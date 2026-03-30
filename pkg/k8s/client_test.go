package k8s

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func errExec(err error) execFunc {
	return func(_ context.Context, _, _, _ string, _ []string) (string, string, error) {
		return "", "", err
	}
}

func errExecWithStderr(err error, stderr string) execFunc {
	return func(_ context.Context, _, _, _ string, _ []string) (string, string, error) {
		return "", stderr, err
	}
}

func okExecWith(stdout string) execFunc {
	return func(_ context.Context, _, _, _ string, _ []string) (string, string, error) {
		return stdout, "", nil
	}
}

type callCountExec struct {
	count   int
	results []execResult
}

type execResult struct {
	stdout string
	stderr string
	err    error
}

func (m *callCountExec) fn() execFunc {
	return func(_ context.Context, _, _, _ string, _ []string) (string, string, error) {
		idx := m.count
		m.count++
		if idx >= len(m.results) {
			return "", "", fmt.Errorf("unexpected exec call #%d", idx)
		}
		r := m.results[idx]
		return r.stdout, r.stderr, r.err
	}
}

func newTestClient(fn execFunc) *Client {
	return &Client{
		clientset: fake.NewSimpleClientset(),
		execFn:    fn,
	}
}

func TestTryListFilesGNUlsSucceeds(t *testing.T) {
	stdout := `total 4
-rw-r--r-- 1 root root 42 2024-01-15 10:30 hello.txt`
	c := newTestClient(okExecWith(stdout))
	files, err := c.tryListFiles(context.Background(), "ns", "pod", "container", "/data", "/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
	if files[0].Name != "hello.txt" {
		t.Errorf("expected name 'hello.txt', got %q", files[0].Name)
	}
}

func TestTryListFilesGNUlsFailsBusyboxSucceeds(t *testing.T) {
	busyboxStdout := `total 4
-rw-r--r--    1 root     root           10 Jan 15 10:30 data.csv`
	noShellErr := fmt.Errorf("command terminated with exit code 127")

	m := &callCountExec{
		results: []execResult{
			{err: noShellErr, stderr: "sh: ls: not found"},
			{stdout: busyboxStdout},
		},
	}
	c := newTestClient(m.fn())
	files, err := c.tryListFiles(context.Background(), "ns", "pod", "container", "/data", "/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 || files[0].Name != "data.csv" {
		t.Errorf("unexpected files: %v", files)
	}
}

func TestTryListFilesAllFailReturnsRBACOverNoShell(t *testing.T) {
	rbacErr := apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "pod", fmt.Errorf("no exec"))
	noShellErr := fmt.Errorf("command terminated with exit code 127")

	m := &callCountExec{
		results: []execResult{
			{err: rbacErr, stderr: ""},
			{err: noShellErr, stderr: "sh: ls: not found"},
			{err: noShellErr, stderr: "sh: find: not found"},
		},
	}
	c := newTestClient(m.fn())
	_, err := c.tryListFiles(context.Background(), "ns", "pod", "container", "/data", "/")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var k8sErr *K8sError
	if !errors.As(err, &k8sErr) {
		t.Fatalf("expected *K8sError, got %T", err)
	}
	if k8sErr.Kind != ErrKindRBAC {
		t.Errorf("expected RBAC kind, got %q", k8sErr.Kind)
	}
}

func TestTryListFilesAllFailReturnsNoShell(t *testing.T) {
	noShellErr := fmt.Errorf("command terminated with exit code 127")

	m := &callCountExec{
		results: []execResult{
			{err: noShellErr, stderr: "sh: ls: not found"},
			{err: noShellErr, stderr: "sh: ls: not found"},
			{err: noShellErr, stderr: "sh: find: not found"},
		},
	}
	c := newTestClient(m.fn())
	_, err := c.tryListFiles(context.Background(), "ns", "pod", "container", "/data", "/")
	var k8sErr *K8sError
	if !errors.As(err, &k8sErr) {
		t.Fatalf("expected *K8sError, got %T: %v", err, err)
	}
	if k8sErr.Kind != ErrKindNoShell {
		t.Errorf("expected NoShell kind, got %q (msg: %s)", k8sErr.Kind, k8sErr.Message)
	}
}

func TestTryListFilesTimeoutPropagated(t *testing.T) {
	m := &callCountExec{
		results: []execResult{
			{err: context.DeadlineExceeded},
			{err: context.DeadlineExceeded},
			{err: context.DeadlineExceeded},
		},
	}
	c := newTestClient(m.fn())
	_, err := c.tryListFiles(context.Background(), "ns", "pod", "container", "/data", "/")
	var k8sErr *K8sError
	if !errors.As(err, &k8sErr) {
		t.Fatalf("expected *K8sError, got %T", err)
	}
	if k8sErr.Kind != ErrKindTimeout {
		t.Errorf("expected Timeout kind, got %q", k8sErr.Kind)
	}
}

func TestTryListFilesPathNotFound(t *testing.T) {
	pathErr := fmt.Errorf("command terminated with exit code 2")
	m := &callCountExec{
		results: []execResult{
			{err: pathErr, stderr: "ls: /nonexistent: No such file or directory"},
			{err: pathErr, stderr: "ls: /nonexistent: No such file or directory"},
			{err: pathErr, stderr: "ls: /nonexistent: No such file or directory"},
		},
	}
	c := newTestClient(m.fn())
	_, err := c.tryListFiles(context.Background(), "ns", "pod", "container", "/data", "/nonexistent")
	var k8sErr *K8sError
	if !errors.As(err, &k8sErr) {
		t.Fatalf("expected *K8sError, got %T", err)
	}
	if k8sErr.Kind != ErrKindPathNotFound {
		t.Errorf("expected PathNotFound kind, got %q", k8sErr.Kind)
	}
}

func TestCreateHelperPodForbidden(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	fakeClient.PrependReactor("create", "pods", func(_ ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(
			schema.GroupResource{Resource: "pods"}, "", fmt.Errorf("not allowed"),
		)
	})

	c := &Client{clientset: fakeClient}
	_, err := c.createHelperPod(context.Background(), "default", "my-pvc", "vol", "node1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var k8sErr *K8sError
	if !errors.As(err, &k8sErr) {
		t.Fatalf("expected *K8sError, got %T: %v", err, err)
	}
	if k8sErr.Kind != ErrKindRBAC {
		t.Errorf("expected RBAC kind, got %q", k8sErr.Kind)
	}
}

func TestCreateHelperPodPendingTimeout(t *testing.T) {
	os.Setenv("HELPER_STARTUP_TIMEOUT_SEC", "1")
	defer os.Unsetenv("HELPER_STARTUP_TIMEOUT_SEC")

	fakeClient := fake.NewSimpleClientset()
	fakeClient.PrependReactor("get", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		getAction := action.(ktesting.GetAction)
		return true, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      getAction.GetName(),
				Namespace: action.GetNamespace(),
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodPending,
			},
		}, nil
	})

	c := &Client{clientset: fakeClient}
	_, err := c.createHelperPod(context.Background(), "default", "my-pvc", "vol", "node1")
	if err == nil {
		t.Fatal("expected error for pending pod, got nil")
	}
	var k8sErr *K8sError
	if !errors.As(err, &k8sErr) {
		t.Fatalf("expected *K8sError, got %T: %v", err, err)
	}
	if k8sErr.Kind != ErrKindHelperPending {
		t.Errorf("expected HelperPending kind, got %q (msg: %s)", k8sErr.Kind, k8sErr.Message)
	}
}

func TestCreateHelperPodRunning(t *testing.T) {
	os.Setenv("HELPER_STARTUP_TIMEOUT_SEC", "1")
	defer os.Unsetenv("HELPER_STARTUP_TIMEOUT_SEC")

	fakeClient := fake.NewSimpleClientset()
	fakeClient.PrependReactor("get", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
		getAction := action.(ktesting.GetAction)
		return true, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      getAction.GetName(),
				Namespace: action.GetNamespace(),
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
			},
		}, nil
	})

	c := &Client{clientset: fakeClient}
	name, err := c.createHelperPod(context.Background(), "default", "my-pvc", "vol", "node1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name == "" {
		t.Error("expected non-empty helper pod name")
	}
}
