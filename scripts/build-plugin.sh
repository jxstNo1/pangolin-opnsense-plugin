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
cp plugin/Makefile plugin/pkg-descr plugin/+PRE_DEINSTALL plugin/+POST_INSTALL plugin/+POST_DEINSTALL dist/"$NAME"/
cp LICENSE README.md CHANGELOG.md dist/"$NAME"/

cd dist
tar -czf "$NAME.tgz" "$NAME"
if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$NAME.tgz" > "$NAME.tgz.sha256"
elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$NAME.tgz" > "$NAME.tgz.sha256"
else
    sha256 -r "$NAME.tgz" > "$NAME.tgz.sha256"
fi
echo "created dist/$NAME.tgz"
