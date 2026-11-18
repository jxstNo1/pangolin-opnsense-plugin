#!/bin/sh
# Build a FreeBSD/OPNsense .pkg from the release tarball inside the test VM.
set -eu
VERSION=${VERSION:-0.1.0}
ARCH=${ARCH:-amd64}
TARBALL=${TARBALL:-/tmp/os-pangolin-$VERSION-$ARCH.tgz}
STAGE=/tmp/pangolin-pkg
ROOT=$STAGE/root
PKG=/tmp/os-pangolin-$VERSION.pkg

rm -rf "$STAGE"
mkdir -p "$ROOT/usr/local" "$STAGE"
tar -xzf "$TARBALL" -C "$STAGE"
cp -R "$STAGE/os-pangolin-$VERSION-$ARCH/src/"* "$ROOT/usr/local/"
cp "$STAGE/os-pangolin-$VERSION-$ARCH/+PRE_DEINSTALL" "$STAGE/+PRE_DEINSTALL"
cp "$STAGE/os-pangolin-$VERSION-$ARCH/+POST_INSTALL" "$STAGE/+POST_INSTALL"
cp "$STAGE/os-pangolin-$VERSION-$ARCH/+POST_DEINSTALL" "$STAGE/+POST_DEINSTALL"

chmod 755 "$ROOT/usr/local/sbin/pangolin-agent" \
    "$ROOT/usr/local/etc/rc.d/pangolin_client" \
    "$ROOT/usr/local/etc/rc.d/pangolin_site" \
    "$STAGE/+PRE_DEINSTALL" "$STAGE/+POST_INSTALL" "$STAGE/+POST_DEINSTALL"
chmod 700 "$ROOT/usr/local/etc/pangolin/hook.sh"

cat > "$STAGE/+MANIFEST" <<EOF
name: os-pangolin
version: $VERSION
origin: opnsense/os-pangolin
comment: Native Pangolin machine client, site connector and subnet router
desc: Native Pangolin integration for OPNsense and FreeBSD
maintainer: os-pangolin@example.invalid
www: https://github.com/os-pangolin/os-pangolin
prefix: /usr/local
licenselogic: single
licenses: [AGPLv3]
categories: [security]
EOF

(cd "$ROOT/usr/local" && find . -type f | sed 's#^\./##' | sort) > "$STAGE/plist"

# pkg create -o names an output directory, not the final file. -m is the
# metadata directory that contains +MANIFEST and the +*_INSTALL scripts;
# without it the scripts are silently left out of the package.
rm -rf "$PKG"
pkg create -m "$STAGE" -r "$ROOT" -p "$STAGE/plist" -o /tmp
ls -l "$PKG"
pkg info --file "$PKG" | head -20
