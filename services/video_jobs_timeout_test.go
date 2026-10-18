package services

import (
	"testing"
	"time"
)

func TestModelStatusTimeout(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  time.Duration
	}{
		{"", 120 * time.Second},
		{"45", 45 * time.Second},
		{" 180 ", 180 * time.Second},
		{"0", 120 * time.Second},
		{"-1", 120 * time.Second},
		{"invalid", 120 * time.Second},
		{"601", 120 * time.Second},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("MODEL_SERVER_STATUS_TIMEOUT_SECONDS", tc.value)
			if got := modelStatusTimeout(); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}
