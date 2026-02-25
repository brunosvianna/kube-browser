<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go&logoColor=white" alt="Go Version"/>
  <img src="https://img.shields.io/badge/Kubernetes-client--go-326CE5?style=flat&logo=kubernetes&logoColor=white" alt="Kubernetes"/>
  <img src="https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey?style=flat" alt="Platform"/>
  <img src="https://img.shields.io/github/license/brunosvianna/kube-browser?style=flat" alt="License"/>
</p>

# KubeBrowser

**A desktop file manager for Kubernetes Persistent Volume Claims.**

KubeBrowser is a single binary that launches a web-based IDE for browsing, downloading, and uploading files stored in Kubernetes PVCs. No installation, no dependencies — just run the binary and manage your persistent data.

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

KubeBrowser uses the Kubernetes API (`client-go`) to interact with your cluster:

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
│ KubeBrowser │────>│  Kubernetes  │────>│   Pod / Helper  │
│  (binary)   │<────│     API      │<────│   Pod (alpine)  │
└─────────────┘     └──────────────┘     └─────┬───────────┘
       │                                       │
       │            Web UI (embedded)          PVC
       └──> Browser ──────────────────>   /mounted/path
```

- **File listing** uses `kubectl exec` to run `ls` inside the pod that mounts the PVC.
- **Download** streams file content via `cat` through the Kubernetes exec API.
- **Upload** writes file content via `tee` through the Kubernetes exec API.

### Helper Pod Fallback

Some container images (like Redis, RabbitMQ, or distroless images) don't include basic shell tools. When KubeBrowser detects this, it automatically:

1. Creates a temporary `alpine:3.19` pod that mounts the same PVC
2. Runs file operations through the helper pod
3. Deletes the helper pod when done

This happens transparently — no manual intervention needed.

---

## Configuration

### Port

By default, KubeBrowser listens on port `5000`. Override it with the `PORT` environment variable:

```bash
PORT=8080 ./kube-browser
```

### Kubeconfig

KubeBrowser auto-detects your kubeconfig from:

1. The `KUBECONFIG` environment variable
2. `~/.kube/config` (default path)

You can also browse and select any kubeconfig file through the UI.

---

## Building from Source

### Prerequisites

- Go 1.21 or later

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
│   │   └── handlers.go      # HTTP API handlers
│   └── k8s/
│       └── client.go        # Kubernetes client, PVC/file operations
├── go.mod
└── go.sum
```

**Key design decisions:**

- **Single binary** — All frontend assets (HTML, CSS, JS) are embedded using Go's `embed` package. No external files needed.
- **No dependencies at runtime** — No Node.js, no Docker, no kubectl required. Just the binary and a kubeconfig.
- **Zero configuration** — Sensible defaults, everything configurable through the UI.

---

## Requirements

- A valid kubeconfig with access to the target Kubernetes cluster
- RBAC permissions: `get`, `list` on `pods`, `persistentvolumeclaims`, `namespaces`; `create` for `pods/exec`
- For the helper pod fallback: `create`, `delete` on `pods`

---

## Security

- KubeBrowser runs **locally** on your machine — it does not expose your cluster to the internet.
- Path traversal protection is enforced on all file operations.
- The binary only communicates with the Kubernetes API using your existing kubeconfig credentials.

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
The container doesn't have shell tools. KubeBrowser will try to create a helper pod automatically. Make sure your RBAC permissions allow creating pods in the target namespace.

**Connection fails:**
Verify your kubeconfig works with kubectl:
```bash
kubectl --kubeconfig=/path/to/config --context=your-context get pods -n your-namespace
```

---

## License

MIT License. See [LICENSE](LICENSE) for details.

---

<p align="center">
  Built with Go and client-go. No Electron, no Docker, no nonsense.
</p>
