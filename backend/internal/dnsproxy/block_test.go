package dnsproxy

import (
	"testing"

	"skyfire/internal/blacklist"
)

// query builds a minimal DNS query for name with qtype A.
func query(name string) []byte {
	q := []byte{0x12, 0x34, 0x01, 0x00, 0, 1, 0, 0, 0, 0, 0, 0}
	label := []byte{}
	for _, part := range splitDots(name) {
		label = append(label, byte(len(part)))
		label = append(label, part...)
	}
	label = append(label, 0)
	q = append(q, label...)
	q = append(q, 0, 1, 0, 1) // qtype A, qclass IN
	return q
}

func splitDots(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

func TestQuestionName(t *testing.T) {
	got, ok := questionName(query("ads.example.com"))
	if !ok || got != "ads.example.com" {
		t.Fatalf("questionName = %q, %v", got, ok)
	}
	if _, ok := questionName([]byte{1, 2, 3}); ok {
		t.Fatal("short buffer should not parse")
	}
}

func TestBlocked(t *testing.T) {
	s := &Server{Blacklist: blacklist.NewStore([]string{"ads.example.com", "*.tracker.test"})}
	if !s.blocked(query("ads.example.com")) {
		t.Error("exact domain should be blocked")
	}
	if !s.blocked(query("a.tracker.test")) {
		t.Error("wildcard subdomain should be blocked")
	}
	if s.blocked(query("good.example.com")) {
		t.Error("unlisted domain must not be blocked")
	}

	empty := &Server{}
	if empty.blocked(query("ads.example.com")) {
		t.Error("empty blacklist must not block")
	}
}
