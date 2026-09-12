// Package totp implements RFC 6238 TOTP (time-based one-time passwords) with
// the parameters used by common authenticator apps: HMAC-SHA1, 6 digits and a
// 30-second time step. Secrets are base32-encoded without padding.
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Digits is the code length (6).
const Digits = 6

// Period is the TOTP time step.
const Period = 30 * time.Second

// Window is how many adjacent time steps are accepted to tolerate clock skew
// (the current step plus one on each side).
const Window = 1

// base32NoPad is the encoding used by authenticator apps (uppercase, no "=").
var base32NoPad = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateSecret returns a fresh random 20-byte secret, base32-encoded
// without padding (e.g. "JBSWY3DPEHPK3PXP").
func GenerateSecret() (string, error) {
	key := make([]byte, 20)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("generate totp secret: %w", err)
	}
	return base32NoPad.EncodeToString(key), nil
}

// Verify reports whether code is a valid TOTP for secret at the current time.
// The code is trimmed and may omit leading zeros. It returns an error when the
// secret is not valid base32.
func Verify(secret, code string) (bool, error) {
	return VerifyAt(secret, code, time.Now())
}

// Code returns the current TOTP code for secret. It is primarily useful for
// diagnostics and tests.
func Code(secret string) (string, error) { return CodeAt(secret, time.Now()) }

// CodeAt returns the TOTP code for secret at the given instant.
func CodeAt(secret string, at time.Time) (string, error) {
	key, err := decodeSecret(secret)
	if err != nil {
		return "", err
	}
	return codeAt(key, at, Digits), nil
}

// VerifyAt reports whether code is valid for the given instant (used by tests
// and to support clock-skew). The secret is decoded leniently: case, padding
// and inner spaces are tolerated.
func VerifyAt(secret, code string, at time.Time) (bool, error) {
	key, err := decodeSecret(secret)
	if err != nil {
		return false, err
	}
	want := codeAt(key, at, Digits)
	got := strings.TrimSpace(code)
	if match(got, want) {
		return true, nil
	}
	// tolerate clock skew within ±Window steps
	for i := 1; i <= Window; i++ {
		if match(got, codeAt(key, at.Add(-time.Duration(i)*Period), Digits)) ||
			match(got, codeAt(key, at.Add(time.Duration(i)*Period), Digits)) {
			return true, nil
		}
	}
	return false, nil
}

// match compares a submitted code with an expected one in constant time.
func match(got, want string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// ProvisioningURI renders an otpauth:// URI suitable for a QR code consumed by
// authenticator apps.
func ProvisioningURI(secret, account, issuer string) string {
	u := url.URL{
		Scheme: "otpauth",
		Host:   "totp",
		Path:   "/" + issuer + ":" + account,
	}
	q := u.Query()
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", strconv.Itoa(Digits))
	q.Set("period", strconv.Itoa(int(Period.Seconds())))
	u.RawQuery = q.Encode()
	return u.String()
}

// codeAt computes the TOTP value for an instant. key is the raw secret bytes.
func codeAt(key []byte, at time.Time, digits int) string {
	counter := uint64(at.Unix() / int64(Period.Seconds()))
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	code := value % uint32(pow10(digits))
	return fmt.Sprintf("%0*d", digits, code)
}

func pow10(n int) int {
	v := 1
	for i := 0; i < n; i++ {
		v *= 10
	}
	return v
}

func decodeSecret(secret string) ([]byte, error) {
	s := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(secret), " ", ""))
	// re-add padding so both padded and unpadded input decode
	if rem := len(s) % 8; rem != 0 {
		s += strings.Repeat("=", 8-rem)
	}
	key, err := base32.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid totp secret: %w", err)
	}
	if len(key) == 0 {
		return nil, fmt.Errorf("invalid totp secret: empty")
	}
	return key, nil
}
