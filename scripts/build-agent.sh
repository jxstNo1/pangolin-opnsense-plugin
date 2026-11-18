#!/bin/sh
# Build the native FreeBSD agent. The plugin package expects the amd64 binary
# at plugin/src/sbin/pangolin-agent; arm64 builds are written to dist/.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
VERSION=${VERSION:-0.1.0}
COMMIT=${COMMIT:-$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || echo unknown)}
DATE=${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}
OUT=${OUT:-"$ROOT/plugin/src/sbin/pangolin-agent"}

cd "$ROOT/agent"
mkdir -p "$(dirname "$OUT")" "$ROOT/dist"

for arch in "${ARCHS:-amd64}"; do
    out=$OUT
    if [ "$arch" != "amd64" ]; then
        out="$ROOT/dist/pangolin-agent-freebsd-$arch"
    fi
    echo "building $out (freebsd/$arch)"
    GOOS=freebsd GOARCH="$arch" CGO_ENABLED=0 \
        go build -trimpath \
        -ldflags "-s -w -X github.com/os-pangolin/os-pangolin/agent/internal/version.Version=$VERSION -X github.com/os-pangolin/os-pangolin/agent/internal/version.Commit=$COMMIT -X github.com/os-pangolin/os-pangolin/agent/internal/version.Date=$DATE" \
        -o "$out" ./cmd/pangolin-agent
done

echo "agent build complete"
