#!/bin/bash
# Builds a self-contained OpenVPN for the macOS package (no Homebrew, no dylibs
# outside the system): OpenSSL, LZ4 and LZO are linked statically.
#
#   tools/build-openvpn-macos.sh            → build/third_party/openvpn-macos/{openvpn,COPYING}
#
# Sources are official releases pinned by SHA-256 (the OpenVPN tarball is also
# GPG-signed by the OpenVPN security team, key F554A3687412CFFEBDEFE0A312F5F7B42F2B01E7).
# OpenVPN is GPLv2 and ships unmodified as a separate program; see
# installer/THIRD-PARTY-NOTICES.txt. Bump a version together with its hash.
set -euo pipefail

OPENVPN_VERSION=2.7.7
OPENVPN_SHA256=3ab8f48fd6c26d49ba2333a092433949afdb5c85c0e6a1ff265784fbc04a2463
OPENSSL_VERSION=3.5.8
OPENSSL_SHA256=a8f84a39918ec6415ce765d9b429d313ba97b8143169c172e734b9514464f5b2
LZ4_VERSION=1.10.0
LZ4_SHA256=537512904744b35e232912055ccf8ec66d768639ff3abe5788d90d792ec5f48b
LZO_VERSION=2.10
LZO_SHA256=c0f892943208266f9b6543b3ae308fab6284c5c90e627931446fb49b4221a072

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$REPO_ROOT/build/third_party/openvpn-macos"
WORK="$REPO_ROOT/build/third_party/openvpn-macos-work"
PREFIX="$WORK/prefix"
MIN_MACOS=12.0
JOBS=$(sysctl -n hw.ncpu)

if [[ -x "$OUT/openvpn" && "${1:-}" != "--force" ]]; then
    echo "  OpenVPN for macOS already built ($OUT)"
    exit 0
fi

rm -rf "$WORK" "$OUT"
mkdir -p "$WORK/src" "$PREFIX" "$OUT"
cd "$WORK/src"

fetch() { # url sha256
    local file; file="$(basename "$1")"
    curl -fsSL "$1" -o "$file"
    echo "$2  $file" | shasum -a 256 -c - >/dev/null || { echo "SHA-256 mismatch: $file" >&2; exit 1; }
    tar -xzf "$file"
}
fetch "https://swupdate.openvpn.org/community/releases/openvpn-$OPENVPN_VERSION.tar.gz" "$OPENVPN_SHA256"
fetch "https://github.com/openssl/openssl/releases/download/openssl-$OPENSSL_VERSION/openssl-$OPENSSL_VERSION.tar.gz" "$OPENSSL_SHA256"
fetch "https://github.com/lz4/lz4/releases/download/v$LZ4_VERSION/lz4-$LZ4_VERSION.tar.gz" "$LZ4_SHA256"
fetch "https://www.oberhumer.com/opensource/lzo/download/lzo-$LZO_VERSION.tar.gz" "$LZO_SHA256"

export MACOSX_DEPLOYMENT_TARGET=$MIN_MACOS
ARCH=$(uname -m)
case "$ARCH" in
    arm64)  SSL_TARGET=darwin64-arm64-cc ;;
    x86_64) SSL_TARGET=darwin64-x86_64-cc ;;
    *) echo "unsupported arch $ARCH" >&2; exit 1 ;;
esac

echo "  OpenSSL $OPENSSL_VERSION"
(cd "openssl-$OPENSSL_VERSION" &&
    ./Configure "$SSL_TARGET" no-shared no-tests no-docs no-apps --prefix="$PREFIX" --libdir=lib >/dev/null &&
    make -j"$JOBS" >/dev/null && make install_sw >/dev/null)

echo "  LZ4 $LZ4_VERSION"
(cd "lz4-$LZ4_VERSION/lib" &&
    make -j"$JOBS" liblz4.a >/dev/null &&
    mkdir -p "$PREFIX/lib" "$PREFIX/include" &&
    cp liblz4.a "$PREFIX/lib/" && cp lz4.h lz4hc.h lz4frame.h "$PREFIX/include/")

echo "  LZO $LZO_VERSION"
(cd "lzo-$LZO_VERSION" &&
    ./configure --prefix="$PREFIX" --disable-shared --enable-static >/dev/null &&
    make -j"$JOBS" >/dev/null && make install >/dev/null)

echo "  OpenVPN $OPENVPN_VERSION"
(cd "openvpn-$OPENVPN_VERSION" &&
    ./configure \
        --disable-shared --enable-static \
        --disable-plugins --disable-plugin-auth-pam --disable-plugin-down-root \
        --disable-pkcs11 --disable-debug \
        CPPFLAGS="-I$PREFIX/include" LDFLAGS="-L$PREFIX/lib" \
        OPENSSL_CFLAGS="-I$PREFIX/include" \
        OPENSSL_LIBS="$PREFIX/lib/libssl.a $PREFIX/lib/libcrypto.a" \
        LZ4_CFLAGS="-I$PREFIX/include" LZ4_LIBS="$PREFIX/lib/liblz4.a" \
        LZO_CFLAGS="-I$PREFIX/include" LZO_LIBS="$PREFIX/lib/liblzo2.a" >/dev/null &&
    make -j"$JOBS" >/dev/null)

cp "openvpn-$OPENVPN_VERSION/src/openvpn/openvpn" "$OUT/openvpn"
cp "openvpn-$OPENVPN_VERSION/COPYING" "$OUT/COPYING"
cp "openvpn-$OPENVPN_VERSION/COPYRIGHT.GPL" "$OUT/COPYRIGHT.GPL"
strip -x "$OUT/openvpn"

# Only system libraries may remain.
if otool -L "$OUT/openvpn" | tail -n +2 | grep -vE "^\s+/(usr/lib|System/Library)/"; then
    echo "openvpn links against non-system libraries" >&2
    exit 1
fi
"$OUT/openvpn" --version | head -2
rm -rf "$WORK"
echo "  Built $OUT/openvpn"
