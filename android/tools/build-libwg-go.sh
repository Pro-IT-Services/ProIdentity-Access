#!/usr/bin/env bash
# Rebuilds libwg-go.so with 16 KB page alignment (Android 15+ / Play requirement).
#
# The com.wireguard.android:tunnel AAR ships libwg-go.so linked with 4 KB
# pages (libwg.so and libwg-quick.so are already 16 KB). The copies in
# app/src/main/jniLibs override the AAR's; rerun this whenever the tunnel
# dependency version in app/build.gradle.kts changes.
#
# Mirrors upstream tunnel/tools/libwg-go/Makefile: same sources, the same
# CLOCK_BOOTTIME Go runtime patch (timers keep counting while the phone
# sleeps), plus -z max-page-size=16384 and the UAPI socket in this app's
# own data dir (upstream hardcodes com.wireguard.android).
#
# Needs: git, Go, Android NDK. Works in Git Bash on Windows and on Linux/macOS.
set -euo pipefail

TAG="${TAG:-1.0.20260102}"          # must match com.wireguard.android:tunnel
API="${API:-26}"                    # minSdk
PKG="${PKG:-com.proitservices.proidentity.access}"   # applicationId (UAPI socket dir)
SDK="${ANDROID_SDK_ROOT:-${ANDROID_HOME:-$LOCALAPPDATA/Android/Sdk}}"
NDK="${NDK:-$(ls -d "$SDK"/ndk/* | sort -V | tail -1)}"
HERE="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*) HOST=windows-x86_64; EXE=.exe; P() { cygpath -w "$1"; } ;;
  Darwin) HOST=darwin-x86_64; EXE=; P() { echo "$1"; } ;;
  *) HOST=linux-x86_64; EXE=; P() { echo "$1"; } ;;
esac
TOOLCHAIN="$NDK/toolchains/llvm/prebuilt/$HOST"

git clone -q --depth 1 --branch "$TAG" https://git.zx2c4.com/wireguard-android "$WORK/wg"

# Patched private copy of the Go toolchain.
cp -r "$(go env GOROOT | { read -r r; [ -n "${EXE}" ] && cygpath -u "$r" || echo "$r"; })" "$WORK/goroot"
RT="$WORK/goroot/src/runtime"
sed -i.bak 's/^#define CLOCK_MONOTONIC 1/#define CLOCK_BOOTTIME 7/; s/MOVW\t\$CLOCK_MONOTONIC, R0/MOVW\t$CLOCK_BOOTTIME, R0/' "$RT/sys_linux_arm64.s"
sed -i.bak 's/^#define CLOCK_MONOTONIC\t1/#define CLOCK_BOOTTIME\t7/; s/MOVW\t\$CLOCK_MONOTONIC, R0/MOVW\t$CLOCK_BOOTTIME, R0/' "$RT/sys_linux_arm.s"
sed -i.bak 's|MOVL\t\$1, DI // CLOCK_MONOTONIC|MOVL\t$7, DI // CLOCK_BOOTTIME|' "$RT/sys_linux_amd64.s"
sed -i.bak 's|MOVL\t\$1, 0(SP)\t// CLOCK_MONOTONIC|MOVL\t$7, 0(SP)\t// CLOCK_BOOTTIME|; s|MOVL\t\$1, BX\t\t// CLOCK_MONOTONIC|MOVL\t$7, BX\t\t// CLOCK_BOOTTIME|' "$RT/sys_linux_386.s"
for f in arm arm64 amd64 386; do
  if grep -q 'CLOCK_MONOTONIC' "$RT/sys_linux_$f.s" || ! grep -q 'CLOCK_BOOTTIME' "$RT/sys_linux_$f.s"; then
    echo "Go runtime BOOTTIME patch did not apply to sys_linux_$f.s" >&2; exit 1
  fi
done

export GOROOT="$(P "$WORK/goroot")" GOOS=android CGO_ENABLED=1
export CC="$(P "$TOOLCHAIN/bin/clang$EXE")"
export CGO_CFLAGS_ALLOW='--target=.*|--sysroot=.*' CGO_LDFLAGS_ALLOW='--target=.*|--sysroot=.*|-Wl,.*'
SYSROOT="$(P "$TOOLCHAIN/sysroot")"

cd "$WORK/wg/tunnel/tools/libwg-go"
for spec in "arm64 aarch64-linux-android arm64-v8a" "amd64 x86_64-linux-android x86_64" \
            "arm armv7a-linux-androideabi armeabi-v7a" "386 i686-linux-android x86"; do
  set -- $spec
  export GOARCH=$1 GOARM=7
  export CGO_CFLAGS="--target=$2$API --sysroot=$SYSROOT -O2 $([ "$1" = arm ] && echo -marm)"
  export CGO_LDFLAGS="--target=$2$API --sysroot=$SYSROOT -Wl,--build-id=none -Wl,-z,max-page-size=16384 -Wl,-soname=libwg-go.so"
  mkdir -p "$HERE/app/src/main/jniLibs/$3"
  "$WORK/goroot/bin/go$EXE" build -tags linux \
    -ldflags="-X golang.zx2c4.com/wireguard/ipc.socketDirectory=/data/data/$PKG/cache/wireguard -buildid=" \
    -trimpath -buildvcs=false -buildmode c-shared -o "$HERE/app/src/main/jniLibs/$3/libwg-go.so"
  rm -f "$HERE/app/src/main/jniLibs/$3/libwg-go.h"
  echo "built $3/libwg-go.so"
done
