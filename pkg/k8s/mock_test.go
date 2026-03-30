package k8s

import (
	"context"
	"errors"
	"sync"
)

type execCall struct {
	namespace     string
	podName       string
	containerName string
	cmd           []string
}

type mockPodExecutor struct {
	mu sync.Mutex

	execResults []struct {
		stdout string
		stderr string
		err    error
	}
	execCalls []execCall

	createResult string
	createErr    error
	createCalled int

	deleteCalled int
	deleteArgs   []struct{ ns, pod string }
}

func (m *mockPodExecutor) execInPod(_ context.Context, namespace, podName, containerName string, cmd []string) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.execCalls = append(m.execCalls, execCall{namespace, podName, containerName, cmd})
	if len(m.execResults) == 0 {
		return "", "", errors.New("no mock exec result configured")
	}
	r := m.execResults[0]
	m.execResults = m.execResults[1:]
	return r.stdout, r.stderr, r.err
}

func (m *mockPodExecutor) createHelperPod(_ context.Context, _, _, _, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createCalled++
	return m.createResult, m.createErr
}

func (m *mockPodExecutor) deleteHelperPod(_ context.Context, ns, pod string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteCalled++
	m.deleteArgs = append(m.deleteArgs, struct{ ns, pod string }{ns, pod})
}

func (m *mockPodExecutor) pushExec(stdout, stderr string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.execResults = append(m.execResults, struct {
		stdout string
		stderr string
		err    error
	}{stdout, stderr, err})
}
