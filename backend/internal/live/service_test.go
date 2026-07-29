package live

import (
	"context"
	"errors"
	"testing"
)

func TestIsDouyinHost(t *testing.T) {
	if !isDouyinHost("LIVE.DOUYIN.COM") {
		t.Fatal("canonical Douyin live host must be accepted")
	}
	if !isDouyinHost("webcast.amemv.com") {
		t.Fatal("official Douyin reflow host must be accepted")
	}
	for _, host := range []string{
		"douyin.com",
		"www.douyin.com",
		"live.douyin.com.evil.example",
		"webcast.amemv.com.evil.example",
		"huya.com",
	} {
		if isDouyinHost(host) {
			t.Fatalf("unexpected live source host accepted: %s", host)
		}
	}
}

func TestExtractAndSelectDouyinStreams(t *testing.T) {
	streams := extractStreamCandidates(`
		{"sd":"https:\/\/pull.example.test\/room_480p.m3u8?token=a",
		 "hd":"https:\/\/pull.example.test\/room_720p.m3u8?token=b",
		 "flv":"https:\/\/pull.example.test\/room_1080p.flv?token=c"}`)
	selected := selectStream(streams, 720, "hls")
	if selected.Format != "hls" || selected.Height != 720 {
		t.Fatalf("unexpected selected source: %#v", selected)
	}
}

func TestExtractDouyinStreamsUpgradesHTTPToHTTPS(t *testing.T) {
	streams := extractStreamCandidates(`
		{"hd":"http:\/\/pull-hs.example.test\/room_hd.m3u8?token=a"}`)
	if len(streams) != 1 {
		t.Fatalf("expected one source, got %d", len(streams))
	}
	if streams[0].URL != "https://pull-hs.example.test/room_hd.m3u8?token=a" {
		t.Fatalf("expected secure source URL, got %q", streams[0].URL)
	}
}

func TestIsDouyinMediaHost(t *testing.T) {
	for _, host := range []string{
		"douyincdn.com",
		"pull-hs-f5.flive.douyincdn.com",
		"PULL-HLS.DOUYINCDN.COM",
	} {
		if !isDouyinMediaHost(host) {
			t.Fatalf("expected media host to be accepted: %s", host)
		}
	}
	for _, host := range []string{
		"douyin.com",
		"douyincdn.com.evil.example",
		"evil-douyincdn.com",
		"127.0.0.1",
	} {
		if isDouyinMediaHost(host) {
			t.Fatalf("unexpected media host accepted: %s", host)
		}
	}
}

func TestIsHLSManifest(t *testing.T) {
	for _, value := range [][]byte{
		[]byte("#EXTM3U\n#EXT-X-VERSION:3"),
		[]byte("\n  #EXTM3U\r\n#EXTINF:2"),
	} {
		if !isHLSManifest(value) {
			t.Fatalf("expected valid HLS manifest: %q", value)
		}
	}
	for _, value := range [][]byte{
		nil,
		[]byte("<html>not found</html>"),
		[]byte("https://example.test/stream.m3u8"),
	} {
		if isHLSManifest(value) {
			t.Fatalf("unexpected HLS manifest accepted: %q", value)
		}
	}
}

func TestProbeDouyinRejectsInvalidRoomIDBeforeNetwork(t *testing.T) {
	service := New(nil, nil)
	if _, err := service.ProbeDouyin(context.Background(), "https://live.douyin.com/123"); !errors.Is(err, ErrSourceUnavailable) {
		t.Fatalf("expected invalid room id to be rejected, got %v", err)
	}
}

func TestSourceCacheKeyIncludesProviderRoomID(t *testing.T) {
	first := sourceCacheKey(Room{ID: 9, ProviderRoomID: "123456"})
	second := sourceCacheKey(Room{ID: 9, ProviderRoomID: "654321"})
	if first == second {
		t.Fatalf("provider room id must participate in cache key: %q", first)
	}
}
