package services

import "testing"

func TestPlayerPortfolioInputValidation(t *testing.T) {
	for _, value := range []string{"https://example.com/video.mp4", "https://youtu.be/example", "http://example.com/history"} {
		if err := validatePortfolioURL(value, true); err != nil {
			t.Errorf("Valid link rejected: %s", value)
		}
	}
	for _, value := range []string{"", "javascript:alert(1)", "file:///video.mp4", "https://user:password@example.com/video"} {
		if err := validatePortfolioURL(value, true); err == nil {
			t.Errorf("Unsafe or empty link accepted: %s", value)
		}
	}
	if err := validatePortfolioURL("", false); err != nil {
		t.Fatal("Optional source links should support missing data")
	}
	if date, err := optionalPortfolioDate(""); err != nil || date != nil {
		t.Fatal("Missing dates must remain unknown")
	}
	if _, err := optionalPortfolioDate("2026-02-30"); err == nil {
		t.Fatal("Invalid calendar date accepted")
	}
	if !validHighlightUpload("video/mp4", 1024) || !validHighlightUpload("video/webm", maxPlayerHighlightSize) {
		t.Fatal("Supported video rejected")
	}
	if validHighlightUpload("text/html", 1024) || validHighlightUpload("video/mp4", 0) || validHighlightUpload("video/mp4", maxPlayerHighlightSize+1) {
		t.Fatal("Invalid video upload accepted")
	}
}
