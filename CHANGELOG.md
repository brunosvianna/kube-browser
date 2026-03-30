# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
### Changed
### Fixed
### Security

---

## [1.0.14] - 2026-03-30

### Added
- **Native OS file picker for kubeconfig** — the Browse button now opens the operating
  system's native file dialog (Windows Explorer, Finder, etc.) instead of the custom
  web modal. The selected file is uploaded to the server, validated, and its path
  is populated in the connection form automatically.

### Changed
- Go toolchain updated to **1.25** across `go.mod`, CI workflow, and release workflow.

---

## [1.0.13] - 2026-03-30

### Security
- **Shell injection fix** — `listFilesFind` no longer passes the container path via
  `sh -c "find '<path>' ..."`. The `find` command and all its arguments are now passed
  as a Go `[]string` slice directly to the Kubernetes exec API, eliminating any risk
  of shell metacharacter injection from paths that contain single quotes or other
  special characters.

### Added
- **Read-only mode** — set `KUBE_BROWSER_READ_ONLY=true` (or `=1`) to disable all
  write operations server-side. The `UploadFileHandler` returns HTTP 405 with a JSON
  error body when read-only mode is active; all other read endpoints are unaffected.
- **UI read-only badge** — when read-only mode is active, a lock-icon "Read-only" badge
  appears in the header and the upload button is permanently disabled (even after a PVC
  is selected).
- **Status API exposes `readOnly` flag** — `GET /api/status` now includes
  `"readOnly": true/false` so clients and scripts can detect the mode programmatically.
- **Helper pod: `imagePullSecrets`** — set `KUBE_BROWSER_IMAGE_PULL_SECRET` to the
  name of an existing secret in the target namespace to authenticate against private
  container registries.
- **Helper pod: `serviceAccountName`** — set `KUBE_BROWSER_SERVICE_ACCOUNT` to run
  the helper pod under a specific Kubernetes service account (useful when OPA/Gatekeeper
  requires it).
- **Helper pod: `nodeSelector`** — set `KUBE_BROWSER_NODE_SELECTOR` to constrain helper
  pods to specific node pools (accepts `key=value,key=value` or a JSON object).
- **Helper pod: `tolerations`** — set `KUBE_BROWSER_TOLERATIONS` to a JSON array of
  Kubernetes Toleration objects so the helper pod can co-locate with tainted nodes
  (e.g. GPU, spot, or dedicated nodes).
- **Helper pod: extra labels** — set `KUBE_BROWSER_EXTRA_LABELS` (`key=value,…`) to
  attach additional labels, satisfying OPA label-requirement policies.
- **Helper pod: extra annotations** — set `KUBE_BROWSER_EXTRA_ANNOTATIONS`
  (`key=value,…`) to add annotations for tools such as Vault injector or Datadog APM.
- **`find` + BusyBox fallback without shell** — when `find -exec stat` fails because
  `stat` is unavailable, the binary retries with a plain `find -print` call. Both calls
  are shell-free Go exec slices.
- Unit tests for `parseKeyValuePairs` (11 cases), `parseTolerations` (7 cases), and
  read-only mode (upload blocked, env-var parsing, status response field).
- README: new section "Helper Pod — cluster-specific configuration" with a full variable
  table and a worked example for a restricted cluster (private registry + GPU taint).
- README: new "Read-only mode" section with env var table and behavior description.

---

## [1.0.11] - 2026-03-30

### Added
- Categorized operational errors with distinct types: `RBAC`, `NoShell`, `Timeout`,
  `HelperPending`, `PathNotFound`, `PermDenied` — each triggers a specific, actionable
  message in the UI instead of a generic error toast.
- Unit test suite (30 tests) covering: path sanitization, path construction, `ls`/`find`
  output parsing (GNU coreutils, BusyBox, symlinks, malformed lines), error classification,
  and the full exec fallback chain.
- `PodExecutor` interface (`pkg/k8s/executor.go`) enabling deterministic mock injection
  for all Kubernetes exec and pod lifecycle tests.
- `mockPodExecutor` test helper with queued exec results and create/delete call counters.
- End-to-end tests for the helper pod fallback path: direct exec failure → helper pod
  creation → file listing → helper pod deletion.
- Tests for timeout propagation and helper pod stuck-in-Pending detection.

### Changed
- Upload and download now stream data via `io.Copy` — large files no longer buffer
  entirely in memory before transfer.
- Helper pod names are now unique per operation (UUID suffix), preventing conflicts
  when multiple operations run concurrently.
- Helper pod cleanup now runs at startup to remove any pods orphaned by a previous crash.

### Security
- Helper pods are created with explicit `securityContext` (`runAsNonRoot`, `readOnlyRootFilesystem`,
  `allowPrivilegeEscalation: false`) and CPU/memory resource limits.

---

## [1.0.10] - 2026-03-30

### Security
- Server now binds to `127.0.0.1` by default instead of `0.0.0.0`, preventing
  unintended network exposure.
- Added configurable read/write timeouts to the HTTP server.
- Graceful shutdown on `SIGTERM`/`SIGINT` with a configurable drain period.
- `/api/browse` endpoint restricted to requests originating from localhost via
  middleware; external requests are rejected with `403 Forbidden`.

---

## [1.0.9] - 2026-02-25

### Fixed
- Windows path separators (`\`) were breaking container exec commands; paths are
  now normalized to forward slashes before being sent to the container.

---

## [1.0.8] - 2026-02-25

### Added
- WSL (Windows Subsystem for Linux) detection: when running inside WSL, the tool
  launches the Windows default browser (`cmd.exe /c start`) instead of a Linux browser.

---

## [1.0.7] - 2026-02-25

### Fixed
- Browser auto-open on Linux now uses a reliable detection chain: checks for
  `xdg-open`, `gnome-open`, snap browsers, flatpak browsers, and falls back to
  a Python-based launcher — avoids silent failures on headless or minimal installs.

---

## [1.0.6] - 2026-02-25

### Fixed
- Further improvements to browser detection logic on Linux environments where
  `DISPLAY` or `WAYLAND_DISPLAY` may not be set.

---

## [1.0.5] - 2026-02-25

### Added
- Helper pod fallback: when a container does not have `ls`, `find`, or a shell,
  a temporary BusyBox pod is scheduled on the same node as the PVC owner and used
  to list files. The pod is deleted automatically after the operation.
- Helper pod is scheduled on the same node as the PVC-owning pod to ensure
  `ReadWriteOnce` volumes can be mounted without conflicts.

---

## [1.0.3] - 2026-02-25

### Added
- File browser UI: directory tree, file listing with size and modification time,
  upload and download actions.
- PVC listing now works with Alpine and BusyBox containers that use non-GNU `ls`
  output formats.

---

## [1.0.2] - 2026-02-25

### Changed
- Application opens as a native-style app window (using `--app=` flag in Chromium-based
  browsers) instead of a regular browser tab.

---

## [1.0.1] - 2026-02-25

### Added
- Connection modal on first launch: select kubeconfig file, context, and namespace
  before connecting to a cluster.

---

## [1.0.0] - 2026-02-25

### Added
- Initial release of KubeBrowser.
- Web-based PVC File Manager: browse directories inside Kubernetes PVCs from a
  local browser UI.
- Connects to clusters via kubeconfig (same credentials used by `kubectl`).
- Single self-contained binary; no cluster-side installation required.
- Cross-platform builds: Linux (amd64/arm64), macOS (Intel/Apple Silicon),
  Windows (amd64), and WSL.
- GitHub Actions CI pipeline publishing versioned release archives on every tag.

[Unreleased]: https://github.com/brunosvianna/kube-browser/compare/v1.0.14...HEAD
[1.0.14]: https://github.com/brunosvianna/kube-browser/compare/v1.0.13...v1.0.14
[1.0.13]: https://github.com/brunosvianna/kube-browser/compare/v1.0.11...v1.0.13
[1.0.11]: https://github.com/brunosvianna/kube-browser/compare/v1.0.10...v1.0.11
[1.0.10]: https://github.com/brunosvianna/kube-browser/compare/v1.0.9...v1.0.10
[1.0.9]: https://github.com/brunosvianna/kube-browser/compare/v1.0.8...v1.0.9
[1.0.8]: https://github.com/brunosvianna/kube-browser/compare/v1.0.7...v1.0.8
[1.0.7]: https://github.com/brunosvianna/kube-browser/compare/v1.0.6...v1.0.7
[1.0.6]: https://github.com/brunosvianna/kube-browser/compare/v1.0.5...v1.0.6
[1.0.5]: https://github.com/brunosvianna/kube-browser/compare/v1.0.3...v1.0.5
[1.0.3]: https://github.com/brunosvianna/kube-browser/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/brunosvianna/kube-browser/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/brunosvianna/kube-browser/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/brunosvianna/kube-browser/releases/tag/v1.0.0
