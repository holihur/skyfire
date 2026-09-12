package totp

import (
	"encoding/base32"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// rfcSecret is the ASCII secret from RFC 6238 Appendix B, base32-encoded the
// way a user would type it.
var rfcSecret = base32.StdEncoding.WithPadding(base32.NoPadding).
	EncodeToString([]byte("12345678901234567890"))

func TestCodeAtRFCVectors(t *testing.T) {
	key, err := decodeSecret(rfcSecret)
	if err != nil {
		t.Fatal(err)
	}
	// RFC 6238 SHA1 vectors (8 digits); we keep the last 6.
	cases := []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
		{20000000000, "353130"},
	}
	for _, tc := range cases {
		got := codeAt(key, time.Unix(tc.unix, 0), 6)
		if got != tc.want {
			t.Fatalf("codeAt(%d) = %s, want %s", tc.unix, got, tc.want)
		}
	}
}

func TestVerifyAtWindow(t *testing.T) {
	at := time.Unix(1111111109, 0)
	code := codeAt(mustKey(t, rfcSecret), at, 6)

	if ok, err := VerifyAt(rfcSecret, code, at); err != nil || !ok {
		t.Fatalf("current step must verify: ok=%v err=%v", ok, err)
	}
	// one step early/late is accepted (clock skew)
	if ok, _ := VerifyAt(rfcSecret, code, at.Add(-Period)); !ok {
		t.Fatal("previous step must verify")
	}
	if ok, _ := VerifyAt(rfcSecret, code, at.Add(Period)); !ok {
		t.Fatal("next step must verify")
	}
	// two steps away is rejected
	if ok, _ := VerifyAt(rfcSecret, code, at.Add(2*Period)); ok {
		t.Fatal("far step must not verify")
	}
	if ok, _ := VerifyAt(rfcSecret, "000000", at); ok {
		t.Fatal("wrong code must not verify")
	}
}

func TestVerifyTolerantSecretFormat(t *testing.T) {
	at := time.Unix(1111111109, 0)
	code := codeAt(mustKey(t, rfcSecret), at, 6)
	// lower-case with spaces should still decode
	spaced := " " + rfcSecret[:4] + " " + rfcSecret[4:] + " "
	if ok, err := VerifyAt(spaced, code, at); err != nil || !ok {
		t.Fatalf("tolerant secret rejected: ok=%v err=%v", ok, err)
	}
}

func TestGenerateSecret(t *testing.T) {
	s, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	key, err := decodeSecret(s)
	if err != nil {
		t.Fatalf("generated secret must decode: %v", err)
	}
	if len(key) != 20 {
		t.Fatalf("key length = %d, want 20", len(key))
	}
	if s == "" || (len(s)%8) == 0 && s[len(s)-1] == '=' {
		t.Fatalf("secret should be unpadded base32, got %q", s)
	}
}

func TestProvisioningURI(t *testing.T) {
	uri := ProvisioningURI("JBSWY3DPEHPK3PXP", "admin", "Skyfire")
	for _, want := range []string{"otpauth://totp/", "secret=JBSWY3DPEHPK3PXP", "issuer=Skyfire", "digits=6"} {
		if !strings.Contains(uri, want) {
			t.Fatalf("URI %q missing %q", uri, want)
		}
	}
}

func TestSaveLoadClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "totp.json")
	if got, err := Load(path); err != nil || got.Secret != "" || got.Bound() {
		t.Fatalf("missing file should load empty: %+v %v", got, err)
	}
	if err := Save(path, Config{Secret: "ABC234", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || got.Secret != "ABC234" || !got.Bound() {
		t.Fatalf("Load = %+v %v", got, err)
	}
	if err := Save(path, Config{Secret: "ABC234", Confirmed: false}); err != nil {
		t.Fatal(err)
	}
	if got, _ := Load(path); got.Bound() {
		t.Fatalf("unconfirmed config must not be bound: %+v", got)
	}
	if err := Clear(path); err != nil {
		t.Fatal(err)
	}
	if got, _ := Load(path); got.Secret != "" {
		t.Fatalf("Clear did not remove secret: %+v", got)
	}
	// clearing again is not an error
	if err := Clear(path); err != nil {
		t.Fatalf("second Clear: %v", err)
	}
}

func mustKey(t *testing.T, secret string) []byte {
	t.Helper()
	key, err := decodeSecret(secret)
	if err != nil {
		t.Fatal(err)
	}
	return key
}
