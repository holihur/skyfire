package api

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"skyfire/internal/totp"
)

func postLogin(t *testing.T, s *Server, username, password, code string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": username, "password": password, "totp": code})
	req := httptest.NewRequest("POST", "/api/login", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	return w
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode body %q: %v", w.Body.String(), err)
	}
	return m
}

// wrongCode returns a 6-digit code that is not accepted at the current time.
func wrongCode(t *testing.T, secret string) string {
	t.Helper()
	now := time.Now()
	accepted := map[string]bool{}
	for i := -1; i <= 1; i++ {
		c, err := totp.CodeAt(secret, now.Add(time.Duration(i)*totp.Period))
		if err != nil {
			t.Fatal(err)
		}
		accepted[c] = true
	}
	for i := 0; i < 1_000_000; i++ {
		cand := fmt.Sprintf("%06d", i)
		if !accepted[cand] {
			return cand
		}
	}
	t.Fatal("could not find a wrong code")
	return ""
}

func TestTOTPEnrollmentOnFirstLogin(t *testing.T) {
	secret, err := totp.GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "totp.json")
	s := mustServer(t, Options{Username: "admin", Password: "pw", TOTPSecret: secret, TOTPBound: false, TOTPPath: path})

	// 1. password only on first login → enrollment payload, no session.
	w := postLogin(t, s, "admin", "pw", "")
	if w.Code != 200 {
		t.Fatalf("enroll: code=%d body=%s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["enroll"] != true {
		t.Fatalf("expected enroll=true, got %s", w.Body.String())
	}
	if body["secret"] != secret {
		t.Fatalf("enrollment secret mismatch: %v", body["secret"])
	}
	if _, ok := body["uri"]; !ok {
		t.Fatalf("missing otpauth uri: %s", w.Body.String())
	}

	// 2. a wrong code does not complete the binding.
	w = postLogin(t, s, "admin", "pw", wrongCode(t, secret))
	if w.Code != 401 {
		t.Fatalf("wrong enroll code: want 401, got %d", w.Code)
	}
	if cfg, _ := totp.Load(path); cfg.Bound() {
		t.Fatal("binding must not persist after a wrong code")
	}

	// 3. confirming with a valid code binds and signs in.
	code, err := totp.Code(secret)
	if err != nil {
		t.Fatal(err)
	}
	w = postLogin(t, s, "admin", "pw", code)
	if w.Code != 200 || decodeBody(t, w)["ok"] != true {
		t.Fatalf("confirm: code=%d body=%s", w.Code, w.Body.String())
	}
	if cfg, err := totp.Load(path); err != nil || !cfg.Bound() {
		t.Fatalf("binding not persisted: %+v %v", cfg, err)
	}

	// 4. later logins require a code.
	w = postLogin(t, s, "admin", "pw", "")
	if w.Code != 200 || decodeBody(t, w)["totpRequired"] != true {
		t.Fatalf("bound login without code: code=%d body=%s", w.Code, w.Body.String())
	}
	w = postLogin(t, s, "admin", "pw", wrongCode(t, secret))
	if w.Code != 401 {
		t.Fatalf("bound login with wrong code: want 401, got %d", w.Code)
	}
	code, _ = totp.Code(secret)
	w = postLogin(t, s, "admin", "pw", code)
	if w.Code != 200 || decodeBody(t, w)["ok"] != true {
		t.Fatalf("bound login with code: code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestAuthStatusEndpointIsPublic(t *testing.T) {
	secret, _ := totp.GenerateSecret()
	s := mustServer(t, Options{Username: "admin", Password: "pw", TOTPSecret: secret, TOTPBound: true, Token: "bearer"})
	req := httptest.NewRequest("GET", "/api/auth", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("GET /api/auth: %d", w.Code)
	}
	body := decodeBody(t, w)
	if body["totpEnabled"] != true || body["totpBound"] != true || body["passwordLogin"] != true {
		t.Fatalf("unexpected auth status: %s", w.Body.String())
	}
}

func TestLoginThrottleAfterRepeatedFailures(t *testing.T) {
	s := mustServer(t, Options{Username: "admin", Password: "pw", Token: "bearer"})
	var last int
	for i := 0; i < loginMaxFails+2; i++ {
		w := postLogin(t, s, "admin", "wrong", "")
		last = w.Code
	}
	if last != 429 {
		t.Fatalf("expected 429 after %d failures, got %d", loginMaxFails+2, last)
	}
}

func TestRedactPath(t *testing.T) {
	cases := map[string]string{
		"/api/p/abc123/wg.conf": "/api/p/<redacted>/wg.conf",
		"/api/p/abc123/wg.png":  "/api/p/<redacted>/wg.png",
		"/api/p/abc123":         "/api/p/<redacted>",
		"/api/interfaces/wg0":   "/api/interfaces/wg0",
		"/api/login":            "/api/login",
	}
	for in, want := range cases {
		if got := redactPath(in); got != want {
			t.Fatalf("redactPath(%q) = %q, want %q", in, got, want)
		}
	}
}
