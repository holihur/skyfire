// Package blacklist matches destinations against an operator-defined block
// list of domains, IP addresses and CIDR prefixes. Matched traffic is dropped
// by the caller: the DNS proxy discards queries for blocked domains and the
// data path discards packets to blocked addresses.
package blacklist

import (
	"net/netip"
	"strings"
	"sync/atomic"
)

// Store is a concurrency-safe holder for the current compiled blacklist, so
// the DNS proxy and the data path can share live updates from settings.
type Store struct {
	p atomic.Pointer[Matcher]
}

// NewStore compiles entries and returns a store holding them.
func NewStore(entries []string) *Store {
	s := &Store{}
	s.Set(entries)
	return s
}

// Set replaces the compiled blacklist.
func (s *Store) Set(entries []string) {
	if s == nil {
		return
	}
	s.p.Store(Compile(entries))
}

// Load returns the current matcher (never nil for a non-nil store).
func (s *Store) Load() *Matcher {
	if s == nil {
		return nil
	}
	return s.p.Load()
}

// Matcher is an immutable compiled blacklist.
type Matcher struct {
	nets    []netip.Prefix
	domains []string
}

// Compile builds a matcher from raw entries. Each entry is a CIDR prefix
// (10.0.0.0/8), a bare IP address (203.0.113.9), or a domain name. Domains are
// matched case-insensitively; a leading "*." matches any subdomain
// (*.ads.example matches a.ads.example but not ads.example). Unparseable
// entries are treated as domains.
func Compile(entries []string) *Matcher {
	m := &Matcher{}
	for _, raw := range entries {
		e := strings.TrimSpace(raw)
		if e == "" || strings.HasPrefix(e, "#") {
			continue
		}
		if p, err := netip.ParsePrefix(e); err == nil {
			m.nets = append(m.nets, p.Masked())
			continue
		}
		if a, err := netip.ParseAddr(e); err == nil {
			m.nets = append(m.nets, netip.PrefixFrom(a.Unmap(), a.Unmap().BitLen()))
			continue
		}
		m.domains = append(m.domains, strings.ToLower(strings.TrimSuffix(e, ".")))
	}
	return m
}

// Empty reports whether the matcher has no entries.
func (m *Matcher) Empty() bool {
	return m == nil || (len(m.nets) == 0 && len(m.domains) == 0)
}

// Prefixes returns the compiled address/prefix entries (for firewalls).
func (m *Matcher) Prefixes() []netip.Prefix {
	if m == nil {
		return nil
	}
	return append([]netip.Prefix(nil), m.nets...)
}

// MatchIP reports whether ip falls under any blocked address or prefix.
func (m *Matcher) MatchIP(ip netip.Addr) bool {
	if m == nil {
		return false
	}
	ip = ip.Unmap()
	for _, p := range m.nets {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// MatchDomain reports whether name is blocked, either exactly or by a
// "*.suffix" wildcard entry.
func (m *Matcher) MatchDomain(name string) bool {
	if m == nil || len(m.domains) == 0 {
		return false
	}
	name = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(name), "."))
	for _, d := range m.domains {
		if suffix, ok := strings.CutPrefix(d, "*"); ok {
			// "*.example.com" -> suffix ".example.com"
			if suffix != "" && strings.HasSuffix(name, suffix) {
				return true
			}
			continue
		}
		if name == d {
			return true
		}
	}
	return false
}
