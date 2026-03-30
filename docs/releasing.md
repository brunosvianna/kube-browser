# Release Process

This document describes the release workflow for KubeBrowser.

---

## Overview

KubeBrowser follows [Semantic Versioning](https://semver.org/) and documents all user-visible
changes in `CHANGELOG.md` using the [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format.

Every release goes through three steps:
1. Update `CHANGELOG.md`
2. Push a version tag to GitHub
3. CI builds the binaries and publishes the GitHub Release automatically

---

## Versioning Rules

| Type | When to use | Example |
|---|---|---|
| **Patch** `x.y.Z` | Bug fixes, small improvements, no new features | `1.0.10 → 1.0.11` |
| **Minor** `x.Y.0` | New features that are backward-compatible | `1.0.11 → 1.1.0` |
| **Major** `X.0.0` | Breaking changes (config format, CLI flags, API) | `1.1.0 → 2.0.0` |

When in doubt, use a patch. Promote to minor when the feature is something a user
would explicitly seek out. Reserve major for changes that require the user to adapt.

---

## Step 1 — Update CHANGELOG.md

During development, add entries under `## [Unreleased]` as changes land:

```markdown
## [Unreleased]

### Added
- Upload progress indicator in the file browser UI.

### Fixed
- Helper pod cleanup now retries on transient API errors.
```

Use the four standard sections — only include sections that have entries:

| Section | Use for |
|---|---|
| `Added` | New features, new endpoints, new CLI flags |
| `Changed` | Modified behavior that users will notice |
| `Fixed` | Bug fixes (reference the symptom, not the internal cause) |
| `Security` | Vulnerability fixes, auth changes, exposure mitigations |

**Writing good entries:**
- Write for the user, not for the reviewer. Describe what changes for them.
- One sentence per entry is usually enough.
- Avoid copying raw commit messages — they are often too terse or too internal.
- Do not list refactors, test additions, or CI changes unless they affect the user.

**Bad entry:**
```
- refactor: extract PodExecutor interface for testability
```

**Good entry:**
```
- File listing now falls back to a helper pod when the container lacks shell tools (Alpine, distroless).
```

---

## Step 2 — Move [Unreleased] to a new version

When ready to release, rename `[Unreleased]` to the new version with today's date:

```markdown
## [1.1.0] - 2026-04-15

### Added
- Upload progress indicator in the file browser UI.

### Fixed
- Helper pod cleanup now retries on transient API errors.

---

## [Unreleased]

### Added
### Changed
### Fixed
### Security
```

Then update the comparison links at the bottom of the file:

```markdown
[Unreleased]: https://github.com/brunosvianna/kube-browser/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/brunosvianna/kube-browser/compare/v1.0.11...v1.1.0
```

---

## Step 3 — Tag and push

Commit the updated `CHANGELOG.md`:

```bash
git add CHANGELOG.md
git commit -m "chore: release v1.1.0"
git tag -a v1.1.0 -m "v1.1.0: <one-line summary of highlights>"
git push origin main
git push origin v1.1.0
```

The CI pipeline triggers on the tag push, builds binaries for all platforms, and
publishes the GitHub Release. The release body is extracted from the `CHANGELOG.md`
section for that version.

---

## Release Highlights

Each release should have a short human summary — one or two sentences at the top
of the CHANGELOG section — before the bullet list. This helps users scan releases
quickly without reading every entry.

Example:

```markdown
## [1.1.0] - 2026-04-15

This release adds upload progress feedback and improves reliability of helper pod
cleanup in environments with flaky API servers.

### Added
- Upload progress indicator in the file browser UI.

### Fixed
- Helper pod cleanup now retries on transient API errors.
```

---

## Example: What v1.0.12 might look like

If the next patch fixes a bug where the file browser crashes on empty directories:

```markdown
## [1.0.12] - 2026-04-01

### Fixed
- File browser no longer shows a blank screen when navigating to an empty directory.
```

Changelog comparison link to add:

```markdown
[1.0.12]: https://github.com/brunosvianna/kube-browser/compare/v1.0.11...v1.0.12
```

---

## Hotfix releases

For urgent fixes on the latest stable version:

1. Branch off the latest tag: `git checkout -b hotfix/v1.0.12 v1.0.11`
2. Apply the fix and add a `CHANGELOG.md` entry
3. Tag `v1.0.12` and push
4. Merge back to `main`

---

## What NOT to include in the changelog

- Internal refactors with no user-visible effect
- Test additions or coverage improvements
- CI/CD workflow changes
- Dependency updates (unless they fix a security vulnerability)
- Commits that were reverted before release
