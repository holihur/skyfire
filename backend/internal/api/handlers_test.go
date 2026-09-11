package api

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"skyfire/internal/driver"
	"skyfire/internal/manager"
)

func mustServer(t *testing.T, opts Options) *Server {
	t.Helper()
	conf := filepath.Join(t.TempDir(), "skyfire.json")
	mgr, err := manager.New(conf, driver.NewMock(), true, nil)
	if err != nil {
		t.Fatalf("manager: %v", err)
	}
	opts.Log = nil
	return New(mgr, driver.NewMock(), true, opts)
}

func doReq(s *Server, method, path, cookie string, username, password string) (*httptest.ResponseRecorder, string) {
	req := httptest.NewRequest(method, path, nil)
	if cookie != "" {
		req.Header.Set("Cookie", sessionCookie+"="+cookie)
	}
	if username != "" || password != "" {
		req.Header.Set("Content-Type", "application/json")
		body := `{"username":"` + username + `","password":"` + password + `"}`
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if cookie != "" {
			req.Header.Set("Cookie", sessionCookie+"="+cookie)
		}
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	set := w.Result().Cookies()
	var c string
	for _, ck := range set {
		if ck.Name == sessionCookie {
			c = ck.Value
		}
	}
	return w, c
}

func TestLoginSessionFlow(t *testing.T) {
	s := mustServer(t, Options{Username: "admin", Password: "s3cret", Token: "bearer"})

	if w, _ := doReq(s, "GET", "/api/interfaces", "", "", ""); w.Code != 401 {
		t.Fatalf("pre-auth GET: expected 401, got %d", w.Code)
	}
	if w, _ := doReq(s, "POST", "/api/login", "", "admin", "wrong"); w.Code != 401 {
		t.Fatalf("bad login: expected 401, got %d", w.Code)
	}
	w, cookie := doReq(s, "POST", "/api/login", "", "admin", "s3cret")
	if w.Code != 200 || cookie == "" {
		t.Fatalf("login failed: code=%d cookie=%q", w.Code, cookie)
	}
	if w, _ := doReq(s, "GET", "/api/interfaces", cookie, "", ""); w.Code != 200 {
		t.Fatalf("authed GET /api/interfaces: got %d", w.Code)
	}
	if w, _ := doReq(s, "POST", "/api/logout", cookie, "", ""); w.Code != 200 {
		t.Fatalf("logout: got %d", w.Code)
	}
	if w, _ := doReq(s, "GET", "/api/interfaces", cookie, "", ""); w.Code != 401 {
		t.Fatalf("GET after logout: got %d", w.Code)
	}
}

func TestBearerToken(t *testing.T) {
	s := mustServer(t, Options{Token: "secret"})
	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHealthReportsDriver(t *testing.T) {
	s := mustServer(t, Options{Token: "x"})
	req := httptest.NewRequest("GET", "/api/health", nil)
	req.Header.Set("Authorization", "Bearer x")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("health: got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "mock") {
		t.Fatalf("health body should mention driver, got %q", w.Body.String())
	}
}
