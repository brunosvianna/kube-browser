# Contributing to KubeBrowser

Thank you for your interest in contributing to KubeBrowser. This document covers everything you need to get started.

---

## Table of Contents

- [Scope](#scope)
- [Getting Started](#getting-started)
- [Running Tests](#running-tests)
- [Commit Conventions](#commit-conventions)
- [Pull Request Process](#pull-request-process)
- [Reporting Bugs](#reporting-bugs)

---

## Scope

KubeBrowser is a **local developer tool** for browsing and managing files in Kubernetes PVCs through a web UI. It runs on your machine and connects to your cluster using your existing kubeconfig.

**In scope:**
- Improvements to file listing, upload, download reliability
- Better error messages and user feedback
- Support for more container runtimes (distroless, Windows containers)
- Performance improvements (streaming, caching)
- Helper pod configurability for restricted clusters
- Cross-platform fixes (Windows, WSL, macOS)

**Out of scope:**
- Features that require deploying server-side components to the cluster
- Multi-user access or authentication layers
- Persistent state beyond the current session
- Supporting Kubernetes versions older than 1.26

If you are unsure whether your idea fits, open a [Feature Request](https://github.com/brunosvianna/kube-browser/issues/new/choose) first before writing code.

---

## Getting Started

**Requirements:**
- Go 1.25+
- A kubeconfig pointing to a Kubernetes cluster (for integration testing)
- `kubectl` available in PATH (optional, for manual validation)

**Clone and build:**

```bash
git clone https://github.com/brunosvianna/kube-browser.git
cd kube-browser
go build -o kube-browser ./cmd/kube-browser/
./kube-browser
```

The server starts on `http://127.0.0.1:5000` and opens your browser automatically.

**Environment variables:**

| Variable | Default | Description |
|---|---|---|
| `PORT` | `5000` | HTTP server port |
| `HOST` | `127.0.0.1` | Bind address (keep as localhost unless you know what you're doing) |
| `KUBE_BROWSER_READ_ONLY` | unset | Set to `true` to disable all write operations |

---

## Running Tests

The unit test suite covers parsers, path handling, error classification, and the exec fallback chain — all without a real cluster:

```bash
go test ./... -timeout 120s
```

To run with the race detector (recommended before submitting a PR):

```bash
go test ./... -timeout 120s -race
```

To run a specific package:

```bash
go test ./pkg/k8s/... -v
go test ./pkg/handlers/... -v
```

See [docs/testing.md](docs/testing.md) for a complete breakdown of what the tests cover and how to manually validate cluster-dependent behavior.

---

## Commit Conventions

Use the [Conventional Commits](https://www.conventionalcommits.org/) format:

```
<type>(<scope>): <short description>

[optional body]
```

| Type | Use for |
|---|---|
| `feat` | New user-visible feature |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `test` | Tests only |
| `chore` | Build, CI, tooling |
| `refactor` | Internal restructuring without behavior change |

**Examples:**

```
feat(helper-pod): support imagePullSecrets via env var
fix(browse): fall back to home dir when kubeconfig path does not exist
docs(readme): add namespace-scoped RBAC example
```

Keep the subject line under 72 characters. Use the body to explain *why*, not *what*.

---

## Pull Request Process

1. **Fork and branch** — create a branch from `main` with a descriptive name (`feat/read-only-mode`, `fix/windows-path-separator`).

2. **Keep PRs focused** — one feature or fix per PR. Large changes are harder to review and more likely to conflict.

3. **Update the changelog** — add an entry under `## [Unreleased]` in `CHANGELOG.md` if your change is user-visible. See [docs/releasing.md](docs/releasing.md) for the format.

4. **Run the full test suite** before opening the PR:
   ```bash
   go test ./... -race && go vet ./...
   ```

5. **Fill in the PR template** — the template will appear automatically when you open a PR. Answer all sections.

6. **Expect review feedback** — maintainers may ask for changes. Please respond to comments or let us know if you need help.

---

## Reporting Bugs

Use the [Bug Report template](https://github.com/brunosvianna/kube-browser/issues/new/choose). Include the KubeBrowser version, your OS, your Kubernetes environment, and steps to reproduce. The more detail you provide, the faster we can help.
