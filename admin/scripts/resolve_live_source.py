#!/usr/bin/env python3
"""Resolve one authorized live page for client-side playback."""

import argparse
import http.cookiejar
import json
import os
import sys
import urllib.parse

from live import DEFAULT_UA, fetch
from live_douyin import DouyinResolver


def parse_args():
    parser = argparse.ArgumentParser(description="Resolve a live page to a direct media URL.")
    parser.add_argument("--page", required=True)
    parser.add_argument("--max-height", type=int, default=720)
    parser.add_argument("--pull-format", choices=["hls", "flv"], default="hls")
    parser.add_argument("--user-agent", default=DEFAULT_UA)
    return parser.parse_args()


def main():
    args = parse_args()
    cookie_jar = http.cookiejar.CookieJar()

    def page_fetch(url, headers=None):
        return fetch(
            url,
            headers=headers,
            maxread=8_000_000,
            cookie_jar=cookie_jar,
        )

    old_random_room = os.environ.get("RANDOM_ROOM")
    old_room_retry = os.environ.get("ROOM_RETRY")
    preferred_room_id = urllib.parse.parse_qs(
        urllib.parse.urlsplit(args.page).query
    ).get("preferred_room_id", [""])[0]
    os.environ["RANDOM_ROOM"] = "0" if preferred_room_id else "1"
    os.environ["ROOM_RETRY"] = "1" if preferred_room_id else "40"
    try:
        candidate = DouyinResolver(page_fetch, args.user_agent).resolve(
            args.page,
            max_height=max(0, args.max_height),
            pull_format=args.pull_format,
        )
    except Exception as exc:
        print(
            json.dumps(
                {"ok": False, "error": f"{type(exc).__name__}: {exc}"},
                ensure_ascii=False,
            )
        )
        return 1
    finally:
        if old_random_room is None:
            os.environ.pop("RANDOM_ROOM", None)
        else:
            os.environ["RANDOM_ROOM"] = old_random_room
        if old_room_retry is None:
            os.environ.pop("ROOM_RETRY", None)
        else:
            os.environ["ROOM_RETRY"] = old_room_retry

    print(
        json.dumps(
            {
                "ok": True,
                "provider": candidate.provider,
                "url": candidate.stream_url,
                "format": candidate.format,
                "height": candidate.height,
                "resolution": candidate.resolution,
                "room_id": candidate.room_id,
                "room_page": candidate.selected_room_page,
                "nickname": candidate.nickname,
                "title": candidate.title,
                "cache_seconds": 30,
            },
            ensure_ascii=False,
        )
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
