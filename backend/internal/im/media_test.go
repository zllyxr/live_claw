package im

import "testing"

func TestAssetURL(t *testing.T) {
	service := &Service{}
	service.SetMediaBaseURL(" https://media.example.test/claw-public/ ")

	tests := map[string]string{
		"":                                  "",
		"users/1/avatar.webp":               "https://media.example.test/claw-public/users/1/avatar.webp",
		"/users/1/avatar.webp":              "https://media.example.test/claw-public/users/1/avatar.webp",
		"https://cdn.example.test/a.webp":   "https://cdn.example.test/a.webp",
		"http://cdn.example.test/a.webp":    "http://cdn.example.test/a.webp",
		"data:image/png;base64,placeholder": "data:image/png;base64,placeholder",
		"blob:https://example.test/id":      "blob:https://example.test/id",
	}
	for input, want := range tests {
		if got := service.assetURL(input); got != want {
			t.Fatalf("assetURL(%q)=%q, want %q", input, got, want)
		}
	}
}
