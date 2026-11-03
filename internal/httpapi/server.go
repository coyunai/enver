package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/coyunai/enver/internal/app"
	"github.com/coyunai/enver/internal/buildinfo"
	"github.com/coyunai/enver/internal/webui"
)

// Server serves the local API and the embedded web UI.
type Server struct {
	App    *app.App
	Token  string
	Auth   bool
	Assets fs.FS
}

// New builds a Server. When auth is true, a one-time token must be exchanged
// at /auth before any other route is available.
func New(a *app.App, token string, auth bool) *Server {
	assets, _ := webui.FS()
	return &Server{App: a, Token: token, Auth: auth, Assets: assets}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /auth", s.handleAuth)
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/session", s.session)
	mux.HandleFunc("GET /api/overview", s.overview)
	mux.HandleFunc("GET /api/vfox", s.vfoxInfo)
	mux.HandleFunc("PUT /api/vfox", s.setVfoxBin)
	mux.HandleFunc("GET /api/catalog", s.pluginCatalog)
	mux.HandleFunc("GET /api/sdks", s.listSDKs)
	mux.HandleFunc("POST /api/sdks/{sdk}/plugins", s.addPlugin)
	mux.HandleFunc("GET /api/sdks/{sdk}/versions", s.versions)
	mux.HandleFunc("POST /api/install", s.install)
	mux.HandleFunc("POST /api/uninstall", s.uninstall)
	mux.HandleFunc("POST /api/use", s.use)
	mux.HandleFunc("GET /api/presets", s.listPresets)
	mux.HandleFunc("POST /api/presets", s.savePreset)
	mux.HandleFunc("PUT /api/presets/{name}", s.savePreset)
	mux.HandleFunc("DELETE /api/presets/{name}", s.deletePreset)
	mux.HandleFunc("POST /api/presets/{name}/apply", s.applyPreset)
	mux.HandleFunc("GET /api/venvs", s.listVenvs)
	mux.HandleFunc("POST /api/venvs", s.createVenv)
	mux.HandleFunc("DELETE /api/venvs/{id}", s.deleteVenv)
	mux.HandleFunc("POST /api/venvs/{id}/rebuild", s.rebuildVenv)
	mux.HandleFunc("GET /api/packages", s.listPackages)
	mux.HandleFunc("GET /api/packages/export", s.exportPackages)
	mux.HandleFunc("GET /api/doctor", s.doctor)
	mux.HandleFunc("GET /api/storage", s.storage)
	mux.HandleFunc("PUT /api/storage", s.setStorage)
	mux.HandleFunc("POST /api/storage/migrate", s.migrateStorage)
	mux.HandleFunc("GET /api/jobs", s.listJobs)
	mux.HandleFunc("GET /api/jobs/{id}", s.getJob)
	mux.HandleFunc("DELETE /api/jobs/{id}", s.cancelJob)
	mux.HandleFunc("GET /api/jobs/{id}/events", s.jobEvents)
	mux.HandleFunc("GET /api/terminal", s.terminal)
	mux.HandleFunc("/", s.spa)
	return s.security(s.authenticate(mux))
}

func (s *Server) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self' ws: wss:; font-src 'self' data:; base-uri 'none'; frame-ancestors 'none'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if s.Auth && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || !strings.EqualFold(u.Host, r.Host) {
					apiError(w, http.StatusForbidden, "origin_denied", "cross-origin request denied")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	if !s.Auth {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth", "/api/health":
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie("enver_session")
		if err == nil && subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(s.Token)) == 1 {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiError(w, http.StatusUnauthorized, "unauthorized", "session expired; reopen the URL printed by enver ui")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `<!doctype html><meta charset="utf-8"><title>enver</title>
<body style="font:14px/1.6 system-ui;background:#0d1117;color:#e6edf3;padding:48px">
<h2>需要会话令牌</h2>
<p>请使用 <code>enver ui</code> 输出的完整地址打开，或重新启动 UI。</p>`)
	})
}

func (s *Server) handleAuth(w http.ResponseWriter, r *http.Request) {
	if !s.Auth {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	token := r.URL.Query().Get("token")
	if subtle.ConstantTimeCompare([]byte(token), []byte(s.Token)) != 1 {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "enver_session",
		Value:    s.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   12 * 60 * 60,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) spa(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		apiError(w, http.StatusNotFound, "not_found", "unknown API route")
		return
	}
	rel := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if rel == "" || rel == "." {
		rel = "index.html"
	}
	if data, err := fs.ReadFile(s.Assets, rel); err == nil {
		if strings.HasPrefix(rel, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-store")
		}
		if ct := mime.TypeByExtension(path.Ext(rel)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		_, _ = w.Write(data)
		return
	}
	if path.Ext(rel) != "" {
		http.NotFound(w, r)
		return
	}
	data, err := fs.ReadFile(s.Assets, "index.html")
	if err != nil {
		http.Error(w, "web UI is not built", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		if err := json.NewEncoder(w).Encode(v); err != nil {
			log.Printf("write json: %v", err)
		}
	}
}

func apiError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}

func decodeJSON(r *http.Request, dst any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(dst)
}

type healthResponse struct {
	Status  string    `json:"status"`
	Version string    `json:"version"`
	Commit  string    `json:"commit"`
	Started time.Time `json:"startedAt"`
	Auth    bool      `json:"auth"`
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		Version: buildinfo.Version,
		Commit:  buildinfo.Commit,
		Started: s.App.Started,
		Auth:    s.Auth,
	})
}

func (s *Server) session(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"version":       buildinfo.Version,
		"commit":        buildinfo.Commit,
	})
}
