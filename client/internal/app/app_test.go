package app

import (
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{5, 32 * time.Second},
		{6, 60 * time.Second},  // 64s capped to 60s
		{50, 60 * time.Second}, // large attempt stays capped
	}
	for _, c := range cases {
		if got := backoff(c.attempt); got != c.want {
			t.Errorf("backoff(%d) = %v, want %v", c.attempt, got, c.want)
		}
	}
}
