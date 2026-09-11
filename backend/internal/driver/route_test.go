package driver

import "testing"

func TestRouteTargets(t *testing.T) {
	got := RouteTargets([]string{"10.42.0.0/24", "0.0.0.0/0", "::/0", "fd00::/8", "", "10.42.0.2/32"})
	want := []string{"10.42.0.0/24", "fd00::/8", "10.42.0.2/32"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if out := RouteTargets([]string{"0.0.0.0/0", "::/0"}); len(out) != 0 {
		t.Fatalf("default routes must be filtered out, got %v", out)
	}
	if out := RouteTargets(nil); len(out) != 0 {
		t.Fatalf("nil input must yield empty output, got %v", out)
	}
}
