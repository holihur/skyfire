package manager

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/skip2/go-qrcode"
	"skyfire/internal/store"
)

// clientEndpoint resolves the endpoint baked into client configs.
func clientEndpoint(settings store.Settings, iface *store.Interface) string {
	ep := strings.TrimSpace(settings.PublicEndpoint)
	if ep == "" {
		ep = "YOUR.SERVER.HOST.OR.IP"
	}
	if !strings.Contains(ep, ":") {
		if iface.ListenPort > 0 {
			ep = net.JoinHostPort(ep, strconv.Itoa(iface.ListenPort))
		}
	}
	return ep
}

// clientAddress normalizes a peer address to a CIDR (defaults to a host /32
// or /128).
func clientAddress(addr string) string {
	if addr == "" {
		return ""
	}
	if strings.Contains(addr, "/") {
		return addr
	}
	if strings.Contains(addr, ":") {
		return addr + "/128"
	}
	return addr + "/32"
}

func confDNS(dns []string) string {
	if len(dns) == 0 {
		return ""
	}
	return strings.Join(dns, ", ")
}

func clientRoutes(p *store.Peer) []string {
	if len(p.ClientRoutes) > 0 {
		return p.ClientRoutes
	}
	return store.DefaultRoutes()
}

// ClientConfig renders a ready-to-use client configuration file (the kind you
// import into a phone or laptop).
func ClientConfig(settings store.Settings, iface *store.Interface, p *store.Peer) string {
	var b strings.Builder
	addr := clientAddress(p.Address)
	if addr != "" {
		fmt.Fprintf(&b, "[Interface]\nPrivateKey = %s\n", p.PrivateKey)
		fmt.Fprintf(&b, "Address = %s\n", addr)
		if d := confDNS(p.DNS); d != "" {
			fmt.Fprintf(&b, "DNS = %s\n", d)
		}
	} else {
		// no assigned address: still emit a config skeleton
		fmt.Fprintf(&b, "[Interface]\nPrivateKey = %s\n", p.PrivateKey)
	}
	b.WriteString("\n[Peer]\n")
	fmt.Fprintf(&b, "PublicKey = %s\n", iface.PublicKey)
	if p.PresharedKey != "" {
		fmt.Fprintf(&b, "PresharedKey = %s\n", p.PresharedKey)
	}
	fmt.Fprintf(&b, "AllowedIPs = %s\n", strings.Join(clientRoutes(p), ", "))
	fmt.Fprintf(&b, "Endpoint = %s\n", clientEndpoint(settings, iface))
	if p.PersistentKeepalive > 0 {
		fmt.Fprintf(&b, "PersistentKeepalive = %d\n", p.PersistentKeepalive)
	}
	return b.String()
}

// ServerConfig renders a wg-quick style configuration of the server interface
// for export.
func ServerConfig(iface *store.Interface) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[Interface]\n")
	if len(iface.Addresses) > 0 {
		fmt.Fprintf(&b, "Address = %s\n", strings.Join(iface.Addresses, ", "))
	}
	fmt.Fprintf(&b, "PrivateKey = %s\n", iface.PrivateKey)
	if iface.ListenPort > 0 {
		fmt.Fprintf(&b, "ListenPort = %d\n", iface.ListenPort)
	}
	if len(iface.DNS) > 0 {
		fmt.Fprintf(&b, "DNS = %s\n", strings.Join(iface.DNS, ", "))
	}
	for _, p := range iface.Peers {
		if !p.Enabled {
			continue
		}
		b.WriteString("\n[Peer]\n")
		fmt.Fprintf(&b, "# %s\n", p.Name)
		fmt.Fprintf(&b, "PublicKey = %s\n", p.PublicKey)
		if p.PresharedKey != "" {
			fmt.Fprintf(&b, "PresharedKey = %s\n", p.PresharedKey)
		}
		if p.Endpoint != "" {
			fmt.Fprintf(&b, "Endpoint = %s\n", p.Endpoint)
		}
		if p.PersistentKeepalive > 0 {
			fmt.Fprintf(&b, "PersistentKeepalive = %d\n", p.PersistentKeepalive)
		}
		ips := p.AllowedIPs
		if len(ips) == 0 {
			ips = []string{p.Address}
		}
		fmt.Fprintf(&b, "AllowedIPs = %s\n", strings.Join(ips, ", "))
	}
	return b.String()
}

// ConfigQR renders a WireGuard configuration file as a PNG QR code. The wire
// guard mobile apps import such codes to add a tunnel.
func ConfigQR(text string) ([]byte, error) {
	png, err := qrcode.Encode(text, qrcode.Medium, 512)
	if err != nil {
		return nil, fmt.Errorf("encode qr: %w", err)
	}
	return png, nil
}
