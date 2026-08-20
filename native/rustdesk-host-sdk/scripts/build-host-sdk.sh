#!/bin/sh
set -eu

script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
sdk_dir="$(dirname "$script_dir")"
workspace_dir="$(dirname "$(dirname "$sdk_dir")")"
android_sdk="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-/Users/mac/Library/Android/sdk}}"
gradle_bin="${GRADLE_BIN:-gradle}"
[ -d "$android_sdk/platforms" ] || { echo "Android SDK not found: $android_sdk" >&2; exit 1; }

(cd "$sdk_dir" && ANDROID_HOME="$android_sdk" ANDROID_SDK_ROOT="$android_sdk" \
  "$gradle_bin" --no-daemon :host-sdk:clean :host-sdk:assembleRelease)

aar="$sdk_dir/host-sdk/build/outputs/aar/host-sdk-release.aar"
destination="$workspace_dir/uniapp/src/uni_modules/claw-rustdesk-host/utssdk/app-android/libs/claw-remote-host-1.0.0.aar"
[ -f "$aar" ] || { echo "Remote Host SDK AAR was not produced" >&2; exit 1; }
mkdir -p "$(dirname "$destination")"
cp "$aar" "$destination"

echo "Built Android Remote Host SDK: $destination"
