package k8s

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func newMockClient(mock *mockPodExecutor) *Client {
	return &Client{
		clientset: fake.NewSimpleClientset(),
		executor:  mock,
	}
}

func runningPodWithPVC(pvcName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: "node-1",
			Volumes: []corev1.Volume{
				{
					Name: "data-vol",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "alpine",
					VolumeMounts: []corev1.VolumeMount{
						{Name: "data-vol", MountPath: "/data"},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
}

func TestTryListFilesGNUlsSucceeds(t *testing.T) {
	stdout := `total 4
-rw-r--r-- 1 root root 42 2024-01-15 10:30 hello.txt`
	mock := &mockPodExecutor{}
	mock.pushExec(stdout, "", nil)
	c := newMockClient(mock)
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

	mock := &mockPodExecutor{}
	mock.pushExec("", "sh: ls: not found", noShellErr)
	mock.pushExec(busyboxStdout, "", nil)
	c := newMockClient(mock)

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

	mock := &mockPodExecutor{}
	mock.pushExec("", "", rbacErr)
	mock.pushExec("", "sh: ls: not found", noShellErr)
	mock.pushExec("", "sh: find: not found", noShellErr)
	c := newMockClient(mock)

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

	mock := &mockPodExecutor{}
	mock.pushExec("", "sh: ls: not found", noShellErr)
	mock.pushExec("", "sh: ls: not found", noShellErr)
	mock.pushExec("", "sh: find: not found", noShellErr)
	c := newMockClient(mock)

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
	mock := &mockPodExecutor{}
	mock.pushExec("", "", context.DeadlineExceeded)
	mock.pushExec("", "", context.DeadlineExceeded)
	mock.pushExec("", "", context.DeadlineExceeded)
	c := newMockClient(mock)

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
	mock := &mockPodExecutor{}
	mock.pushExec("", "ls: /nonexistent: No such file or directory", pathErr)
	mock.pushExec("", "ls: /nonexistent: No such file or directory", pathErr)
	mock.pushExec("", "ls: /nonexistent: No such file or directory", pathErr)
	c := newMockClient(mock)

	_, err := c.tryListFiles(context.Background(), "ns", "pod", "container", "/data", "/nonexistent")
	var k8sErr *K8sError
	if !errors.As(err, &k8sErr) {
		t.Fatalf("expected *K8sError, got %T", err)
	}
	if k8sErr.Kind != ErrKindPathNotFound {
		t.Errorf("expected PathNotFound kind, got %q", k8sErr.Kind)
	}
}

func TestListFilesDirectSucceeds(t *testing.T) {
	const pvcName = "my-pvc"
	stdout := `total 4
-rw-r--r-- 1 root root 100 2024-01-15 10:30 direct.txt`

	fakeClient := fake.NewSimpleClientset(runningPodWithPVC(pvcName))
	mock := &mockPodExecutor{}
	mock.pushExec(stdout, "", nil)

	c := &Client{clientset: fakeClient, executor: mock}
	files, err := c.ListFiles(context.Background(), "default", pvcName, "/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 || files[0].Name != "direct.txt" {
		t.Errorf("unexpected files: %v", files)
	}
	if mock.createCalled != 0 {
		t.Error("expected no helper pod creation for direct success")
	}
}

func TestListFilesDirectFailsFallsBackToHelperPod(t *testing.T) {
	const pvcName = "my-pvc"
	noShellErr := fmt.Errorf("command terminated with exit code 127")
	helperStdout := `total 4
-rw-r--r-- 1 root root 200 2024-01-15 10:30 from-helper.txt`

	fakeClient := fake.NewSimpleClientset(runningPodWithPVC(pvcName))
	mock := &mockPodExecutor{
		createResult: "kube-browser-helper-abc",
	}
	mock.pushExec("", "sh: ls: not found", noShellErr)
	mock.pushExec("", "sh: ls: not found", noShellErr)
	mock.pushExec("", "sh: find: not found", noShellErr)
	mock.pushExec(helperStdout, "", nil)

	c := &Client{clientset: fakeClient, executor: mock}
	files, err := c.ListFiles(context.Background(), "default", pvcName, "/")
	if err != nil {
		t.Fatalf("unexpected error from ListFiles: %v", err)
	}
	if len(files) != 1 || files[0].Name != "from-helper.txt" {
		t.Errorf("unexpected files: %v", files)
	}
	if mock.createCalled != 1 {
		t.Errorf("expected 1 helper pod creation, got %d", mock.createCalled)
	}

	time.Sleep(50 * time.Millisecond)
	mock.mu.Lock()
	deleteCalled := mock.deleteCalled
	mock.mu.Unlock()
	if deleteCalled != 1 {
		t.Errorf("expected 1 helper pod deletion, got %d", deleteCalled)
	}
}

func TestListFilesDirectFailsHelperPodCreateFails(t *testing.T) {
	const pvcName = "my-pvc"
	noShellErr := fmt.Errorf("command terminated with exit code 127")
	rbacErr := &K8sError{Kind: ErrKindRBAC, Message: "pods forbidden"}

	fakeClient := fake.NewSimpleClientset(runningPodWithPVC(pvcName))
	mock := &mockPodExecutor{
		createErr: rbacErr,
	}
	mock.pushExec("", "sh: ls: not found", noShellErr)
	mock.pushExec("", "sh: ls: not found", noShellErr)
	mock.pushExec("", "sh: find: not found", noShellErr)

	c := &Client{clientset: fakeClient, executor: mock}
	_, err := c.ListFiles(context.Background(), "default", pvcName, "/")
	if err == nil {
		t.Fatal("expected error when helper pod creation fails")
	}
	var k8sErr *K8sError
	if !errors.As(err, &k8sErr) {
		t.Fatalf("expected *K8sError, got %T: %v", err, err)
	}
	if k8sErr.Kind != ErrKindRBAC {
		t.Errorf("expected RBAC kind, got %q", k8sErr.Kind)
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
