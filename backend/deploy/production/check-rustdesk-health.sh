#!/bin/sh
set -eu

for port in 21115 21116 21117; do
  if ! nc -z -w 3 127.0.0.1 "$port"; then
    echo "RustDesk TCP port $port is unavailable" >&2
    exit 1
  fi
done

if ! curl -fsS --max-time 5 -o /dev/null http://127.0.0.1:21114/; then
  echo "RustDesk Pro console is unavailable on loopback" >&2
  exit 1
fi

echo "RustDesk hbbs, hbbr and local Pro console are healthy"
