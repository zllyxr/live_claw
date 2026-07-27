#!/usr/bin/env python3
"""Douyin resolver used by the authorized PAGE restream worker."""

from dataclasses import dataclass
import html as html_module
import json
import os
import random
import re
import subprocess
import urllib.parse


DEFAULT_CATEGORY_PAGES = tuple(
    f"https://live.douyin.com/categorynew/4_{category}"
    for category in range(101, 109)
)


@dataclass
class StreamCandidate:
    provider: str
    source_page: str
    selected_room_page: str
    room_id: str
    nickname: str
    stream_url: str
    format: str
    resolution: str
    height: int
    has_audio: bool = True
    gender_filter: str = ""
    gender_verified: bool = False
    gender_source: str = ""
    gender_confidence: float = 0.0
    title: str = ""
    avatar: str = ""
    cover: str = ""
    uniq_id: str = ""
    category_page: str = ""


def env_flag(name, default=False):
    value = os.environ.get(name)
    if value is None:
        return default
    return str(value).strip().lower() in {"1", "true", "yes", "on"}


def repeated_unescape(value, rounds=4):
    text = value or ""
    for _ in range(rounds):
        previous = text
        text = html_module.unescape(text)
        text = (
            text.replace("\\u0026", "&")
            .replace("\\u002F", "/")
            .replace("\\u003D", "=")
            .replace("\\u003F", "?")
            .replace("\\/", "/")
            .replace("\\&", "&")
            .replace("\\=", "=")
            .replace('\\"', '"')
        )
        try:
            text = urllib.parse.unquote(text)
        except Exception:
            pass
        if text == previous:
            break
    return text


def room_id_from_page(page):
    path = urllib.parse.urlsplit(page).path.strip("/")
    return path if re.fullmatch(r"\d{5,}", path or "") else ""


def is_douyin_list_page(page):
    path = urllib.parse.urlsplit(page).path.strip("/").lower()
    return (
        path == ""
        or path in {"hot_live", "hot_live/"}
        or path.startswith("category/")
        or path.startswith("categorynew/")
    )


def extract_room_pages(page, html):
    decoded = repeated_unescape(html)
    room_ids = []
    patterns = (
        r"https?://live\.douyin\.com/(\d{5,})",
        r"(?:href|url|room_url)[\"']?\s*[:=]\s*[\"'](?:https?:)?//live\.douyin\.com/(\d{5,})",
        r"[\"']/(\d{5,})(?:[?\"'/]|$)",
        r"[\"']web_rid[\"']\s*:\s*[\"']?(\d{5,})",
    )
    for pattern in patterns:
        for room_id in re.findall(pattern, decoded, re.I):
            if room_id not in room_ids:
                room_ids.append(room_id)
    return [f"https://live.douyin.com/{room_id}" for room_id in room_ids]


def infer_stream_height(url):
    path = urllib.parse.urlsplit(url).path.lower()
    rules = (
        (r"(?:_|/)(?:or4|origin|uhd)(?:[_.\-/]|$)", 1080),
        (r"(?:_|/)(?:hd|720p?)(?:[_.\-/]|$)", 720),
        (r"(?:_|/)(?:ld|540p?)(?:[_.\-/]|$)", 540),
        (r"(?:_|/)(?:sd|360p?)(?:[_.\-/]|$)", 360),
    )
    for pattern, height in rules:
        if re.search(pattern, path):
            return height
    match = re.search(r"(?:^|[_/\-])(\d{3,4})p?(?:[_./\-]|$)", path)
    return int(match.group(1)) if match else 0


def extract_stream_urls(html):
    decoded = repeated_unescape(html)
    urls = []
    patterns = (
        r"https?://[^\"'<>\s\\]+?\.m3u8(?:\?[^\"'<>\s\\]*)?",
        r"https?://[^\"'<>\s\\]+?\.flv(?:\?[^\"'<>\s\\]*)?",
    )
    for pattern in patterns:
        for url in re.findall(pattern, decoded, re.I):
            cleaned = url.rstrip("),;]}")
            if cleaned not in urls:
                urls.append(cleaned)
    return urls


def choose_stream_url(urls, max_height=720, pull_format="hls"):
    if not urls:
        raise RuntimeError("douyin stream url not found")

    preferred_format = "flv" if pull_format == "flv" else "hls"
    candidates = []
    for index, url in enumerate(urls):
        stream_format = "hls" if ".m3u8" in urllib.parse.urlsplit(url).path.lower() else "flv"
        height = infer_stream_height(url)
        within_limit = height == 0 or max_height <= 0 or height <= max_height
        format_score = 1 if stream_format == preferred_format else 0
        exact_hd = 1 if max_height == 720 and height == 720 else 0
        usable_height = height if within_limit else -height
        candidates.append(
            (
                (format_score, 1 if within_limit else 0, exact_hd, usable_height, -index),
                url,
                stream_format,
                height,
            )
        )

    _, url, stream_format, height = max(candidates, key=lambda item: item[0])
    resolution = f"1280x{height}" if height else ""
    return url, stream_format, height, resolution


def extract_room_metadata(html, fallback_room_id=""):
    decoded = repeated_unescape(html)
    nickname = ""
    room_id = fallback_room_id
    for match in re.finditer(r'"nickname"\s*:\s*"((?:\\.|[^"\\])*)"', decoded):
        try:
            candidate = json.loads('"' + match.group(1) + '"')
        except Exception:
            candidate = match.group(1)
        candidate = str(candidate or "").strip()
        if candidate.lower() in {"undefined", "$undefined", "null", "$null", "none"}:
            continue
        nickname = candidate
        break
    match = re.search(r'"web_rid"\s*:\s*"?(?P<room>\d{5,})', decoded)
    if match:
        room_id = match.group("room")
    return room_id, nickname


def _walk_json(value):
    yield value
    if isinstance(value, dict):
        for child in value.values():
            yield from _walk_json(child)
    elif isinstance(value, list):
        for child in value:
            yield from _walk_json(child)


def _quality_height(name):
    name = str(name or "").upper()
    if "FULL_HD" in name or name in {"ORIGIN", "OR4", "UHD"}:
        return 1080
    if name.startswith("HD"):
        return 720
    if name.startswith("SD1") or name in {"LD", "MD"}:
        return 540
    if name.startswith("SD2") or name == "SD":
        return 360
    return 0


def _choose_mapped_stream(stream_data, max_height=720, pull_format="hls"):
    if not isinstance(stream_data, dict):
        return None
    map_name = "flv_pull_url" if pull_format == "flv" else "hls_pull_url_map"
    stream_map = stream_data.get(map_name) or {}
    if not isinstance(stream_map, dict) or not stream_map:
        fallback_name = "hls_pull_url_map" if map_name == "flv_pull_url" else "flv_pull_url"
        stream_map = stream_data.get(fallback_name) or {}
    candidates = []
    for index, (quality, url) in enumerate(stream_map.items()):
        if not isinstance(url, str) or not url.startswith(("http://", "https://")):
            continue
        height = _quality_height(quality) or infer_stream_height(url)
        within_limit = height == 0 or max_height <= 0 or height <= max_height
        stream_format = "hls" if ".m3u8" in urllib.parse.urlsplit(url).path.lower() else "flv"
        candidates.append(
            (
                (
                    1 if stream_format == pull_format else 0,
                    1 if within_limit else 0,
                    height if within_limit else -height,
                    -index,
                ),
                url,
                stream_format,
                height,
            )
        )
    if not candidates:
        return None
    _, url, stream_format, height = max(candidates, key=lambda item: item[0])
    return url, stream_format, height, f"1280x{height}" if height else ""


def extract_list_stream_candidates(page, html, max_height=720, pull_format="hls"):
    candidates = []
    known = set()
    scripts = re.findall(r"<script[^>]*>(.*?)</script>", html or "", re.I | re.S)
    for script in scripts:
        if "stream_url" not in script and "streamSrc" not in script:
            continue
        match = re.fullmatch(r"\s*self\.__pace_f\.push\((.*)\)\s*;?\s*", script, re.S)
        if not match:
            continue
        try:
            pace_item = json.loads(match.group(1))
        except (ValueError, TypeError):
            continue
        if not isinstance(pace_item, list) or len(pace_item) < 2 or not isinstance(pace_item[1], str):
            continue
        payload_text = pace_item[1]
        if ":" not in payload_text:
            continue
        try:
            payload = json.loads(payload_text.split(":", 1)[1])
        except (ValueError, TypeError):
            continue
        for item in _walk_json(payload):
            if not isinstance(item, dict):
                continue
            room_id = str(item.get("web_rid") or "").strip()
            room = item.get("room")
            if not room_id or room_id in known or not isinstance(room, dict):
                continue
            mapped = _choose_mapped_stream(
                room.get("stream_url"),
                max_height=max_height,
                pull_format=pull_format,
            )
            if not mapped:
                continue
            stream_url, stream_format, height, resolution = mapped
            owner = room.get("owner") if isinstance(room.get("owner"), dict) else {}
            nickname = str(owner.get("nickname") or "").strip()
            if nickname.lower() in {"undefined", "$undefined", "null", "$null", "none"}:
                nickname = ""
            room_cover = room.get("cover") if isinstance(room.get("cover"), dict) else {}
            cover_urls = room_cover.get("url_list") if isinstance(room_cover.get("url_list"), list) else []
            known.add(room_id)
            candidates.append(
                StreamCandidate(
                    provider="douyin",
                    source_page=page,
                    selected_room_page=f"https://live.douyin.com/{room_id}",
                    room_id=room_id,
                    nickname=nickname,
                    stream_url=stream_url,
                    format=stream_format,
                    resolution=resolution,
                    height=height,
                    has_audio=True,
                    title=str(room.get("title") or "").strip(),
                    avatar=str(item.get("avatar") or "").strip(),
                    cover=str(item.get("cover") or (cover_urls[0] if cover_urls else "")).strip(),
                    uniq_id=str(item.get("uniq_id") or "").strip(),
                    category_page=page,
                )
            )
    return candidates


def _mysql_pool_rows():
    mysql_bin = os.environ.get("MYSQL_BIN", "mysql")
    host = os.environ.get("DATABASE_HOSTNAME", "db")
    port = os.environ.get("DATABASE_HOSTPORT", "3306")
    database = os.environ.get("DATABASE_DATABASE", "claw_live")
    username = os.environ.get("DATABASE_USERNAME", "claw")
    password = os.environ.get("DATABASE_PASSWORD", "")
    prefix = os.environ.get("DATABASE_PREFIX", "cmf_")
    table = prefix + "live_source_room_pool"
    query = (
        "SELECT room_id,room_page,nickname,category_page,verify_source,confidence "
        f"FROM `{table}` WHERE provider='douyin' AND gender_tag='female' "
        "AND verify_status=1 AND status=1 ORDER BY last_seen_at DESC,id DESC"
    )
    command = [
        mysql_bin,
        "--batch",
        "--raw",
        "--skip-column-names",
        "-h",
        host,
        "-P",
        str(port),
        "-u",
        username,
        database,
        "-e",
        query,
    ]
    env = os.environ.copy()
    if password:
        env["MYSQL_PWD"] = password
    try:
        output = subprocess.check_output(
            command,
            stderr=subprocess.DEVNULL,
            env=env,
            text=True,
            timeout=10,
        )
    except (OSError, subprocess.SubprocessError):
        return []

    rows = []
    for line in output.splitlines():
        columns = line.split("\t")
        if len(columns) < 6:
            continue
        try:
            confidence = float(columns[5] or 0)
        except ValueError:
            confidence = 0.0
        rows.append(
            {
                "room_id": columns[0],
                "room_page": columns[1] or f"https://live.douyin.com/{columns[0]}",
                "nickname": columns[2],
                "category_page": columns[3],
                "verify_source": columns[4],
                "confidence": confidence,
            }
        )
    return rows


def approved_pool_rows():
    pages = [
        value.strip()
        for value in os.environ.get("APPROVED_ROOM_PAGES", "").replace("\n", ",").split(",")
        if value.strip()
    ]
    rows = []
    for page in pages:
        room_id = room_id_from_page(page)
        if room_id:
            rows.append(
                {
                    "room_id": room_id,
                    "room_page": f"https://live.douyin.com/{room_id}",
                    "nickname": "",
                    "category_page": "",
                    "verify_source": "environment",
                    "confidence": 1.0,
                }
            )
    mysql_rows = _mysql_pool_rows()
    known = {row["room_id"] for row in rows}
    rows.extend(row for row in mysql_rows if row["room_id"] not in known)
    return rows


class DouyinResolver:
    def __init__(self, fetcher, user_agent):
        self.fetcher = fetcher
        self.user_agent = user_agent
        self._room_candidate_cache = []
        self._stream_candidate_cache = []

    def match(self, page):
        host = (urllib.parse.urlsplit(page).hostname or "").lower()
        return host == "live.douyin.com"

    def _pick_room(self, page, max_height=720, pull_format="hls"):
        strict_gender = env_flag("STRICT_GENDER", False)
        room_pool_only = env_flag("ROOM_POOL_ONLY", False)
        gender_filter = os.environ.get("GENDER_FILTER", "").strip().lower()
        rows = approved_pool_rows() if room_pool_only or strict_gender else []
        room_id = room_id_from_page(page)
        preferred_room_id = urllib.parse.parse_qs(
            urllib.parse.urlsplit(page).query
        ).get("preferred_room_id", [""])[0]

        if room_id:
            if strict_gender and gender_filter == "female":
                match = next((row for row in rows if row["room_id"] == room_id), None)
                if not match:
                    raise RuntimeError("room is not approved as female")
                return match
            return {
                "room_id": room_id,
                "room_page": f"https://live.douyin.com/{room_id}",
                "nickname": "",
                "verify_source": "",
                "confidence": 0.0,
            }

        if not is_douyin_list_page(page):
            raise RuntimeError("unsupported douyin page")

        if room_pool_only or (strict_gender and gender_filter == "female"):
            scoped = [
                row
                for row in rows
                if not row.get("category_page") or row.get("category_page") == page
            ]
            if not scoped:
                raise RuntimeError("no approved female room candidates")
            return random.choice(scoped)

        if self._stream_candidate_cache:
            if preferred_room_id:
                for index, candidate in enumerate(self._stream_candidate_cache):
                    if candidate.get("room_id") == preferred_room_id:
                        return self._stream_candidate_cache.pop(index)
                raise RuntimeError("preferred douyin room is not currently live")
            return self._stream_candidate_cache.pop()

        if self._room_candidate_cache:
            if preferred_room_id:
                chosen = next(
                    (
                        candidate
                        for candidate in self._room_candidate_cache
                        if room_id_from_page(candidate) == preferred_room_id
                    ),
                    "",
                )
                if not chosen:
                    raise RuntimeError("preferred douyin room is not currently live")
                self._room_candidate_cache.remove(chosen)
            else:
                chosen = self._room_candidate_cache.pop()
            return {
                "room_id": room_id_from_page(chosen),
                "room_page": chosen,
                "nickname": "",
                "verify_source": "",
                "confidence": 0.0,
            }

        list_pages = [page]
        if urllib.parse.urlsplit(page).path.strip("/").lower() in {"", "hot_live"}:
            categories = list(DEFAULT_CATEGORY_PAGES)
            random.shuffle(categories)
            list_pages.extend(categories)
        elif preferred_room_id:
            # 分类页内容会动态调整；固定房间不在原分类时，只在其他官方分类中
            # 继续寻找同一个 room_id，绝不改播别的房间。
            list_pages.extend(
                candidate
                for candidate in DEFAULT_CATEGORY_PAGES
                if candidate != page
            )

        candidates = []
        stream_candidates = []
        for list_page in list_pages:
            try:
                list_html = self.fetcher(
                    list_page,
                    {
                        "User-Agent": self.user_agent,
                        "Referer": "https://live.douyin.com/",
                        "Accept-Language": "zh-CN,zh;q=0.9",
                    },
                )
            except Exception:
                continue
            for candidate in extract_list_stream_candidates(
                list_page,
                list_html,
                max_height=max_height,
                pull_format=pull_format,
            ):
                row = {
                    "room_id": candidate.room_id,
                    "room_page": candidate.selected_room_page,
                    "nickname": candidate.nickname,
                    "verify_source": "",
                    "confidence": 0.0,
                    "stream_url": candidate.stream_url,
                    "stream_format": candidate.format,
                    "height": candidate.height,
                    "resolution": candidate.resolution,
                    "title": candidate.title,
                    "avatar": candidate.avatar,
                    "cover": candidate.cover,
                    "uniq_id": candidate.uniq_id,
                    "category_page": candidate.category_page,
                }
                if preferred_room_id and candidate.room_id == preferred_room_id:
                    return row
                if all(existing["room_id"] != candidate.room_id for existing in stream_candidates):
                    stream_candidates.append(row)
            for candidate in extract_room_pages(list_page, list_html):
                if candidate not in candidates:
                    candidates.append(candidate)
            if len(stream_candidates) >= 40 or len(candidates) >= 120:
                break
        if stream_candidates:
            random.shuffle(stream_candidates)
            self._stream_candidate_cache = stream_candidates
            if preferred_room_id:
                for index, candidate in enumerate(self._stream_candidate_cache):
                    if candidate["room_id"] == preferred_room_id:
                        return self._stream_candidate_cache.pop(index)
                raise RuntimeError("preferred douyin room is not currently live")
            return self._stream_candidate_cache.pop()
        if not candidates:
            raise RuntimeError("douyin list has no room candidates")
        random.shuffle(candidates)
        self._room_candidate_cache = candidates
        if preferred_room_id:
            chosen = next(
                (
                    candidate
                    for candidate in self._room_candidate_cache
                    if room_id_from_page(candidate) == preferred_room_id
                ),
                "",
            )
            if not chosen:
                raise RuntimeError("preferred douyin room is not currently live")
            self._room_candidate_cache.remove(chosen)
        else:
            chosen = self._room_candidate_cache.pop()
        return {
            "room_id": room_id_from_page(chosen),
            "room_page": chosen,
            "nickname": "",
            "verify_source": "",
            "confidence": 0.0,
        }

    def resolve(self, page, max_height=720, pull_format="hls"):
        if not self.match(page):
            raise RuntimeError("provider does not match PAGE")
        retry_count = max(1, int(os.environ.get("ROOM_RETRY", "1") or 1))
        random_room = env_flag("RANDOM_ROOM", False)
        initial = self._pick_room(page, max_height=max_height, pull_format=pull_format)
        last_error = None
        attempted_rooms = set()

        for attempt in range(retry_count):
            if attempt == 0:
                selected = initial
            elif random_room:
                selected = self._pick_room(
                    "https://live.douyin.com/",
                    max_height=max_height,
                    pull_format=pull_format,
                )
            else:
                break

            room_page = selected["room_page"]
            if room_page in attempted_rooms and attempt + 1 < retry_count:
                continue
            attempted_rooms.add(room_page)
            if selected.get("stream_url"):
                return StreamCandidate(
                    provider="douyin",
                    source_page=page,
                    selected_room_page=room_page,
                    room_id=selected["room_id"],
                    nickname=selected.get("nickname", ""),
                    stream_url=selected["stream_url"],
                    format=selected.get("stream_format", pull_format),
                    resolution=selected.get("resolution", ""),
                    height=int(selected.get("height", 0) or 0),
                    has_audio=True,
                    title=selected.get("title", ""),
                    avatar=selected.get("avatar", ""),
                    cover=selected.get("cover", ""),
                    uniq_id=selected.get("uniq_id", ""),
                    category_page=selected.get("category_page", ""),
                )
            try:
                room_html = self.fetcher(
                    room_page,
                    {
                        "User-Agent": self.user_agent,
                        "Referer": "https://live.douyin.com/",
                        "Accept-Language": "zh-CN,zh;q=0.9",
                    },
                )
                urls = extract_stream_urls(room_html)
                stream_url, stream_format, height, resolution = choose_stream_url(
                    urls,
                    max_height=max_height,
                    pull_format=pull_format,
                )
            except Exception as exc:
                last_error = exc
                if not random_room:
                    raise
                continue

            room_id, nickname = extract_room_metadata(room_html, selected["room_id"])
            gender_filter = os.environ.get("GENDER_FILTER", "").strip().lower()
            verified = bool(selected.get("verify_source"))
            return StreamCandidate(
                provider="douyin",
                source_page=page,
                selected_room_page=room_page,
                room_id=room_id,
                nickname=nickname or selected.get("nickname", ""),
                stream_url=stream_url,
                format=stream_format,
                resolution=resolution,
                height=height,
                has_audio=True,
                gender_filter=gender_filter,
                gender_verified=verified,
                gender_source=selected.get("verify_source", ""),
                gender_confidence=float(selected.get("confidence", 0.0)),
            )

        if last_error:
            raise RuntimeError(f"douyin live room fallback exhausted: {last_error}")
        raise RuntimeError("douyin live room fallback exhausted")
