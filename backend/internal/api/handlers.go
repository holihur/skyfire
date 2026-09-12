package api

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"skyfire/internal/driver"
	"skyfire/internal/manager"
	"skyfire/internal/store"
	"skyfire/internal/totp"
)

// Server exposes the manager over HTTP and optionally serves the built
// frontend.
type Server struct {
	mgr      *manager.Manager
	drv      driver.Driver
	dryRun   bool
	version  string
	static   string
	staticFS fs.FS
	token    string
	username string
	password string
	// totpSecret, when non-empty, requires a TOTP code in addition to the
	// password on web login. totpBound is false until the first login has
	// confirmed an authenticator.
	totpSecret string
	totpBound  bool
	totpPath   string
	totpMu     sync.Mutex
	log        *slog.Logger
	mux        *http.ServeMux
	handler    http.Handler

	sessionMu sync.Mutex
	sessions  map[string]time.Time
	login     *loginLimiter
}

// Options configures the HTTP server.
type Options struct {
	StaticDir string
	// StaticFS, when set, serves the frontend from an embedded filesystem and
	// takes precedence over StaticDir.
	StaticFS fs.FS
	Token    string
	// Username/Password enable single-user password login (cookie session).
	Username string
	Password string
	// TOTPSecret, when set, enables TOTP two-factor authentication.
	TOTPSecret string
	// TOTPBound reports whether an authenticator has already been enrolled.
	TOTPBound bool
	// TOTPPath is where a successful first-login enrollment is persisted.
	TOTPPath string
	// Version is the daemon version reported by /api/health.
	Version string
	Log     *slog.Logger
}

// New builds the HTTP handler.
func New(mgr *manager.Manager, drv driver.Driver, dryRun bool, opts Options) *Server {
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}
	s := &Server{
		mgr:        mgr,
		drv:        drv,
		dryRun:     dryRun,
		version:    opts.Version,
		static:     opts.StaticDir,
		staticFS:   opts.StaticFS,
		token:      opts.Token,
		username:   opts.Username,
		password:   opts.Password,
		totpSecret: opts.TOTPSecret,
		totpBound:  opts.TOTPBound,
		totpPath:   opts.TOTPPath,
		log:        log,
		mux:        http.NewServeMux(),
		sessions:   make(map[string]time.Time),
		login:      newLoginLimiter(),
	}
	s.routes()
	s.handler = s.chain()
	return s
}

func (s *Server) routes() {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/auth", s.handleAuthStatus)
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	mux.HandleFunc("PUT /api/settings", s.handlePutSettings)
	mux.HandleFunc("GET /api/interfaces", s.handleList)
	mux.HandleFunc("POST /api/interfaces", s.handleCreate)
	mux.HandleFunc("GET /api/interfaces/{name}", s.handleGet)
	mux.HandleFunc("PUT /api/interfaces/{name}", s.handleUpdate)
	mux.HandleFunc("DELETE /api/interfaces/{name}", s.handleDelete)
	mux.HandleFunc("POST /api/interfaces/{name}/up", s.handleUp)
	mux.HandleFunc("GET /api/interfaces/{name}/config", s.handleServerConfig)
	mux.HandleFunc("GET /api/interfaces/{name}/private-key", s.handlePrivateKey)
	mux.HandleFunc("POST /api/interfaces/{name}/peers", s.handleCreatePeer)
	mux.HandleFunc("PUT /api/interfaces/{name}/peers/{key}", s.handleUpdatePeer)
	mux.HandleFunc("DELETE /api/interfaces/{name}/peers/{key}", s.handleDeletePeer)
	mux.HandleFunc("GET /api/interfaces/{name}/peers/{key}/config", s.handlePeerConfig)
	mux.HandleFunc("GET /api/interfaces/{name}/peers/{key}/config.png", s.handlePeerQR)

	// Token-scoped client endpoints: reachable without a login. The token
	// itself is the credential and only exposes a single peer's config.
	mux.HandleFunc("GET /api/p/{token}/wg.conf", s.handleTokenConfig)
	mux.HandleFunc("GET /api/p/{token}/wg.png", s.handleTokenQR)

	s.mux = mux
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

// ---------------------------------------------------------------------------
// Auth
// ---------------------------------------------------------------------------

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	// TOTP is the 6-digit authenticator code, required when 2FA is enabled.
	TOTP string `json:"totp"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in loginRequest
	if err := decode(r, &in); err != nil {
		writeError(w, 400, err)
		return
	}
	if s.password == "" {
		writeError(w, 401, errors.New("password login not configured"))
		return
	}

	key := clientKey(r)
	if ok, retry := s.login.allowed(key); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		writeError(w, http.StatusTooManyRequests, errors.New("too many failed attempts, try again later"))
		return
	}

	// Constant-time credential comparison so a mismatch does not leak which
	// field was wrong through response timing.
	userOK := subtle.ConstantTimeCompare([]byte(in.Username), []byte(s.username)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(in.Password), []byte(s.password)) == 1
	if !userOK || !passOK {
		s.login.fail(key)
		writeError(w, http.StatusUnauthorized, errors.New("invalid credentials"))
		return
	}

	if s.totpEnabled() {
		if err := s.checkTOTP(w, key, &in); err != nil {
			return // a response was already written
		}
	}

	s.login.reset(key)
	s.createSession(w, r)
	writeJSON(w, 200, map[string]any{"ok": true})
}

// totpEnabled reports whether TOTP two-factor is active.
func (s *Server) totpEnabled() bool { return s.totpSecret != "" }

// checkTOTP handles the second factor: it either writes an enrollment payload,
// requests a code, or validates the submitted code. It returns a non-nil
// error when it has already written a response and the caller must stop.
func (s *Server) checkTOTP(w http.ResponseWriter, key string, in *loginRequest) error {
	s.totpMu.Lock()
	secret, bound := s.totpSecret, s.totpBound
	s.totpMu.Unlock()

	// First login: no authenticator bound yet. Ask the user to enroll one and
	// confirm with a code before creating a session.
	if !bound {
		if in.TOTP == "" {
			writeJSON(w, 200, s.enrollPayload(secret))
			return errResponded
		}
		ok, err := totp.Verify(secret, in.TOTP)
		if err != nil || !ok {
			s.login.fail(key)
			writeError(w, http.StatusUnauthorized, errors.New("invalid two-factor code"))
			return errResponded
		}
		s.totpMu.Lock()
		s.totpBound = true
		s.totpMu.Unlock()
		if s.totpPath != "" {
			if err := totp.Save(s.totpPath, totp.Config{Secret: secret, Confirmed: true}); err != nil {
				s.log.Error("persist totp binding", "error", err)
			}
		}
		return nil
	}

	// Bound: a valid code is mandatory.
	if in.TOTP == "" {
		writeJSON(w, 200, map[string]any{"totpRequired": true})
		return errResponded
	}
	ok, err := totp.Verify(secret, in.TOTP)
	if err != nil || !ok {
		s.login.fail(key)
		writeError(w, http.StatusUnauthorized, errors.New("invalid two-factor code"))
		return errResponded
	}
	return nil
}

// errResponded signals that a handler has already written its response.
var errResponded = errors.New("response already written")

// enrollPayload builds the first-login enrollment response: the shared secret,
// the otpauth URI and a QR data URL for authenticator apps.
func (s *Server) enrollPayload(secret string) map[string]any {
	uri := totp.ProvisioningURI(secret, s.username, "Skyfire")
	resp := map[string]any{
		"enroll": true,
		"secret": secret,
		"uri":    uri,
	}
	if png, err := manager.ConfigQR(uri); err == nil {
		resp["qr"] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	}
	return resp
}

// handleAuthStatus is public and lets the login page discover which factors are
// required, without revealing any credential material.
func (s *Server) handleAuthStatus(w http.ResponseWriter, _ *http.Request) {
	s.totpMu.Lock()
	bound := s.totpBound
	s.totpMu.Unlock()
	writeJSON(w, 200, map[string]any{
		"passwordLogin": s.password != "",
		"totpEnabled":   s.totpEnabled(),
		"totpBound":     bound,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.destroySession(w, r)
	writeJSON(w, 200, map[string]any{"ok": true})
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{
		"status":  "ok",
		"driver":  s.drv.Name(),
		"dryRun":  s.dryRun,
		"version": s.version,
	})
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.mgr.Settings())
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var in store.Settings
	if err := decode(r, &in); err != nil {
		writeError(w, 400, err)
		return
	}
	if err := s.mgr.UpdateSettings(in); err != nil {
		writeError(w, 500, err)
		return
	}
	writeJSON(w, 200, s.mgr.Settings())
}

func (s *Server) handleList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, s.mgr.List())
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var in store.Interface
	if err := decode(r, &in); err != nil {
		writeError(w, 400, err)
		return
	}
	view, err := s.mgr.CreateInterface(&in)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, 201, view)
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	view, err := s.mgr.Get(r.PathValue("name"))
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, 200, view)
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	var patch manager.InterfacePatch
	if err := decode(r, &patch); err != nil {
		writeError(w, 400, err)
		return
	}
	view, err := s.mgr.UpdateInterface(r.PathValue("name"), &patch)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, 200, view)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.mgr.DeleteInterface(r.PathValue("name")); err != nil {
		writeManagerError(w, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) handleUp(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Up bool `json:"up"`
	}
	if err := decode(r, &in); err != nil {
		writeError(w, 400, err)
		return
	}
	view, err := s.mgr.SetInterfaceUp(r.PathValue("name"), in.Up)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, 200, view)
}

func (s *Server) handleServerConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.mgr.ServerConfig(r.PathValue("name"))
	if err != nil {
		writeManagerError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", attachment(r.PathValue("name")+"-server.conf"))
	_, _ = w.Write([]byte(cfg))
}

func (s *Server) handlePrivateKey(w http.ResponseWriter, r *http.Request) {
	key, err := s.mgr.PrivateKey(r.PathValue("name"))
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"privateKey": key})
}

func (s *Server) handleCreatePeer(w http.ResponseWriter, r *http.Request) {
	var in manager.PeerInput
	if err := decode(r, &in); err != nil {
		writeError(w, 400, err)
		return
	}
	view, err := s.mgr.CreatePeer(r.PathValue("name"), &in)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, 201, view)
}

func (s *Server) handleUpdatePeer(w http.ResponseWriter, r *http.Request) {
	pub, err := urlDecodeKey(r.PathValue("key"))
	if err != nil {
		writeError(w, 400, err)
		return
	}
	var in manager.PeerInput
	if err := decode(r, &in); err != nil {
		writeError(w, 400, err)
		return
	}
	view, err := s.mgr.UpdatePeer(r.PathValue("name"), pub, &in)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	writeJSON(w, 200, view)
}

func (s *Server) handleDeletePeer(w http.ResponseWriter, r *http.Request) {
	pub, err := urlDecodeKey(r.PathValue("key"))
	if err != nil {
		writeError(w, 400, err)
		return
	}
	if err := s.mgr.DeletePeer(r.PathValue("name"), pub); err != nil {
		writeManagerError(w, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) handlePeerConfig(w http.ResponseWriter, r *http.Request) {
	pub, err := urlDecodeKey(r.PathValue("key"))
	if err != nil {
		writeError(w, 400, err)
		return
	}
	cfg, err := s.mgr.ClientConfig(r.PathValue("name"), pub)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", attachment(r.PathValue("name")+".conf"))
	_, _ = w.Write([]byte(cfg))
}

func (s *Server) handlePeerQR(w http.ResponseWriter, r *http.Request) {
	pub, err := urlDecodeKey(r.PathValue("key"))
	if err != nil {
		writeError(w, 400, err)
		return
	}
	cfg, err := s.mgr.ClientConfig(r.PathValue("name"), pub)
	if err != nil {
		writeManagerError(w, err)
		return
	}
	png, err := manager.ConfigQR(cfg)
	if err != nil {
		writeError(w, 500, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(png)
}

// handleTokenConfig serves a single peer's client config identified by its
// client token, without requiring a login.
func (s *Server) handleTokenConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.mgr.ClientConfigByToken(r.PathValue("token"))
	if err != nil {
		writeManagerError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", attachment("wg.conf"))
	_, _ = w.Write([]byte(cfg))
}

// handleTokenQR serves a single peer's client config as a QR image,
// identified by its client token and without requiring a login.
func (s *Server) handleTokenQR(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.mgr.ClientConfigByToken(r.PathValue("token"))
	if err != nil {
		writeManagerError(w, err)
		return
	}
	png, err := manager.ConfigQR(cfg)
	if err != nil {
		writeError(w, 500, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(png)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func writeManagerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, manager.ErrNotFound):
		writeError(w, 404, err)
	case errors.Is(err, manager.ErrConflict):
		writeError(w, 409, err)
	default:
		writeError(w, 400, err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func attachment(name string) string {
	return `attachment; filename="` + name + `"`
}
