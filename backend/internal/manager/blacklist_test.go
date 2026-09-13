package manager

import "testing"

func TestSameEntries(t *testing.T) {
	cases := []struct {
		a, b []string
		want bool
	}{
		{nil, nil, true},
		{[]string{"a"}, []string{"a"}, true},
		{[]string{"a", "b"}, []string{"b", "a"}, true},
		{[]string{"a"}, []string{"b"}, false},
		{[]string{"a"}, []string{"a", "b"}, false},
		{[]string{"a", "a"}, []string{"a", "b"}, false},
	}
	for _, c := range cases {
		if got := sameEntries(c.a, c.b); got != c.want {
			t.Errorf("sameEntries(%v, %v) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
