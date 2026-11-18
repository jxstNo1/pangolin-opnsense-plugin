#!/bin/sh
# Regenerate the upstream-ready patches from pristine tag archives.
#
# Usage:
#   scripts/regen-patches.sh [UPSTREAM_ROOT]
#
# UPSTREAM_ROOT must contain newt/ and olm/ checked out at the pinned tags.
# The default is ../_upstream relative to this repository.
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
UPSTREAM=${1:-"$ROOT/_upstream"}
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

mkdir -p "$TMP/old" "$TMP/new"
for mod in newt olm; do
    cp -R "$UPSTREAM/$mod" "$TMP/old/$mod"
    cp -R "$ROOT/third_party/$mod" "$TMP/new/$mod"
    rm -rf "$TMP/old/$mod/.git" "$TMP/new/$mod/.git"
done

cd "$TMP"
git diff --no-index --src-prefix=a/ --dst-prefix=b/ --output="$TMP/newt.patch" old/newt new/newt || true
git diff --no-index --src-prefix=a/ --dst-prefix=b/ --output="$TMP/olm.patch" old/olm new/olm || true

# Strip the temporary old/ and new/ path components so the patches apply with
# -p1 from a directory containing newt/ and olm/.
sed -e 's#a/old/#a/#g' -e 's#b/new/#b/#g' -e 's#a/new/#a/#g' "$TMP/newt.patch" > "$ROOT/patches/newt/0001-freebsd-network-support.patch"
sed -e 's#a/old/#a/#g' -e 's#b/new/#b/#g' -e 's#a/new/#a/#g' "$TMP/olm.patch" > "$ROOT/patches/olm/0001-freebsd-support.patch"

echo "regenerated:"
wc -l "$ROOT/patches/newt/0001-freebsd-network-support.patch" \
      "$ROOT/patches/olm/0001-freebsd-support.patch"
