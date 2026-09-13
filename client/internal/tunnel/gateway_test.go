package tunnel

import "testing"

func TestGateway(t *testing.T) {
	tr := &Tunnel{}
	if got := tr.Gateway(); got != "" {
		t.Errorf("Gateway() with nil conf = %q, want empty", got)
	}

	tr.conf = &Conf{DNS: []string{"10.42.0.1"}}
	if got := tr.Gateway(); got != "10.42.0.1" {
		t.Errorf("Gateway() = %q, want 10.42.0.1", got)
	}

	tr.conf = &Conf{DNS: []string{"not-an-ip", "1.1.1.1"}}
	if got := tr.Gateway(); got != "1.1.1.1" {
		t.Errorf("Gateway() = %q, want first IP 1.1.1.1", got)
	}

	tr.conf = &Conf{DNS: []string{"not-an-ip"}}
	if got := tr.Gateway(); got != "" {
		t.Errorf("Gateway() with no IP = %q, want empty", got)
	}
}
