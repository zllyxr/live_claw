#!/bin/sh
set -eu

if [ "$#" -lt 1 ]; then
  echo "usage: check-16k-elf.sh <shared-library> [...]" >&2
  exit 2
fi

for library in "$@"; do
  [ -f "$library" ] || { echo "missing shared library: $library" >&2; exit 1; }
  alignments="$(readelf -lW "$library" | awk '$1 == "LOAD" { print $NF }')"
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
