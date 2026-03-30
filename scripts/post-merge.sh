#!/bin/bash
set -e

echo "Post-merge: verifying Go build..."
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
go build -o /dev/null ./cmd/kube-browser/
echo "Build OK"
