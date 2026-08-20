#!/bin/sh
set -eu

script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
sdk_dir="$(dirname "$script_dir")"
workspace_dir="$(dirname "$(dirname "$sdk_dir")")"
lock_file="$sdk_dir/rustdesk-upstream.lock"

if [ "${RUSTDESK_COMMERCIAL_EMBEDDING_LICENSE_ACK:-}" != "accepted" ]; then
  echo "Set RUSTDESK_COMMERCIAL_EMBEDDING_LICENSE_ACK=accepted only after written proprietary embedding authorization is on file" >&2
  exit 1
fi

source_dir="${RUSTDESK_SOURCE_DIR:-}"
[ -d "$source_dir/.git" ] || { echo "RUSTDESK_SOURCE_DIR must point to the licensed RustDesk fork" >&2; exit 1; }

expected_upstream="$(sed -n 's/^RUSTDESK_COMMIT=//p' "$lock_file")"
licensed_commit="$(sed -n 's/^LICENSED_FORK_COMMIT=//p' "$lock_file")"
case "$licensed_commit" in
  ''|*[!0-9a-f]*) echo "LICENSED_FORK_COMMIT must be replaced with the approved lowercase hexadecimal fork commit" >&2; exit 1 ;;
esac
[ "${#licensed_commit}" -eq 40 ] || { echo "LICENSED_FORK_COMMIT must contain exactly 40 characters" >&2; exit 1; }
actual_commit="$(git -C "$source_dir" rev-parse HEAD)"
[ "$actual_commit" = "$licensed_commit" ] || {
  echo "RustDesk source is $actual_commit; expected approved licensed fork $licensed_commit" >&2
  exit 1
}
git -C "$source_dir" merge-base --is-ancestor "$expected_upstream" "$actual_commit" || {
  echo "licensed fork is not based on pinned RustDesk upstream $expected_upstream" >&2
  exit 1
}
[ -z "$(git -C "$source_dir" status --porcelain --untracked-files=no)" ] || {
  echo "licensed RustDesk fork has uncommitted tracked changes" >&2
  exit 1
}

case "${ANDROID_NDK_HOME:-}" in
  *28.2.13676358*) ;;
  *) echo "ANDROID_NDK_HOME must point to NDK r28c (28.2.13676358)" >&2; exit 1 ;;
esac

adapter="$source_dir/claw-host-adapter/src/main/java/com/claw/remote/generated/RustDesk149Adapter.kt"
[ -f "$adapter" ] || { echo "licensed fork is missing the adapter defined by ADAPTER_CONTRACT.md" >&2; exit 1; }

expected_rust="$(sed -n 's/^RUST_VERSION=//p' "$lock_file")"
rust_version="$(rustc --version | awk '{print $2}')"
[ "$rust_version" = "$expected_rust" ] || { echo "rustc $expected_rust is required; found $rust_version" >&2; exit 1; }
expected_cargo_ndk="$(sed -n 's/^CARGO_NDK_VERSION=//p' "$lock_file")"
cargo_ndk_version="$(cargo ndk --version 2>/dev/null | awk '{print $NF}')"
[ "$cargo_ndk_version" = "$expected_cargo_ndk" ] || { echo "cargo-ndk $expected_cargo_ndk is required; found ${cargo_ndk_version:-missing}" >&2; exit 1; }

(cd "$source_dir" && sh flutter/ndk_arm64.sh)
rust_library="$source_dir/target/aarch64-linux-android/release/librustdesk.so"
[ -f "$rust_library" ] || { echo "RustDesk ARM64 library was not produced" >&2; exit 1; }
"$script_dir/check-16k-elf.sh" "$rust_library"

mkdir -p "$sdk_dir/host-sdk/src/main/jniLibs/arm64-v8a"
mkdir -p "$sdk_dir/host-sdk/src/main/java/com/claw/remote/generated"
cp "$rust_library" "$sdk_dir/host-sdk/src/main/jniLibs/arm64-v8a/librustdesk.so"
cp "$adapter" "$sdk_dir/host-sdk/src/main/java/com/claw/remote/generated/RustDesk149Adapter.kt"

gradle_bin="${GRADLE_BIN:-gradle}"
(cd "$sdk_dir" && "$gradle_bin" --no-daemon :host-sdk:clean :host-sdk:assembleRelease)

aar="$sdk_dir/host-sdk/build/outputs/aar/host-sdk-release.aar"
destination="$workspace_dir/uniapp/src/uni_modules/claw-rustdesk-host/utssdk/app-android/libs/claw-rustdesk-host-1.0.0.aar"
[ -f "$aar" ] || { echo "Host SDK AAR was not produced" >&2; exit 1; }
mkdir -p "$(dirname "$destination")"
cp "$aar" "$destination"

echo "Built pinned licensed Host SDK: $destination"
