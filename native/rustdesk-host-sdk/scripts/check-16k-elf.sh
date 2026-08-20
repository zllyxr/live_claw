#!/bin/sh
set -eu

if [ "$#" -lt 1 ]; then
  echo "usage: check-16k-elf.sh <shared-library> [...]" >&2
  exit 2
fi

readelf_bin="${READELF_BIN:-}"
if [ -z "$readelf_bin" ]; then
  if command -v readelf >/dev/null 2>&1; then
    readelf_bin="readelf"
  else
    android_sdk="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-/Users/mac/Library/Android/sdk}}"
    readelf_bin="$(find "$android_sdk/ndk" -path '*/toolchains/llvm/prebuilt/*/bin/llvm-readelf' \( -type f -o -type l \) 2>/dev/null | sort | tail -1)"
  fi
fi
[ -n "$readelf_bin" ] && [ -x "$readelf_bin" ] || { echo "readelf was not found" >&2; exit 1; }
for library in "$@"; do
  [ -f "$library" ] || { echo "missing shared library: $library" >&2; exit 1; }
  alignments="$($readelf_bin -lW "$library" | awk '$1 == "LOAD" { print $NF }')"
  [ -n "$alignments" ] || { echo "no LOAD segments: $library" >&2; exit 1; }
  for alignment in $alignments; do
    value=$((alignment))
    if [ "$value" -lt 16384 ]; then
      echo "$library contains LOAD alignment $alignment below 0x4000" >&2
      exit 1
    fi
  done
done

echo "All native libraries satisfy the 16 KB LOAD alignment gate"
