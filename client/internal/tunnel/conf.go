package tunnel

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

// Conf is a parsed wg-quick style configuration file.
type Conf struct {
	PrivateKey string
	Addresses  []string
	DNS        []string
	MTU        int
	Peers      []Peer
	// Whitelist lists hostnames routed through the tunnel in split-by-domain
	// mode. It is set programmatically (not parsed from the config file).
	Whitelist []string
}

// Peer is a parsed [Peer] section.
type Peer struct {
	PublicKey    string
	PresharedKey string
	AllowedIPs   []string
	Endpoint     string
	Keepalive    int
}

// Parse reads a wg-quick configuration (the format Skyfire serves at
// /api/p/{token}/wg.conf) into a Conf.
func Parse(text string) (*Conf, error) {
	c := &Conf{}
	section := ""
	var cur *Peer

	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			if section == "peer" {
				c.Peers = append(c.Peers, Peer{})
				cur = &c.Peers[len(c.Peers)-1]
			}
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)

		switch section {
		case "interface":
			switch key {
			case "privatekey":
				c.PrivateKey = val
			case "address":
				c.Addresses = splitList(val)
			case "dns":
				c.DNS = splitList(val)
			case "mtu":
				n, err := strconv.Atoi(val)
				if err != nil {
					return nil, fmt.Errorf("invalid MTU %q", val)
				}
				c.MTU = n
			}
		case "peer":
			if cur == nil {
				continue
			}
			switch key {
			case "publickey":
				cur.PublicKey = val
			case "presharedkey":
				cur.PresharedKey = val
			case "allowedips":
				cur.AllowedIPs = splitList(val)
			case "endpoint":
				cur.Endpoint = val
			case "persistentkeepalive":
				if n, err := strconv.Atoi(val); err == nil {
					cur.Keepalive = n
				}
			}
		}
	}

	if c.PrivateKey == "" {
		return nil, fmt.Errorf("configuration has no [Interface] PrivateKey")
	}
	if len(c.Peers) == 0 {
		return nil, fmt.Errorf("configuration has no [Peer]")
	}
	for i := range c.Peers {
		if c.Peers[i].PublicKey == "" {
			return nil, fmt.Errorf("peer %d has no PublicKey", i+1)
		}
		if c.Peers[i].Endpoint == "" {
			return nil, fmt.Errorf("peer %d has no Endpoint", i+1)
		}
	}
	return c, nil
}

// HasDefaultRoutes reports whether any peer claims 0.0.0.0/0 or ::/0.
func (c *Conf) HasDefaultRoutes() (v4, v6 bool) {
	for _, p := range c.Peers {
		for _, a := range p.AllowedIPs {
			switch a {
			case "0.0.0.0/0":
				v4 = true
			case "::/0":
				v6 = true
			}
		}
	}
	return v4, v6
}

// WhitelistMode reports whether the client routes only whitelisted hostnames
// through the tunnel (split-by-domain), ignoring the peer's AllowedIPs.
func (c *Conf) WhitelistMode() bool { return len(c.Whitelist) > 0 }

// EndpointIPs resolves every peer endpoint (host:port, [v6]:port or bare
// host) to its IP addresses. Hostnames go through the system resolver and
// unresolvable endpoints are skipped. The platform link code uses these to
// pin the tunnel transport to the physical path, so a full tunnel cannot
// capture its own encapsulated packets (a routing loop).
func (c *Conf) EndpointIPs() []netip.Addr {
	var out []netip.Addr
	seen := make(map[netip.Addr]struct{})
	add := func(ip netip.Addr) {
		ip = ip.Unmap()
		if _, ok := seen[ip]; ok {
			return
		}
		seen[ip] = struct{}{}
		out = append(out, ip)
	}
	for _, p := range c.Peers {
		host := p.Endpoint
		if h, _, err := net.SplitHostPort(p.Endpoint); err == nil {
			host = h
		}
		host = strings.Trim(host, "[]")
		if host == "" {
			continue
		}
		if ip, err := netip.ParseAddr(host); err == nil {
			add(ip)
			continue
		}
		for _, s := range lookupHost(host) {
			if ip, err := netip.ParseAddr(s); err == nil {
				add(ip)
			}
		}
	}
	return out
}

func lookupHost(host string) []string {
	ips, err := net.LookupHost(host)
	if err != nil {
		return nil
	}
	return ips
}

// UAPI renders the configuration in wireguard-go's IPC-set format. Keys are
// converted from base64 (config file) to hex (UAPI). When a full tunnel is
// requested the device gets a firewall mark so Linux policy routing can keep
// the endpoint's own packets out of the tunnel (matching wg-quick).
func (c *Conf) UAPI() (string, error) {
	var b strings.Builder
	priv, err := keyHex(c.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("private key: %w", err)
	}
	fmt.Fprintf(&b, "private_key=%s\n", priv)
	b.WriteString("listen_port=0\n")
	if v4, v6 := c.HasDefaultRoutes(); v4 || v6 {
		fmt.Fprintf(&b, "fwmark=%d\n", routeTable)
	}
	for _, p := range c.Peers {
		pub, err := keyHex(p.PublicKey)
		if err != nil {
			return "", fmt.Errorf("peer public key: %w", err)
		}
		fmt.Fprintf(&b, "public_key=%s\n", pub)
		if p.PresharedKey != "" {
			psk, err := keyHex(p.PresharedKey)
			if err != nil {
				return "", fmt.Errorf("preshared key: %w", err)
			}
			fmt.Fprintf(&b, "preshared_key=%s\n", psk)
		}
		fmt.Fprintf(&b, "endpoint=%s\n", p.Endpoint)
		if p.Keepalive > 0 {
			fmt.Fprintf(&b, "persistent_keepalive_interval=%d\n", p.Keepalive)
		}
		if len(p.AllowedIPs) > 0 {
			b.WriteString("replace_allowed_ips=true\n")
			for _, a := range p.AllowedIPs {
				fmt.Fprintf(&b, "allowed_ip=%s\n", a)
			}
		}
	}
	return b.String(), nil
}

func splitList(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func keyHex(b64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", errors.New("invalid base64 key")
	}
	if len(raw) != 32 {
		return "", fmt.Errorf("key is %d bytes, want 32", len(raw))
	}
	return hex.EncodeToString(raw), nil
}
