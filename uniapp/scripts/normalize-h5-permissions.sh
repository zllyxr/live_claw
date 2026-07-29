#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
uniapp_root=$(CDPATH= cd -- "$script_dir/.." && pwd)

normalize_tree() {
  target=$1
  [ -d "$target" ] || return 0

  find "$target" -type d -exec chmod a+rx {} +
  find "$target" -type f -exec chmod a+r {} +
}

# Static files can arrive with owner-only permissions when copied from an
# archive or generated on another machine. The web server runs as a non-owner
# user, so every directory must be traversable and every file must remain
# world-readable.
normalize_tree "$uniapp_root/src/static"
normalize_tree "$uniapp_root/dist/build/h5"
