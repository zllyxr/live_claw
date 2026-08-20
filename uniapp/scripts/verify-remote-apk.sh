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
echo "$entries" | grep -q '^lib/arm64-v8a/librustdesk\.so$' || {
  echo "APK is missing lib/arm64-v8a/librustdesk.so" >&2
  exit 1
}
if echo "$entries" | grep -E '^lib/(armeabi-v7a|x86|x86_64)/' >/dev/null; then
  echo "APK contains an unsupported non-ARM64 ABI" >&2
  exit 1
fi

unzip -q "$apk" 'lib/arm64-v8a/*.so' -d "$temp_dir"
"$alignment_check" "$temp_dir"/lib/arm64-v8a/*.so

apkanalyzer_bin="${APKANALYZER_BIN:-${ANDROID_HOME:-}/cmdline-tools/latest/bin/apkanalyzer}"
apksigner_bin="${APKSIGNER_BIN:-${ANDROID_HOME:-}/build-tools/36.0.0/apksigner}"
[ -x "$apkanalyzer_bin" ] || { echo "set APKANALYZER_BIN to Android apkanalyzer" >&2; exit 1; }
[ -x "$apksigner_bin" ] || { echo "set APKSIGNER_BIN to Android 36 apksigner" >&2; exit 1; }

manifest="$($apkanalyzer_bin manifest print "$apk")"
echo "$manifest" | grep -q 'android.permission.FOREGROUND_SERVICE_MEDIA_PROJECTION' || { echo "merged Manifest lacks mediaProjection FGS permission" >&2; exit 1; }
echo "$manifest" | grep -q 'com.claw.remote.RemoteHostService' || { echo "merged Manifest lacks RemoteHostService" >&2; exit 1; }
echo "$manifest" | grep -q 'com.claw.remote.RemoteInputService' || { echo "merged Manifest lacks accessibility service" >&2; exit 1; }
echo "$manifest" | grep -q 'targetSdkVersion="36"' || { echo "APK targetSdkVersion is not 36" >&2; exit 1; }

"$apksigner_bin" verify --verbose --print-certs "$apk" >/dev/null
echo "Signed ARM64 remote-assistance APK passed Manifest and 16 KB checks"
