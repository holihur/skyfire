package ui

import "testing"

func TestFmtBytes(t *testing.T) {
	cases := []struct {
		in   uint64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{3 * 1024 * 1024 * 1024, "3.0 GiB"},
	}
	for _, c := range cases {
		if got := fmtBytes(c.in); got != c.want {
			t.Errorf("fmtBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFmtRate(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0 B/s"},
		{0.4, "0 B/s"},
		{999, "999 B/s"},
		{2048, "2.0 KiB/s"},
		{5 * 1024 * 1024, "5.0 MiB/s"},
	}
	for _, c := range cases {
		if got := fmtRate(c.in); got != c.want {
			t.Errorf("fmtRate(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
