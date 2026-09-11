package api

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"skyfire/internal/driver"
	"skyfire/internal/manager"
	"skyfire/internal/store"
)

// Server exposes the manager over HTTP and optionally serves the built
// frontend.
type Server struct {
	mgr      *manager.Manager
	drv      driver.Driver
	dryRun   bool
	static   string
	staticFS fs.FS
	token    string
	username string
	password string
	log      *slog.Logger
	mux      *http.ServeMux
	handler  http.Handler

	sessionMu sync.Mutex
	sessions  map[string]time.Time
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
	Log      *slog.Logger
}

// New builds the HTTP handler.
func New(mgr *manager.Manager, drv driver.Driver, dryRun bool, opts Options) *Server {
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}
	s := &Server{
		mgr:      mgr,
		drv:      drv,
		dryRun:   dryRun,
		static:   opts.StaticDir,
		staticFS: opts.StaticFS,
		token:    opts.Token,
		username: opts.Username,
		password: opts.Password,
		log:      log,
		mux:      http.NewServeMux(),
		sessions: make(map[string]time.Time),
	}
	s.routes()
	s.handler = s.chain()
	return s
}

func (s *Server) routes() {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
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
	if in.Username != s.username || in.Password != s.password {
		writeError(w, http.StatusUnauthorized, errors.New("invalid credentials"))
		return
	}
	s.createSession(w, r)
	writeJSON(w, 200, map[string]any{"ok": true})
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
		"status": "ok",
		"driver": s.drv.Name(),
		"dryRun": s.dryRun,
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
