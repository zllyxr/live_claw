#!/bin/sh
set -eu

image="${RUSTDESK_SERVER_PRO_IMAGE:-}"
case "$image" in
  docker.io/rustdesk/rustdesk-server-pro:*@sha256:*) ;;
  *)
    echo "RUSTDESK_SERVER_PRO_IMAGE must be a versioned docker.io/rustdesk/rustdesk-server-pro image with @sha256 digest" >&2
    exit 1
    ;;
esac

case "$image" in
  *:latest@*)
    echo "RUSTDESK_SERVER_PRO_IMAGE must not use latest" >&2
    exit 1
    ;;
esac

case "$image" in
  *REPLACE*) echo "RUSTDESK_SERVER_PRO_IMAGE still contains a placeholder" >&2; exit 1 ;;
esac

digest="${image##*@sha256:}"
case "$digest" in
  *[!0-9a-f]*) echo "RustDesk image digest must be lowercase hexadecimal" >&2; exit 1 ;;
esac
if [ "${#digest}" -ne 64 ]; then
  echo "RustDesk image digest must contain exactly 64 hexadecimal characters" >&2
  exit 1
fi

case "${RUSTDESK_DATA_DIR:-/opt/claw-rustdesk/data}" in
  /opt/claw-rustdesk/data|/srv/claw-rustdesk/data) ;;
  *)
    echo "RUSTDESK_DATA_DIR must be an approved dedicated persistent directory" >&2
    exit 1
    ;;
esac

echo "RustDesk Server Pro deployment inputs are immutable and scoped"
