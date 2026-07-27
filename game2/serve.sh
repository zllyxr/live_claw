#!/usr/bin/env sh
set -eu

game2_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec python3 -m http.server "${PORT:-4173}" --bind 0.0.0.0 --directory "$game2_dir"
