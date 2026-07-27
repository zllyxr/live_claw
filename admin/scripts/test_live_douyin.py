#!/usr/bin/env python3

import json
import os
import unittest
from unittest import mock

from live import build_ffmpeg_cmd
from discover_douyin_rooms import collect_list_streams, list_pages_for
from live_douyin import (
    DouyinResolver,
    choose_stream_url,
    extract_list_stream_candidates,
    extract_room_metadata,
    extract_room_pages,
    extract_stream_urls,
    infer_stream_height,
)


ROOM_HTML = r"""
<script>
{
  \"web_rid\":\"575144254842\",
  \"nickname\":\"授权测试主播\",
  \"hls_pull_url_map\": {
    \"uhd\":\"https:\/\/pull.example\/live_uhd.m3u8?expire=1\u0026sign=uhd\",
    \"hd\":\"https:\/\/pull.example\/live_hd.m3u8?expire=1\u0026sign=hd\",
    \"ld\":\"https:\/\/pull.example\/live_ld.m3u8?expire=1\u0026sign=ld\"
  },
  \"flv_pull_url\": {
    \"hd\":\"https:\/\/pull.example\/live_hd.flv?expire=1\u0026token=flv\"
  }
}
</script>
"""


class DouyinResolverTest(unittest.TestCase):
    def test_discovery_expands_homepage_categories(self):
        pages = list_pages_for("https://live.douyin.com/")
        self.assertEqual(pages[0], "https://live.douyin.com/")
        self.assertGreaterEqual(len(pages), 9)

    def test_discovery_deduplicates_resolved_rooms(self):
        page_rows = [
            {
                "provider": "douyin",
                "room_id": "111111",
                "room_page": "https://live.douyin.com/111111",
                "nickname": "A",
            },
        ]
        other_page_rows = [
            {
                "provider": "douyin",
                "room_id": "111111",
                "room_page": "https://live.douyin.com/111111",
                "nickname": "A",
            },
            {
                "provider": "douyin",
                "room_id": "222222",
                "room_page": "https://live.douyin.com/222222",
                "nickname": "B",
            },
        ]
        with mock.patch(
            "discover_douyin_rooms.list_pages_for",
            return_value=["page-a", "page-b"],
        ), mock.patch(
            "discover_douyin_rooms.fetch_list_streams",
            side_effect=[page_rows, other_page_rows],
        ):
            rooms = collect_list_streams(
                "https://live.douyin.com/",
                "test-agent",
                720,
                "hls",
                workers=1,
            )
        self.assertEqual({room["room_id"] for room in rooms}, {"111111", "222222"})

    def test_extracts_online_streams_from_category_payload(self):
        card = {
            "web_rid": "123456",
            "uniq_id": "female-room-candidate",
            "avatar": "https://img.example/avatar.jpg",
            "room": {
                "title": "晚风直播间",
                "owner": {"nickname": "晚风主播"},
                "stream_url": {
                    "hls_pull_url_map": {
                        "FULL_HD1": "https://pull.example/live_or4.m3u8",
                        "HD1": "https://pull.example/live_hd.m3u8",
                        "SD1": "https://pull.example/live_ld.m3u8",
                    }
                },
            },
        }
        flight = "e:" + json.dumps(["$", {"cards": [card]}], ensure_ascii=False)
        script = json.dumps([1, flight], ensure_ascii=False)
        html = f"<script>self.__pace_f.push({script})</script>"
        candidates = extract_list_stream_candidates(
            "https://live.douyin.com/categorynew/4_101",
            html,
            max_height=720,
            pull_format="hls",
        )
        self.assertEqual(len(candidates), 1)
        self.assertEqual(candidates[0].room_id, "123456")
        self.assertEqual(candidates[0].nickname, "晚风主播")
        self.assertEqual(candidates[0].height, 720)
        self.assertIn("live_hd.m3u8", candidates[0].stream_url)
        self.assertEqual(candidates[0].avatar, "https://img.example/avatar.jpg")
        self.assertEqual(candidates[0].uniq_id, "female-room-candidate")

    def test_missing_preferred_room_does_not_switch_to_another_room(self):
        card = {
            "web_rid": "222222",
            "room": {
                "title": "其他直播间",
                "owner": {"nickname": "其他主播"},
                "stream_url": {
                    "hls_pull_url_map": {
                        "HD1": "https://pull.example/other_hd.m3u8",
                    }
                },
            },
        }
        flight = "e:" + json.dumps(["$", {"cards": [card]}], ensure_ascii=False)
        script = json.dumps([1, flight], ensure_ascii=False)
        html = f"<script>self.__pace_f.push({script})</script>"
        resolver = DouyinResolver(lambda *_args, **_kwargs: html, "test-agent")
        with mock.patch.dict(
            os.environ,
            {
                "RANDOM_ROOM": "0",
                "ROOM_RETRY": "1",
                "ROOM_POOL_ONLY": "0",
                "STRICT_GENDER": "0",
                "GENDER_FILTER": "",
            },
            clear=False,
        ):
            with self.assertRaisesRegex(RuntimeError, "preferred douyin room"):
                resolver.resolve(
                    "https://live.douyin.com/categorynew/4_101"
                    "?preferred_room_id=111111",
                    max_height=720,
                    pull_format="hls",
                )

    def test_ffmpeg_starts_near_live_edge(self):
        command = build_ffmpeg_cmd(
            "https://pull.example/live.m3u8",
            "",
            "source",
            "https://live.douyin.com/575144254842",
            "test-agent",
            "",
            "rtmp://srs/live/test",
            0,
            live_start_index=-2,
        )
        index = command.index("-live_start_index")
        self.assertEqual(command[index + 1], "-2")

    def test_extracts_numeric_room_links(self):
        html = (
            r'<a href=\"https:\/\/live.douyin.com\/575144254842?show_type=highlight\">A</a>'
            r'<a href=\"\/693877538059\">B</a>'
        )
        self.assertEqual(
            extract_room_pages("https://live.douyin.com/", html),
            [
                "https://live.douyin.com/575144254842",
                "https://live.douyin.com/693877538059",
            ],
        )

    def test_metadata_skips_undefined_nickname_placeholders(self):
        room_id, nickname = extract_room_metadata(
            r'{"nickname":"$undefined","nickname":"晚风主播","web_rid":"123456"}'
        )
        self.assertEqual(room_id, "123456")
        self.assertEqual(nickname, "晚风主播")

    def test_repeated_unescape_preserves_signed_query(self):
        urls = extract_stream_urls(ROOM_HTML)
        self.assertIn(
            "https://pull.example/live_hd.m3u8?expire=1&sign=hd",
            urls,
        )

    def test_chooses_720p_hls(self):
        urls = extract_stream_urls(ROOM_HTML)
        url, stream_format, height, _ = choose_stream_url(urls, 720, "hls")
        self.assertIn("live_hd.m3u8", url)
        self.assertEqual(stream_format, "hls")
        self.assertEqual(height, 720)
        self.assertEqual(infer_stream_height(url), 720)

    def test_strict_mode_rejects_unapproved_room(self):
        resolver = DouyinResolver(lambda *_args, **_kwargs: ROOM_HTML, "test-agent")
        with mock.patch.dict(
            os.environ,
            {
                "ROOM_POOL_ONLY": "1",
                "STRICT_GENDER": "1",
                "GENDER_FILTER": "female",
                "APPROVED_ROOM_PAGES": "",
            },
            clear=False,
        ), mock.patch("live_douyin._mysql_pool_rows", return_value=[]):
            with self.assertRaisesRegex(RuntimeError, "not approved as female"):
                resolver.resolve("https://live.douyin.com/575144254842")

    def test_approved_room_resolves_with_verified_gender(self):
        resolver = DouyinResolver(lambda *_args, **_kwargs: ROOM_HTML, "test-agent")
        with mock.patch.dict(
            os.environ,
            {
                "ROOM_POOL_ONLY": "1",
                "STRICT_GENDER": "1",
                "GENDER_FILTER": "female",
                "APPROVED_ROOM_PAGES": "https://live.douyin.com/575144254842",
            },
            clear=False,
        ), mock.patch("live_douyin._mysql_pool_rows", return_value=[]):
            candidate = resolver.resolve(
                "https://live.douyin.com/575144254842",
                max_height=720,
                pull_format="hls",
            )
        self.assertEqual(candidate.room_id, "575144254842")
        self.assertEqual(candidate.nickname, "授权测试主播")
        self.assertEqual(candidate.height, 720)
        self.assertTrue(candidate.gender_verified)

    def test_homepage_falls_back_to_live_category(self):
        def fetcher(url, _headers=None):
            if "categorynew" in url:
                return r'<a href="https://live.douyin.com/575144254842">直播间</a>'
            if url.endswith("/575144254842"):
                return ROOM_HTML
            return "<html></html>"

        resolver = DouyinResolver(fetcher, "test-agent")
        with mock.patch.dict(
            os.environ,
            {
                "ROOM_POOL_ONLY": "0",
                "STRICT_GENDER": "0",
                "GENDER_FILTER": "",
            },
            clear=False,
        ):
            candidate = resolver.resolve("https://live.douyin.com/")
        self.assertEqual(candidate.room_id, "575144254842")
        self.assertEqual(candidate.height, 720)

    def test_offline_direct_room_falls_back_to_an_online_room(self):
        dead_room = "https://live.douyin.com/111111111111"

        def fetcher(url, _headers=None):
            if url == dead_room:
                return "<html>直播已结束</html>"
            if url == "https://live.douyin.com/":
                return r'<a href="https://live.douyin.com/575144254842">直播间</a>'
            if url.endswith("/575144254842"):
                return ROOM_HTML
            return "<html></html>"

        resolver = DouyinResolver(fetcher, "test-agent")
        with mock.patch.dict(
            os.environ,
            {
                "RANDOM_ROOM": "1",
                "ROOM_RETRY": "3",
                "ROOM_POOL_ONLY": "0",
                "STRICT_GENDER": "0",
                "GENDER_FILTER": "",
            },
            clear=False,
        ):
            candidate = resolver.resolve(dead_room, max_height=720, pull_format="hls")
        self.assertEqual(candidate.room_id, "575144254842")
        self.assertEqual(candidate.source_page, dead_room)


if __name__ == "__main__":
    unittest.main()
