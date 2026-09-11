package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const sessionCookie = "skyfire_session"
const sessionTTL = 24 * time.Hour

// chain assembles the full middleware chain and SPA static handler. Auth is
// enforced everywhere except /api/login and /api/logout.
func (s *Server) chain() http.Handler {
	// static + SPA fallback for everything not matched by the API mux
	switch {
	case s.staticFS != nil:
		s.mux.Handle("/", spaFS(s.staticFS))
	case s.static != "":
		s.mux.Handle("/", spaHandler(s.static))
	default:
		s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeError(w, 404, errors.New("not found"))
				return
			}
			http.NotFound(w, r)
		})
	}
	var h http.Handler = s.mux
	h = cors(h)
	if s.log != nil {
		h = logging(s.log, h)
	}
	h = s.auth(h)
	return h
}

// authorized reports whether a request may proceed.
func (s *Server) authorized(r *http.Request) bool {
	// legacy bearer token (scripts / non-browser clients)
	if s.token != "" && r.Header.Get("Authorization") == "Bearer "+s.token {
		return true
	}
	// single-user session cookie
	if _, ok := s.validSession(r); ok {
		return true
	}
	// nothing configured: open (unauthenticated) mode
	return s.token == "" && s.password == ""
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isAPI := strings.HasPrefix(r.URL.Path, "/api/")
		// login/logout are always reachable; the static SPA shell is public so
		// the login screen can load. Everything else under /api requires auth.
		if !isAPI || r.URL.Path == "/api/login" || r.URL.Path == "/api/logout" {
			next.ServeHTTP(w, r)
			return
		}
		if !s.authorized(r) {
			writeError(w, http.StatusUnauthorized, errors.New("unauthorized"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) validSession(r *http.Request) (string, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return "", false
	}
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	exp, ok := s.sessions[c.Value]
	if !ok || time.Now().After(exp) {
		delete(s.sessions, c.Value)
		return "", false
	}
	return c.Value, true
}

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	id := make([]byte, 32)
	_, _ = rand.Read(id)
	token := hex.EncodeToString(id)
	now := time.Now()
	s.sessionMu.Lock()
	if len(s.sessions) > 100 {
		for k, exp := range s.sessions {
			if now.After(exp) {
				delete(s.sessions, k)
			}
		}
	}
	s.sessions[token] = now.Add(sessionTTL)
	s.sessionMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) destroySession(w http.ResponseWriter, r *http.Request) {
	if id, ok := s.validSession(r); ok {
		s.sessionMu.Lock()
		delete(s.sessions, id)
		s.sessionMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil,
	})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Write(b []byte) (int, error) {
	n, err := sr.ResponseWriter.Write(b)
	sr.bytes += n
	return n, err
}

func logging(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sr := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(sr, r)
		log.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sr.status,
			"bytes", sr.bytes,
			"duration", time.Since(start).Round(time.Millisecond).String(),
		)
	})
}

// spaHandler serves the built frontend with an index.html fallback for the
// client-side router.
func spaHandler(dir string) http.Handler {
	root := filepath.Clean(dir)
	index := filepath.Join(root, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleanPath := strings.TrimPrefix(filepath.Clean(filepath.FromSlash(r.URL.Path)), ".")

		candidate := filepath.Join(root, cleanPath)
		if !strings.HasPrefix(candidate, root+string(os.PathSeparator)) {
			http.NotFound(w, r)
			return
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			http.ServeFile(w, r, candidate)
			return
		}
		if _, err := os.Stat(index); err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, index)
	})
}

// spaFS serves the embedded frontend with an index.html fallback for the
// client-side router.
func spaFS(fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, 404, errors.New("not found"))
			return
		}
		clean := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		name := clean
		f, err := fsys.Open(clean)
		if err != nil {
			name = "index.html"
			f, err = fsys.Open(name)
			if err != nil {
				http.NotFound(w, r)
				return
			}
		}
		defer f.Close()
		if fi, err := f.Stat(); err == nil && fi.IsDir() {
			_ = f.Close()
			name = strings.TrimSuffix(clean, "/") + "/index.html"
			if clean == "" {
				name = "index.html"
			}
			f, err = fsys.Open(name)
			if err != nil {
				name = "index.html"
				f, err = fsys.Open(name)
				if err != nil {
					http.NotFound(w, r)
					return
				}
			}
			defer f.Close()
		}
		fi, err := f.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(name)))
		if rs, ok := f.(io.ReadSeeker); ok {
			http.ServeContent(w, r, name, fi.ModTime(), rs)
			return
		}
		io.Copy(w, f)
	})
}
