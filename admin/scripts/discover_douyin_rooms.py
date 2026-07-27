#!/usr/bin/env python3
"""Discover currently online Douyin rooms without persisting signed stream URLs."""

import argparse
import concurrent.futures
import http.cookiejar
import json
import os
import random
import sys
import urllib.parse

from live import DEFAULT_UA, fetch
from live_douyin import (
    DEFAULT_CATEGORY_PAGES,
    DouyinResolver,
    extract_list_stream_candidates,
    extract_room_pages,
    is_douyin_list_page,
    room_id_from_page,
)


def list_pages_for(source_page):
    pages = [source_page]
    path = urllib.parse.urlsplit(source_page).path.strip("/").lower()
    if path in {"", "hot_live"}:
        categories = list(DEFAULT_CATEGORY_PAGES)
        random.shuffle(categories)
        pages.extend(categories)
    return list(dict.fromkeys(pages))


def fetch_room_links(list_page, user_agent):
    cookie_jar = http.cookiejar.CookieJar()
    html = fetch(
        list_page,
        headers={
            "User-Agent": user_agent,
            "Referer": "https://live.douyin.com/",
            "Accept-Language": "zh-CN,zh;q=0.9",
        },
        maxread=8_000_000,
        cookie_jar=cookie_jar,
    )
    return extract_room_pages(list_page, html)


def collect_room_pages(source_page, user_agent, workers=4):
    if room_id_from_page(source_page):
        return [source_page]
    if not is_douyin_list_page(source_page):
        raise RuntimeError("unsupported douyin page")

    room_pages = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=max(1, workers)) as executor:
        futures = [
            executor.submit(fetch_room_links, page, user_agent)
            for page in list_pages_for(source_page)
        ]
        for future in concurrent.futures.as_completed(futures):
            try:
                discovered = future.result()
            except Exception:
                continue
            for room_page in discovered:
                if room_page not in room_pages:
                    room_pages.append(room_page)
    return room_pages


def fetch_list_streams(list_page, user_agent, max_height, pull_format):
    cookie_jar = http.cookiejar.CookieJar()
    html = fetch(
        list_page,
        headers={
            "User-Agent": user_agent,
            "Referer": "https://live.douyin.com/",
            "Accept-Language": "zh-CN,zh;q=0.9",
        },
        maxread=8_000_000,
        cookie_jar=cookie_jar,
    )
    return [
        {
            "provider": candidate.provider,
            "room_id": candidate.room_id,
            "room_page": candidate.selected_room_page,
            "category_page": list_page,
            "nickname": candidate.nickname,
            "title": candidate.title,
            "uniq_id": candidate.uniq_id,
            "avatar": candidate.avatar,
            "cover": candidate.cover,
            "format": candidate.format,
            "height": candidate.height,
            "resolution": candidate.resolution,
        }
        for candidate in extract_list_stream_candidates(
            list_page,
            html,
            max_height=max_height,
            pull_format=pull_format,
        )
    ]


def collect_list_streams(
    source_page,
    user_agent,
    max_height,
    pull_format,
    workers=4,
):
    rooms = []
    seen = set()
    with concurrent.futures.ThreadPoolExecutor(max_workers=max(1, workers)) as executor:
        futures = [
            executor.submit(
                fetch_list_streams,
                page,
                user_agent,
                max_height,
                pull_format,
            )
            for page in list_pages_for(source_page)
        ]
        for future in concurrent.futures.as_completed(futures):
            try:
                discovered = future.result()
            except Exception:
                continue
            for room in discovered:
                room_id = room.get("room_id", "")
                if not room_id or room_id in seen:
                    continue
                seen.add(room_id)
                rooms.append(room)
    return rooms


def resolve_room(room_page, user_agent, max_height, pull_format):
    cookie_jar = http.cookiejar.CookieJar()

    def room_fetch(url, headers=None):
        return fetch(
            url,
            headers=headers,
            maxread=8_000_000,
            cookie_jar=cookie_jar,
        )

    resolver = DouyinResolver(room_fetch, user_agent)
    candidate = resolver.resolve(
        room_page,
        max_height=max_height,
        pull_format=pull_format,
    )
    return {
        "provider": candidate.provider,
        "room_id": candidate.room_id,
        "room_page": candidate.selected_room_page,
        "nickname": candidate.nickname,
        "format": candidate.format,
        "height": candidate.height,
        "resolution": candidate.resolution,
    }


def discover_online_rooms(
    source_page,
    count,
    user_agent=DEFAULT_UA,
    max_height=720,
    pull_format="hls",
    workers=6,
):
    if not room_id_from_page(source_page):
        if not is_douyin_list_page(source_page):
            raise RuntimeError("unsupported douyin page")
        online = collect_list_streams(
            source_page,
            user_agent,
            max_height,
            pull_format,
            workers=min(workers, 6),
        )
        random.shuffle(online)
        return online[:count]

    room_pages = collect_room_pages(source_page, user_agent, min(workers, 6))
    random.shuffle(room_pages)
    candidate_limit = max(count * 5, count)
    room_pages = room_pages[:candidate_limit]
    online = []
    seen = set()

    old_random_room = os.environ.get("RANDOM_ROOM")
    old_room_retry = os.environ.get("ROOM_RETRY")
    os.environ["RANDOM_ROOM"] = "0"
    os.environ["ROOM_RETRY"] = "1"
    try:
        with concurrent.futures.ThreadPoolExecutor(max_workers=max(1, workers)) as executor:
            futures = [
                executor.submit(
                    resolve_room,
                    room_page,
                    user_agent,
                    max_height,
                    pull_format,
                )
                for room_page in room_pages
            ]
            for future in concurrent.futures.as_completed(futures):
                try:
                    room = future.result()
                except Exception:
                    continue
                room_id = room.get("room_id") or room_id_from_page(room.get("room_page", ""))
                if not room_id or room_id in seen:
                    continue
                seen.add(room_id)
                online.append(room)
                if len(online) >= count:
                    for pending in futures:
                        pending.cancel()
                    break
    finally:
        if old_random_room is None:
            os.environ.pop("RANDOM_ROOM", None)
        else:
            os.environ["RANDOM_ROOM"] = old_random_room
        if old_room_retry is None:
            os.environ.pop("ROOM_RETRY", None)
        else:
            os.environ["ROOM_RETRY"] = old_room_retry

    return online


def parse_args():
    parser = argparse.ArgumentParser(description="Discover online Douyin live rooms.")
    parser.add_argument("--page", default="https://live.douyin.com/")
    parser.add_argument("--count", type=int, default=8)
    parser.add_argument("--workers", type=int, default=6)
    parser.add_argument("--max-height", type=int, default=720)
    parser.add_argument("--pull-format", choices=["hls", "flv"], default="hls")
    parser.add_argument("--user-agent", default=DEFAULT_UA)
    return parser.parse_args()


def main():
    args = parse_args()
    count = min(max(1, args.count), 1000)
    workers = min(max(1, args.workers), 16)
    try:
        rooms = discover_online_rooms(
            args.page,
            count,
            user_agent=args.user_agent,
            max_height=args.max_height,
            pull_format=args.pull_format,
            workers=workers,
        )
    except Exception as exc:
        print(json.dumps({"ok": False, "error": str(exc), "rooms": []}, ensure_ascii=False))
        return 1

    print(
        json.dumps(
            {"ok": bool(rooms), "count": len(rooms), "rooms": rooms},
            ensure_ascii=False,
        )
    )
    return 0 if rooms else 2


if __name__ == "__main__":
    sys.exit(main())
