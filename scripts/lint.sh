#!/bin/sh
# Run local checks that do not require an OPNsense/FreeBSD host.
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

cd "$ROOT/agent"
go test ./...
go vet ./...
for arch in amd64 arm64; do
    echo "cross-compiling freebsd/$arch"
    GOOS=freebsd GOARCH="$arch" CGO_ENABLED=0 go build ./...
done

cd "$ROOT"
if command -v php >/dev/null 2>&1; then
    python3 scripts/check-php.py "$(command -v php)"
else
    echo "php not found; skipping PHP lint"
fi
python3 scripts/check-xml.py
echo "checks complete"
