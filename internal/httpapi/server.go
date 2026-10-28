// Package httpapi is the dashboard, JSON API, and ping receiver.
package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/rejisterjack/tock/internal/app"
	"github.com/rejisterjack/tock/internal/auth"
	"github.com/rejisterjack/tock/internal/check"
	"github.com/rejisterjack/tock/internal/config"
	"github.com/rejisterjack/tock/internal/store"
	"github.com/rejisterjack/tock/web"
	"html/template"
)

// Server serves Tock.
type Server struct {
	cfg     config.Config
	svc     *app.Service
	auth    *auth.Manager
	log     *slog.Logger
	tmpl    *template.Template
	version string
	now     func() time.Time
}

// New parses templates and returns a server.
func New(cfg config.Config, svc *app.Service, log *slog.Logger, version string) (*Server, error) {
	tmpl, err := template.New("tock").Funcs(template.FuncMap{
		"utc": func(t time.Time) string { return t.UTC().Format("2006-01-02 15:04:05 UTC") },
		"iso": func(t time.Time) string { return t.UTC().Format(time.RFC3339) },
	}).ParseFS(web.FS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	if log == nil {
		log = slog.Default()
	}
	return &Server{
		cfg: cfg, svc: svc, log: log, tmpl: tmpl, version: version,
		auth: auth.New(cfg.AdminPassword, cfg.SessionSecret, cfg.APIToken, cfg.SecureCookies(), cfg.SessionLifetime),
		now:  time.Now,
	}, nil
}

// Handler is the root HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /ping/{token}", s.ping)
	mux.HandleFunc("POST /ping/{token}", s.ping)
	mux.Handle("GET /static/", s.static())

	mux.HandleFunc("GET /login", s.loginGET)
	mux.HandleFunc("POST /login", s.loginPOST)
	mux.HandleFunc("POST /logout", s.logout)

	mux.HandleFunc("GET /{$}", s.checksGET)
	mux.HandleFunc("GET /checks/new", s.checkNewGET)
	mux.HandleFunc("POST /checks", s.checkCreate)
	mux.HandleFunc("GET /checks/{id}", s.checkGET)
	mux.HandleFunc("GET /checks/{id}/edit", s.checkEditGET)
	mux.HandleFunc("POST /checks/{id}", s.checkUpdate)
	mux.HandleFunc("POST /checks/{id}/pause", s.checkPause)
	mux.HandleFunc("POST /checks/{id}/resume", s.checkResume)
	mux.HandleFunc("POST /checks/{id}/delete", s.checkDelete)

	mux.HandleFunc("GET /channels", s.channelsGET)
	mux.HandleFunc("POST /channels", s.channelCreate)
	mux.HandleFunc("POST /channels/{id}", s.channelUpdate)
	mux.HandleFunc("POST /channels/{id}/delete", s.channelDelete)

	mux.HandleFunc("GET /api/v1/checks", s.apiListChecks)
	mux.HandleFunc("POST /api/v1/checks", s.apiCreateCheck)
	mux.HandleFunc("GET /api/v1/checks/{id}", s.apiGetCheck)
	mux.HandleFunc("PATCH /api/v1/checks/{id}", s.apiPatchCheck)
	mux.HandleFunc("DELETE /api/v1/checks/{id}", s.apiDeleteCheck)
	mux.HandleFunc("POST /api/v1/checks/{id}/pause", s.apiPause)
	mux.HandleFunc("POST /api/v1/checks/{id}/resume", s.apiResume)
	mux.HandleFunc("GET /api/v1/checks/{id}/pings", s.apiPings)
	mux.HandleFunc("GET /api/v1/checks/{id}/alerts", s.apiAlerts)
	mux.HandleFunc("GET /api/v1/channels", s.apiListChannels)
	mux.HandleFunc("POST /api/v1/channels", s.apiCreateChannel)
	mux.HandleFunc("PATCH /api/v1/channels/{id}", s.apiPatchChannel)
	mux.HandleFunc("DELETE /api/v1/channels/{id}", s.apiDeleteChannel)

	return s.recover(s.logRequests(s.secure(mux)))
}

func (s *Server) static() http.Handler {
	sub, err := fs.Sub(web.FS, "static")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		http.StripPrefix("/static/", files).ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": s.version})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.Repo.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ping(w http.ResponseWriter, r *http.Request) {
	err := s.svc.Ping(r.Context(), r.PathValue("token"), remoteIP(r, s.cfg.TrustProxy), r.UserAgent())
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "not found\n", http.StatusNotFound)
		return
	}
	if err != nil {
		s.log.Error("ping", "err", err)
		http.Error(w, "error\n", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "OK\n")
}

func (s *Server) loginGET(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.auth.SessionCSRF(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	token, err := s.auth.SetLoginCSRF(w)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.render(w, http.StatusOK, "login", Page{Title: "Sign in", CSRF: token, Demo: s.cfg.Demo, Version: s.version})
}

func (s *Server) loginPOST(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	if !s.auth.LoginCSRFOK(r) {
		http.Error(w, "invalid CSRF token", http.StatusBadRequest)
		return
	}
	if !s.auth.PasswordOK(r.FormValue("password")) {
		token := ""
		if c, err := r.Cookie("tock_csrf"); err == nil {
			token = c.Value
		}
		s.render(w, http.StatusUnauthorized, "login", Page{Title: "Sign in", CSRF: token, Error: "Invalid password", Demo: s.cfg.Demo, Version: s.version})
		return
	}
	if _, err := s.auth.StartSession(w); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	if _, ok := s.auth.SessionCSRF(r); !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !s.auth.CSRFOK(r) {
		http.Error(w, "invalid CSRF token", http.StatusBadRequest)
		return
	}
	s.auth.ClearSession(w)
	dest := "/login"
	if s.cfg.Demo {
		dest = "/"
	}
	http.Redirect(w, r, dest, http.StatusSeeOther)
}

func (s *Server) view(w http.ResponseWriter, r *http.Request) (Page, bool) {
	csrf, authed := s.auth.SessionCSRF(r)
	if !authed && !s.cfg.Demo {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return Page{}, false
	}
	return Page{
		Demo: s.cfg.Demo, Authed: authed, CSRF: csrf, Version: s.version,
	}, true
}

func (s *Server) mutate(w http.ResponseWriter, r *http.Request) (Page, bool) {
	if s.cfg.Demo {
		http.Error(w, "demo mode is read-only", http.StatusForbidden)
		return Page{}, false
	}
	csrf, ok := s.auth.SessionCSRF(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return Page{}, false
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return Page{}, false
	}
	if !s.auth.CSRFOK(r) {
		http.Error(w, "invalid CSRF token", http.StatusBadRequest)
		return Page{}, false
	}
	return Page{Demo: false, Authed: true, CSRF: csrf, Version: s.version}, true
}

func (s *Server) render(w http.ResponseWriter, status int, name string, page Page) {
	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, name, page); err != nil {
		s.log.Error("render", "template", name, "err", err)
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) clock() time.Time {
	if s.svc != nil && s.svc.Now != nil {
		return s.svc.Now().UTC()
	}
	if s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func (s *Server) writeErr(w http.ResponseWriter, err error) {
	var ve *check.ValidationError
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	case errors.Is(err, app.ErrReadOnly):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
	case errors.As(err, &ve):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": ve.Error()})
	default:
		s.log.Error("request failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
}

func (s *Server) apiAuth(w http.ResponseWriter, r *http.Request, mutating bool) bool {
	if r.Header.Get("Authorization") != "" {
		tok, ok := auth.Bearer(r)
		if !ok || !s.auth.TokenOK(tok) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return false
		}
		return true
	}
	if _, ok := s.auth.SessionCSRF(r); !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return false
	}
	if mutating && !s.auth.CSRFOK(r) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid CSRF token"})
		return false
	}
	return true
}

func (s *Server) readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(v); err != nil {
		http.Error(w, "json error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-eval'; style-src 'self'; img-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		s.log.Info("request", "method", r.Method, "path", redactPath(r.URL.Path), "status", sw.status, "ms", time.Since(start).Milliseconds())
	})
}

func (s *Server) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("panic", "err", rec, "stack", string(debug.Stack()))
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func redactPath(p string) string {
	if strings.HasPrefix(p, "/ping/") {
		return "/ping/:token"
	}
	return p
}

func remoteIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if fwd := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); fwd != "" {
			first, _, _ := strings.Cut(fwd, ",")
			first = strings.TrimSpace(first)
			if first != "" {
				return first
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func queryLimit(r *http.Request) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	return n
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
