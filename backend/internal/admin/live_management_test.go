package admin

import (
	"testing"
)

func TestNormalizeDouyinPage(t *testing.T) {
	page, roomID, err := normalizeDouyinPage("https://www.douyin.com/live/123456789?from=web")
	if err != nil {
		t.Fatalf("normalize douyin page: %v", err)
	}
	if page != "https://live.douyin.com/123456789" || roomID != "123456789" {
		t.Fatalf("unexpected page normalization: %s %s", page, roomID)
	}
}

func TestNormalizeDouyinPageRejectsOtherProviders(t *testing.T) {
	for _, raw := range []string{
		"https://www.huya.com/123456",
		"https://live.douyin.com.evil.example/123456",
		"http://live.douyin.com/123456",
	} {
		if _, _, err := normalizeDouyinPage(raw); err == nil {
			t.Fatalf("expected provider URL to be rejected: %s", raw)
		}
	}
}

func TestNormalizeDouyinRoomID(t *testing.T) {
	page, roomID, err := normalizeDouyinRoomID(" 826694648629 ")
	if err != nil {
		t.Fatalf("normalize douyin room id: %v", err)
	}
	if roomID != "826694648629" || page != "https://live.douyin.com/826694648629" {
		t.Fatalf("unexpected normalized room: %q %q", page, roomID)
	}
	for _, raw := range []string{"", "12", "https://live.douyin.com/123", "123/456"} {
		if _, _, err = normalizeDouyinRoomID(raw); err == nil {
			t.Fatalf("expected room id to be rejected: %q", raw)
		}
	}
}

func TestPositiveDecimalID(t *testing.T) {
	id, err := positiveDecimalID("1785252579710207004")
	if err != nil || id != 1785252579710207004 {
		t.Fatalf("parse decimal id: %d %v", id, err)
	}
	for _, raw := range []string{"", "0", "-1", "01", "1785252579710207004.0"} {
		if _, err = positiveDecimalID(raw); err == nil {
			t.Fatalf("expected decimal id to be rejected: %q", raw)
		}
	}
}

func TestAdminMediaAssetURL(t *testing.T) {
	handler := &Handler{mediaBaseURL: "/media"}
	if actual := handler.mediaAssetURL("claw-public", "users/1/avatar name.webp"); actual != "/media/claw-public/users/1/avatar%20name.webp" {
		t.Fatalf("unexpected media URL: %q", actual)
	}
}
