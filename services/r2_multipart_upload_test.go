package services

import (
	"strings"
	"testing"
)

func TestUniqueSortedParts(t *testing.T) {
	got := uniqueSortedParts([]int{4, 2, 4, 1, 3, 2})
	want := []int{1, 2, 3, 4}

	if len(got) != len(want) {
		t.Fatalf("expected %d parts, got %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected part %d at index %d, got %d", want[i], i, got[i])
		}
	}
}

func TestBuildMultipartMatchVideoObjectKey(t *testing.T) {
	key := buildMultipartMatchVideoObjectKey(9, "VIPERS FT KITARA.mp4")

	if !strings.HasPrefix(key, "matches/9/") {
		t.Fatalf("expected match prefix, got %q", key)
	}
	if !strings.HasSuffix(key, "-vipers-ft-kitara.mp4") {
		t.Fatalf("expected sanitized filename suffix, got %q", key)
	}
}
