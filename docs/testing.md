# Testing Guide

This document explains the testing strategy for KubeBrowser: what is covered by automated tests, what requires a real cluster, and how to validate behavior manually.

---

## Overview

KubeBrowser's test suite is split into two categories:

| Category | Requires cluster? | How to run |
|---|---|---|
| Unit tests | No | `go test ./...` |
| Manual integration tests | Yes | See [Manual Validation](#manual-validation) |

---

## Unit Tests (No Cluster Required)

All unit tests can run locally without a kubeconfig or a running cluster. They use a `mockPodExecutor` to simulate Kubernetes exec responses.

Run them with:

```bash
go test ./... -timeout 120s
```

With race detection (recommended before submitting a PR):

```bash
go test ./... -timeout 120s -race
```

### What the tests cover

#### `pkg/k8s` — Kubernetes logic

| Test file | What it covers |
|---|---|
| `parse_test.go` | Parsing `ls -la` output (GNU coreutils and BusyBox), `find + stat` output, symlinks, malformed lines |
| `errors_test.go` | `classifyExecError`: maps exec errors to `ErrorKind` (RBAC, NoShell, Timeout, PathNotFound, PermDenied); `mostActionableError` picks the most specific error from a list |
| `client_test.go` | Full exec fallback chain (GNU ls → BusyBox ls → find+stat → helper pod); helper pod creation, file listing, and deletion; timeout propagation; stuck-in-Pending helper pod detection |
| `mock_test.go` | `mockPodExecutor`: queued exec results and create/delete call counters used by all client tests |

#### `pkg/handlers` — HTTP handlers

| Test file | What it covers |
|---|---|
| `handlers_test.go` | `sanitizePath`: rejects path traversal (`../`), null bytes, overly long paths; accepts valid relative paths |

#### Test counts (as of v1.0.11)

```
pkg/handlers    — 4 test cases (sanitizePath)
pkg/k8s         — 26 test cases across parse, errors, client
Total           — 30 tests
```

---

## What Requires a Real Cluster

The following behaviors cannot be covered by unit tests because they depend on live Kubernetes API responses, real container exec, or actual file I/O:

| Behavior | Why it needs a cluster |
|---|---|
| End-to-end file listing in a real PVC | Requires a mounted volume and a running pod |
| Helper pod creation and scheduling | Requires the Kubernetes scheduler and a node |
| Upload and download of real files | Requires exec into a pod and actual byte streams |
| RBAC error detection | Requires a real API server to return 403 |
| Timeout behavior under network failure | Requires simulating real network conditions |
| Cleanup of orphaned helper pods | Requires a real pod to exist and be deleted |

---

## Manual Validation

### Basic flow

1. Start KubeBrowser pointing to a working cluster:
   ```bash
   ./kube-browser
   ```
2. Select your kubeconfig, context, and namespace in the Connection modal.
3. Select a PVC from the sidebar.
4. Verify the file listing loads correctly.
5. Upload a small test file and verify it appears in the listing.
6. Download the file and verify the content matches.

### Testing the fallback chain

The fallback chain exercises three listing strategies in order: GNU ls → BusyBox ls → helper pod.

To force the helper pod path:
- Use a container without shell tools (e.g., a distroless or scratch-based image).
- KubeBrowser should automatically create a BusyBox helper pod, list the files, and delete the pod.
- Verify with `kubectl get pods -n <namespace>` that the helper pod is gone after the listing.

### Testing RBAC errors

Apply a restrictive Role that omits `pods/exec`:
```yaml
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]
- apiGroups: [""]
  resources: ["pods/exec"]
  verbs: []  # intentionally empty
```

KubeBrowser should display an actionable error message identifying the RBAC issue, not a generic failure.

### Testing read-only mode

```bash
KUBE_BROWSER_READ_ONLY=true ./kube-browser
```

- The Upload button should be disabled in the UI.
- A "Read-only" badge should appear in the header.
- `POST /api/upload` should return `405` with `{"error": "read-only mode: write operations are disabled"}`.
- Browse, list, and download should continue to work normally.

### Testing the file browser (kubeconfig selection)

- Run on a machine where `~/.kube/config` does not exist.
- The kubeconfig field should be empty (path shown as placeholder only).
- Clicking the Browse button should open the file browser at the home directory without errors.

---

## Adding New Tests

When adding a feature that has testable logic (parsers, classifiers, path handlers), add unit tests in the relevant `_test.go` file. Use `mockPodExecutor` to simulate Kubernetes responses — see `pkg/k8s/mock_test.go` for the pattern.

If your feature requires a real cluster to test end-to-end, document the manual validation steps in a PR comment or in this file under [Manual Validation](#manual-validation).
