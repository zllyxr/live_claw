#!/bin/sh
set -eu

apk="${1:-}"
[ -f "$apk" ] || { echo "usage: verify-remote-apk.sh <signed.apk>" >&2; exit 2; }

script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
workspace_dir="$(dirname "$(dirname "$script_dir")")"
alignment_check="$workspace_dir/native/rustdesk-host-sdk/scripts/check-16k-elf.sh"
temp_dir="$(mktemp -d)"
trap 'rm -rf -- "$temp_dir"' EXIT

entries="$(unzip -Z1 "$apk")"
if echo "$entries" | grep -q 'librustdesk\.so$'; then
  echo "APK unexpectedly contains RustDesk native code" >&2
  exit 1
fi
if echo "$entries" | grep -E '^lib/(armeabi-v7a|x86|x86_64)/' >/dev/null; then
  echo "APK contains an unsupported non-ARM64 ABI" >&2
  exit 1
fi

unzip -q "$apk" 'lib/arm64-v8a/*.so' -d "$temp_dir"
sh "$alignment_check" "$temp_dir"/lib/arm64-v8a/*.so

android_sdk="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-/Users/mac/Library/Android/sdk}}"
latest_build_tools="$(find "$android_sdk/build-tools" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | sort -V | tail -1)"
aapt_bin="${AAPT_BIN:-$latest_build_tools/aapt}"
apksigner_bin="${APKSIGNER_BIN:-$latest_build_tools/apksigner}"
dexdump_bin="${DEXDUMP_BIN:-$latest_build_tools/dexdump}"
[ -x "$aapt_bin" ] || { echo "set AAPT_BIN to Android aapt" >&2; exit 1; }
[ -x "$apksigner_bin" ] || { echo "set APKSIGNER_BIN to Android apksigner" >&2; exit 1; }
[ -x "$dexdump_bin" ] || { echo "set DEXDUMP_BIN to Android dexdump" >&2; exit 1; }

manifest="$($aapt_bin dump xmltree "$apk" AndroidManifest.xml)"
echo "$manifest" | grep -q 'android.permission.FOREGROUND_SERVICE_MEDIA_PROJECTION' || { echo "merged Manifest lacks mediaProjection FGS permission" >&2; exit 1; }
echo "$manifest" | grep -q 'com.claw.remote.RemoteHostService' || { echo "merged Manifest lacks RemoteHostService" >&2; exit 1; }
echo "$manifest" | grep -q 'com.claw.remote.RemoteInputService' || { echo "merged Manifest lacks accessibility service" >&2; exit 1; }
target_sdk="$($aapt_bin dump badging "$apk" | sed -n "s/^targetSdkVersion:'\([^']*\)'.*/\1/p")"
[ "$target_sdk" = "36" ] || { echo "APK targetSdkVersion is not 36" >&2; exit 1; }

unzip -q "$apk" 'classes*.dex' -d "$temp_dir"
host_sdk_found=false
for dex_file in "$temp_dir"/classes*.dex; do
  if "$dexdump_bin" -f "$dex_file" | grep -q 'Lcom/claw/remote/HostSdk;'; then
    host_sdk_found=true
    break
  fi
done
[ "$host_sdk_found" = true ] || { echo "APK is missing the in-house remote Host SDK" >&2; exit 1; }

"$apksigner_bin" verify --verbose --print-certs "$apk" >/dev/null
echo "Signed ARM64 in-app remote-assistance APK passed Manifest and 16 KB checks"
