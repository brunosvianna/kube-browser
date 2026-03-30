<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go Version"/>
  <img src="https://img.shields.io/badge/Kubernetes-client--go-326CE5?style=flat&logo=kubernetes&logoColor=white" alt="Kubernetes"/>
  <img src="https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey?style=flat" alt="Platform"/>
  <img src="https://img.shields.io/github/license/brunosvianna/kube-browser?style=flat" alt="License"/>
</p>

# KubeBrowser

**A desktop file manager for Kubernetes Persistent Volume Claims.**

KubeBrowser is a single binary that launches a web-based IDE for browsing, downloading, and uploading files stored in Kubernetes PVCs. No installation, no dependencies — just run the binary and manage your persistent data.

---

## Demo

![KubeBrowser Demo](docs/kube-browser-demo.gif)

---

## Why KubeBrowser?

Working with files inside Kubernetes PVCs usually means chaining together `kubectl exec`, `kubectl cp`, or spinning up debug pods manually. KubeBrowser replaces that workflow with a visual file manager that:

- **Connects to any cluster** via your existing kubeconfig
- **Lists all PVCs** in a namespace and shows which pods mount them
- **Browses directories** inside PVCs with a familiar file explorer UI
- **Downloads and uploads files** with a single click
- **Works with any container image** — even distroless or minimal images that lack shell tools (Redis, RabbitMQ, etc.)

---

## Quick Start

### 1. Download

Grab the latest release for your platform from [Releases](https://github.com/brunosvianna/kube-browser/releases):

| Platform       | Architecture | File                                   |
|----------------|-------------|----------------------------------------|
| Linux          | AMD64       | `kube-browser-linux-amd64.tar.gz`      |
| Linux          | ARM64       | `kube-browser-linux-arm64.tar.gz`      |
| macOS          | Intel       | `kube-browser-darwin-amd64.tar.gz`     |
| macOS          | Apple Silicon | `kube-browser-darwin-arm64.tar.gz`   |
| Windows        | AMD64       | `kube-browser-windows-amd64.zip`       |

### 2. Extract

**Linux / macOS:**
```bash
tar -xzf kube-browser-*.tar.gz
chmod +x kube-browser
```

**Windows:**
Extract the `.zip` file.

### 3. Run

```bash
./kube-browser
```

The UI opens automatically in your browser. If it doesn't, navigate to `http://localhost:5000`.

---

## Usage

### Connecting to a Cluster

When KubeBrowser starts, a connection dialog appears:

1. **Kubeconfig** — The path to your kubeconfig file (auto-detected). Use the folder icon to browse your filesystem.
2. **Load** — Click the arrow icon to load available contexts from the kubeconfig.
3. **Context** — Select the Kubernetes context you want to use.
4. **Namespace** — Choose a namespace or leave as "All namespaces".
5. **Connect** — Click to establish the connection.

You can switch clusters at any time by clicking **Connection** in the top-right corner.

### Browsing Files

After connecting:

1. Select a **namespace** from the sidebar dropdown.
2. Click on a **PVC** to browse its contents. Each PVC card shows:
   - Bound status and capacity
   - Which pod currently mounts it
3. Navigate directories by clicking on folders.
4. Use the **breadcrumb** at the top to go back to parent directories.

### Downloading Files

Click on any file to download it directly to your machine.

### Uploading Files

1. Click the **Upload** button in the toolbar.
2. Drag & drop a file or click to select one.
3. The file is uploaded to the currently viewed directory.

---

## How It Works

KubeBrowser runs entirely on your machine and communicates with the cluster through the Kubernetes API using your existing kubeconfig credentials. No permanent in-cluster components are installed — the helper pod is a temporary fallback that is deleted immediately after use.

### Normal mode (direct exec)

```
  Your machine
  ┌─────────────────────────────────────────────────────────────┐
  │                                                             │
  │  Browser ──────> KubeBrowser binary (127.0.0.1:5000)       │
  │                        │                                    │
  │                        │  reads kubeconfig                  │
  │                        ▼                                    │
  │                  Kubernetes API ──> pods/exec               │
  │                        │                                    │
  └────────────────────────┼────────────────────────────────────┘
                           │  exec: ls / cat / tee
                           ▼
                    ┌─────────────┐
                    │  App pod    │  (already running)
                    │             │
                    │  /data ─────┼──> PVC contents
                    └─────────────┘
```

1. The browser (running on the same machine) connects to KubeBrowser on `127.0.0.1:5000`.
2. KubeBrowser locates the running pod that mounts the target PVC.
3. File listing runs `ls` inside that pod via the Kubernetes exec API.
4. Downloads stream file content via `cat`; uploads write via `tee`.

KubeBrowser tries three listing strategies in order, falling back when the previous one fails:

| Strategy | Command | Requires |
|----------|---------|---------|
| GNU ls   | `ls -la --time-style=long-iso` | GNU coreutils |
| BusyBox ls | `ls -la` | BusyBox or any POSIX ls |
| find + stat | `find … -exec stat …` | find + stat |

### Helper Pod mode (fallback for minimal/distroless images)

When all three exec strategies fail (e.g. the container has no shell at all — Redis, RabbitMQ, distroless images), KubeBrowser automatically switches to helper pod mode:

```
  Your machine
  ┌─────────────────────────────────────────────────────────────┐
  │                                                             │
  │  Browser ──────> KubeBrowser binary (127.0.0.1:5000)       │
  │                        │                                    │
  │                        │  reads kubeconfig                  │
  │                        ▼                                    │
  │                  Kubernetes API                             │
  │                   │         │                               │
  │               pods/create  pods/exec                        │
  └───────────────────┼─────────┼───────────────────────────────┘
                      │         │
                      ▼         ▼
             ┌──────────────┐  ┌──────────────┐
             │  App pod     │  │ Helper pod   │  (temporary alpine)
             │  (no shell)  │  │              │
             │              │  │  /data ──────┼──> same PVC
             └──────────────┘  └──────────────┘
                                      │
                              deleted after use
```

1. KubeBrowser creates a temporary `alpine:3.19` pod on the **same node** as the original pod, mounting the same PVC.
2. All file operations (list / download / upload) run through the helper pod.
3. The helper pod is deleted immediately after the operation completes (or fails).
4. Helper pods are named `kube-browser-helper-<pvc>-<timestamp>` and labelled `managed-by: kube-browser`.

**What you see in the logs:**
```
Trying GNU ls on default/redis-pod (container: redis, mount: /data)
  stderr: exec: "ls": executable file not found in $PATH
Trying BusyBox ls on default/redis-pod ...
  stderr: exec: "ls": executable file not found in $PATH
Trying find on default/redis-pod ...
  stderr: exec: "find": executable file not found in $PATH
Direct exec failed, creating helper pod for PVC redis-data on node worker-1
Creating helper pod kube-browser-helper-redis-data-1a2b3c on node worker-1 for PVC redis-data (image: alpine:3.19)
Helper pod kube-browser-helper-redis-data-1a2b3c is running
```

---

## Configuration

### Bind address

By default, KubeBrowser binds to `127.0.0.1` (loopback only), so it is not reachable from other hosts on the network. Override with the `HOST` environment variable:

```bash
HOST=0.0.0.0 ./kube-browser   # listen on all interfaces (use with care)
```

### Port

By default, KubeBrowser listens on port `5000`. Override it with the `PORT` environment variable:

```bash
PORT=8080 ./kube-browser
```

### Timeouts

All timeout values are specified in **seconds** and can be overridden via environment variables:

| Variable           | Default | Description                                          |
|--------------------|---------|------------------------------------------------------|
| `READ_TIMEOUT`     | `15`    | Maximum time to read the full request (headers + body) |
| `WRITE_TIMEOUT`    | `60`    | Maximum time to write the response                   |
| `IDLE_TIMEOUT`     | `120`   | Maximum time to keep an idle keep-alive connection   |
| `SHUTDOWN_TIMEOUT` | `10`    | Grace period to drain active connections on shutdown |

Example:

```bash
READ_TIMEOUT=30 WRITE_TIMEOUT=120 ./kube-browser
```

### Helper Pod tuning

| Variable                  | Default      | Description                                          |
|---------------------------|-------------|------------------------------------------------------|
| `HELPER_IMAGE`            | `alpine:3.19` | Image used for the helper pod                      |
| `HELPER_STARTUP_TIMEOUT_SEC` | `60`     | Seconds to wait for the helper pod to become Running |
| `HELPER_CPU_REQUEST`      | `10m`        | CPU request for the helper pod container             |
| `HELPER_MEM_REQUEST`      | `16Mi`       | Memory request for the helper pod container          |
| `HELPER_CPU_LIMIT`        | `100m`       | CPU limit for the helper pod container               |
| `HELPER_MEM_LIMIT`        | `64Mi`       | Memory limit for the helper pod container            |
| `HELPER_RUN_AS_ROOT`      | `false`      | Set to `true` to run the helper as root (UID 0)      |
| `HELPER_RUN_AS_USER`      | _(unset)_    | Specific UID to run the helper container as          |

### Helper Pod — cluster-specific configuration

These variables let you adapt the helper pod to clusters with stricter admission policies, private registries, or dedicated node pools.

| Variable                          | Default   | Description                                                                                      |
|-----------------------------------|-----------|--------------------------------------------------------------------------------------------------|
| `KUBE_BROWSER_IMAGE_PULL_SECRET`  | _(unset)_ | Name of an `imagePullSecret` in the target namespace, used when the helper image is in a private registry. |
| `KUBE_BROWSER_SERVICE_ACCOUNT`    | _(unset)_ | `serviceAccountName` for the helper pod. Useful when your cluster's RBAC or OPA requires a specific account. |
| `KUBE_BROWSER_NODE_SELECTOR`      | _(unset)_ | Pin the helper pod to specific nodes. Accepts `key=value,key=value` or a JSON object `{"key":"value"}`. |
| `KUBE_BROWSER_TOLERATIONS`        | _(unset)_ | JSON array of Kubernetes [Toleration](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) objects, allowing the helper pod to run on tainted nodes. |
| `KUBE_BROWSER_EXTRA_LABELS`       | _(unset)_ | Additional labels to attach to the helper pod. Format: `key=value,key=value`. Merged with the built-in `app` and `managed-by` labels. |
| `KUBE_BROWSER_EXTRA_ANNOTATIONS`  | _(unset)_ | Annotations to attach to the helper pod. Format: `key=value,key=value`. Useful for Vault injection, Datadog APM, etc. |

#### Example: restricted cluster (private registry + GPU taint)

```bash
# Pull helper image from internal mirror
HELPER_IMAGE=registry.internal.example.com/alpine:3.19 \

# Authenticate to the private registry
KUBE_BROWSER_IMAGE_PULL_SECRET=registry-credentials \

# Run with a dedicated service account
KUBE_BROWSER_SERVICE_ACCOUNT=kube-browser-helper \

# Schedule only on CPU-optimised nodes
KUBE_BROWSER_NODE_SELECTOR="node-pool=cpu-optimised,disk=ssd" \

# Tolerate a GPU-reserved taint so the helper can co-locate with GPU workloads
KUBE_BROWSER_TOLERATIONS='[{"key":"nvidia.com/gpu","operator":"Exists","effect":"NoSchedule"}]' \

# Add custom labels required by your OPA policies
KUBE_BROWSER_EXTRA_LABELS="team=platform,cost-center=infra" \

./kube-browser
```

### Read-only mode

KubeBrowser can be started in **read-only mode**, which disables all write operations (file uploads) at the server level. This is useful when you want to give colleagues or CI pipelines read access to PVCs without the risk of accidental data modification.

```bash
KUBE_BROWSER_READ_ONLY=true ./kube-browser
```

| Variable                  | Values         | Default  | Effect                                                           |
|---------------------------|----------------|----------|------------------------------------------------------------------|
| `KUBE_BROWSER_READ_ONLY`  | `true` / `1`   | _(unset)_| Rejects upload requests with HTTP 405 and disables the UI upload button. |

When read-only mode is active:
- The upload endpoint (`POST /api/upload`) returns **HTTP 405** with `{"error": "read-only mode: write operations are disabled"}`.
- A **"Read-only" badge** appears in the browser header with a lock icon.
- The **upload button** is permanently disabled regardless of which PVC is selected.
- `GET /api/status` includes `"readOnly": true` so scripts can detect the mode.

### Graceful shutdown

KubeBrowser handles `SIGINT` and `SIGTERM` gracefully: it stops accepting new connections and waits up to `SHUTDOWN_TIMEOUT` seconds for active requests to finish before exiting.

### Kubeconfig

KubeBrowser auto-detects your kubeconfig from:

1. The `KUBECONFIG` environment variable
2. `~/.kube/config` (default path)

You can also browse and select any kubeconfig file through the UI.

---

## Requirements

### Minimum RBAC permissions

For **normal mode** (direct exec into existing pods):

```yaml
rules:
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["list"]
- apiGroups: [""]
  resources: ["persistentvolumeclaims"]
  verbs: ["get", "list"]
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]
- apiGroups: [""]
  resources: ["pods/exec"]
  verbs: ["create"]
```

For **helper pod mode** (required when the app container has no shell):

```yaml
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "create", "delete"]
```

> KubeBrowser polls the helper pod status with repeated `get` calls until it reaches `Running` state.  
> `create` and `delete` on `pods` are **only** needed if your workloads use minimal/distroless images.  
> Adding `watch` is harmless and may be required by some admission policies, but it is not used by the current implementation.

A complete example ClusterRole:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kube-browser
rules:
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["list"]
- apiGroups: [""]
  resources: ["persistentvolumeclaims"]
  verbs: ["get", "list"]
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "create", "delete"]
- apiGroups: [""]
  resources: ["pods/exec"]
  verbs: ["create"]
```

---

## Security

### Loopback-only by default

KubeBrowser binds to `127.0.0.1` by default. This means only software running on your own machine can connect to it — the port is not exposed on your LAN or the internet.

If you set `HOST=0.0.0.0`, the server becomes reachable from other hosts. **Only do this on a private, trusted network.** KubeBrowser has no authentication layer — anyone who can reach the port can browse and download files from your PVCs.

### `/api/browse` — localhost-only middleware

The `/api/browse` endpoint lets the UI navigate your local filesystem to select a kubeconfig file. Because this endpoint exposes your local filesystem, it is protected by a middleware that checks the request's remote address:

- Requests from `127.0.0.1` or `::1` → allowed
- Any other origin → `403 Forbidden`

This check runs regardless of the `HOST` setting: even if you bind to `0.0.0.0`, external clients cannot access `/api/browse`.

### HTTP timeouts

The HTTP server enforces configurable timeouts on every connection to protect against slow-client attacks:

- **Read timeout** — caps the time to receive a full request (default 15 s).
- **Write timeout** — caps the time to send a full response (default 60 s; set higher for large file transfers).
- **Idle timeout** — closes keep-alive connections that have been idle too long (default 120 s).

### Path traversal protection

All file paths supplied by the UI are sanitized on the server before being passed to `ls`, `cat`, or `tee`. Paths are resolved through `path.Clean`; any path that still contains a `..` segment after cleaning is rejected with `400 Bad Request`.

### Credentials

KubeBrowser uses your existing kubeconfig credentials and respects any RBAC restrictions your cluster administrator has set. It does not store, cache, or transmit credentials beyond what `client-go` requires for the current session.

---

## Tested Environments

KubeBrowser has been developed and tested against the following distributions:

| Environment | Notes |
|-------------|-------|
| **AKS** (Azure Kubernetes Service) | Fully functional. Helper pod mode works unless pod security admission blocks `pods/create` in restricted namespaces. |
| **EKS** (Amazon Elastic Kubernetes Service) | Fully functional. IAM-to-RBAC mapping must include the required verbs. |
| **GKE** (Google Kubernetes Engine) | Fully functional. Workload Identity clusters require that the local kubeconfig uses `gke-gcloud-auth-plugin`. |
| **k3s** | Fully functional on both single-node and multi-node setups. |
| **minikube** | Fully functional. Helper pod mode tested with the `docker` driver. |
| **WSL (Windows Subsystem for Linux)** | Fully functional. Run the Linux binary inside WSL; the kubeconfig from Windows (`%USERPROFILE%\.kube\config`) can be referenced via `/mnt/c/Users/<user>/.kube/config`. |

---

## Known Risks and Limitations

### PVC with ReadWriteOnce access mode already in use

A `ReadWriteOnce` PVC can only be mounted by pods running on **the same node**. KubeBrowser's helper pod is always scheduled on the same node as the existing pod, so this should work — but if the PVC is mounted read-write and you upload a large file through the helper pod while the application is writing, a write conflict is possible. Treat uploads to active RWO volumes with care.

### Distroless and minimal images (no shell)

Containers built from `scratch`, `gcr.io/distroless/*`, or other stripped-down bases have no shell and no filesystem utilities. KubeBrowser handles this transparently via the helper pod fallback. If the helper pod mode is also blocked (e.g. missing RBAC), a descriptive error is shown in the UI with a link to the RBAC documentation.

### Clusters with PodSecurity / OPA / Gatekeeper policies

Restrictive admission webhooks (Pod Security Standards in `restricted` mode, OPA Gatekeeper, Kyverno) may block the helper pod because:
- It uses `alpine:3.19`, which may not be in the allowlist of approved images.
- It runs as a non-root user but with `readOnlyRootFilesystem: true`, which some policies require additional configuration for.

**Workaround:** set `HELPER_IMAGE` to an image allowed by your policy and, if needed, `HELPER_RUN_AS_USER` to a UID your policy accepts.

### Helper pod blocked by NetworkPolicy

NetworkPolicy rules that deny egress or ingress on the pod CIDR do not affect the helper pod directly (KubeBrowser communicates via the Kubernetes API, not directly to the pod's IP). However, if your cluster requires that all pods can only pull images from an internal registry, the helper pod creation will fail if `alpine:3.19` is not mirrored there.

**Workaround:** mirror `alpine:3.19` to your internal registry and set `HELPER_IMAGE=your-registry.example.com/alpine:3.19`.

### Private image registries

If the Kubernetes nodes cannot pull `alpine:3.19` from Docker Hub (air-gapped clusters, private registries), helper pod creation will fail with an `ImagePullBackOff` error. KubeBrowser detects this and reports `ErrKindHelperPending` in the UI.

**Workaround:** same as above — mirror the image and point `HELPER_IMAGE` to the internal copy.

### No pod mounting the PVC

KubeBrowser requires at least one **Running** pod that mounts the PVC. If the PVC exists but no running pod mounts it, the UI shows a "no running pod found" error. In that case, temporarily scale up a workload or deploy a debug pod that mounts the PVC before using KubeBrowser.

---

## Building from Source

### Prerequisites

- Go 1.25 or later

### Build

```bash
git clone https://github.com/brunosvianna/kube-browser.git
cd kube-browser
CGO_ENABLED=0 go build -o kube-browser ./cmd/kube-browser/
```

### Cross-compilation

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o kube-browser ./cmd/kube-browser/

# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o kube-browser ./cmd/kube-browser/

# Windows
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o kube-browser.exe ./cmd/kube-browser/
```

### Running tests

```bash
go test ./pkg/... -timeout 60s
```

---

## Project Structure

```
kube-browser/
├── cmd/kube-browser/
│   ├── main.go              # Entry point, HTTP server, embedded assets
│   ├── static/
│   │   ├── css/style.css    # Dark theme UI styles
│   │   └── js/app.js        # Frontend application logic
│   └── templates/
│       └── index.html       # Main HTML template
├── pkg/
│   ├── browser/
│   │   └── open.go          # Cross-platform browser auto-open
│   ├── handlers/
│   │   ├── handlers.go      # HTTP API handlers
│   │   └── handlers_test.go
│   └── k8s/
│       ├── client.go        # Kubernetes client, PVC/file operations
│       ├── errors.go        # Structured error types and classification
│       ├── executor.go      # PodExecutor interface
│       ├── parse.go         # ls/find output parsers
│       ├── client_test.go
│       ├── errors_test.go
│       ├── mock_test.go
│       ├── parse_test.go
│       └── paths_test.go
├── go.mod
└── go.sum
```

**Key design decisions:**

- **Single binary** — All frontend assets (HTML, CSS, JS) are embedded using Go's `embed` package. No external files needed.
- **No dependencies at runtime** — No Node.js, no Docker, no kubectl required. Just the binary and a kubeconfig.
- **Zero configuration** — Sensible defaults, everything configurable through the UI.
- **Testable without a cluster** — The `PodExecutor` interface allows all Kubernetes exec and pod-lifecycle calls to be replaced with in-memory mocks in tests.

---

## Roadmap

The following features are planned for future releases:

| Feature | Description |
|---------|-------------|
| **File deletion** | Delete individual files or entire directories from a PVC. |
| **Rename / move** | Rename files and move them between directories within the same PVC. |
| **Integration tests** | End-to-end tests against a real cluster using `kind`, exercising the full exec and helper pod paths. |
| **Private registry support** | Configure `KUBE_BROWSER_IMAGE_PULL_SECRET` to pull from private registries. See [Helper Pod configuration](#helper-pod--cluster-specific-configuration). |
| **Multi-file download** | Select and download multiple files as a single `.zip` archive. |
| **Directory upload** | Upload entire directory trees (expanded from the current single-file upload). |

---

## Troubleshooting

**Browser doesn't open automatically on Linux:**
```bash
# Check if xdg-open works
xdg-open http://localhost:5000

# Or open manually in your browser
firefox http://localhost:5000
```

**"exec format error" when running the binary:**
You downloaded the wrong architecture. Check your system:
```bash
uname -m
# x86_64 → use linux-amd64
# aarch64 → use linux-arm64
```

**"Failed to list files: all methods failed":**
The container doesn't have shell tools. KubeBrowser will try to create a helper pod automatically. Make sure your RBAC permissions allow `create` and `delete` on `pods` in the target namespace.

**Helper pod stuck in Pending / ImagePullBackOff:**
The node cannot pull `alpine:3.19`. Mirror the image to your internal registry and set:
```bash
HELPER_IMAGE=your-registry.example.com/alpine:3.19 ./kube-browser
```

**Connection fails:**
Verify your kubeconfig works with kubectl:
```bash
kubectl --kubeconfig=/path/to/config --context=your-context get pods -n your-namespace
```

**Upload or download fails silently:**
Increase the write timeout for large files:
```bash
WRITE_TIMEOUT=300 ./kube-browser
```

---

## License

MIT License. See [LICENSE](LICENSE) for details.

---

<p align="center">
  Built with Go and client-go. No Electron, no Docker, no nonsense.
</p>
