#!/bin/sh
# Build a release tarball of the OPNsense plugin. The agent binary is built
# first and is intentionally not stored in git.
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
VERSION=${VERSION:-0.1.0}
ARCH=${ARCH:-amd64}
NAME="os-pangolin-$VERSION-$ARCH"

cd "$ROOT"
VERSION="$VERSION" ARCHS="$ARCH" scripts/build-agent.sh

mkdir -p dist/"$NAME"
cp -R plugin/src dist/"$NAME"/
cp plugin/Makefile plugin/pkg-descr plugin/+POST_INSTALL plugin/+POST_DEINSTALL dist/"$NAME"/
cp LICENSE README.md CHANGELOG.md dist/"$NAME"/

cd dist
tar -czf "$NAME.tgz" "$NAME"
sha256sum "$NAME.tgz" > "$NAME.tgz.sha256" 2>/dev/null || shasum -a 256 "$NAME.tgz" > "$NAME.tgz.sha256"
echo "created dist/$NAME.tgz"
