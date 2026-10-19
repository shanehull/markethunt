// Package web serves the Markethunt ranking dashboard.
package web

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/shanehull/markethunt/internal/model"
)

//go:embed templates/*.html static/*
var files embed.FS

// Store is the subset of the store the web layer needs.
type Store interface {
	Snapshot() *model.Snapshot
}

// Options configures the HTTP server.
type Options struct {
	Addr            string
	ShutdownTimeout time.Duration
	Logger          *slog.Logger
}

// Server renders the ranking dashboard.
type Server struct {
	store Store
	opts  Options

	tmpl    *template.Template
	static  fs.FS
	css     template.CSS
	httpSrv *http.Server
}

// New builds a Server and its underlying HTTP server.
func New(st Store, opts Options) (*Server, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.ShutdownTimeout == 0 {
		opts.ShutdownTimeout = 10 * time.Second
	}
	funcs := template.FuncMap{
		"pct":           pct,
		"scoreClass":    scoreClass,
		"sortURL":       sortURL,
		"sortMark":      sortMark,
		"exportURL":     exportURL,
		"pb":            priceBook,
		"evEbit":        evEbit,
		"pe":            priceEarnings,
		"dividend":      dividend,
		"gdp":           gdp,
		"gdpVol":        gdpVol,
		"savings":       savingsRate,
		"trade":         tradeBalance,
		"freedom":       freedomScore,
		"freedomChange": freedomChange,
		"freedomYears":  freedomYears,
		"debt":          debt,
		"debtYear":      debtYear,
		"gdpYears":      growthYears,
		"firms":         firms,
	}
	tmpl, err := template.New("").Funcs(funcs).ParseFS(files, "templates/*.html")
	if err != nil {
		return nil, err
	}
	static, err := fs.Sub(files, "static")
	if err != nil {
		return nil, err
	}
	css, err := fs.ReadFile(files, "static/app.css")
	if err != nil {
		return nil, err
	}

	s := &Server{store: st, opts: opts, tmpl: tmpl, static: static, css: template.CSS(css)}
	s.httpSrv = &http.Server{
		Addr:              opts.Addr,
		Handler:           s.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
		ErrorLog:          slog.NewLogLogger(opts.Logger.Handler(), slog.LevelWarn),
	}
	return s, nil
}

func (s *Server) routes() http.Handler {
	static := http.StripPrefix("/static/", cacheControl(http.FileServer(http.FS(s.static))))
	mux := http.NewServeMux()
	mux.Handle("GET /static/", static)
	mux.HandleFunc("GET /{$}", s.index)
	mux.HandleFunc("GET /export.csv", s.exportCSV)
	mux.HandleFunc("GET /country/{iso}", s.detail)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if s.store.Snapshot() == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"loading"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})
	return s.logRequests(gzipHandler(secureHeaders(mux)))
}

// Run serves until ctx is cancelled, then drains in-flight requests. It binds
// before logging, so a port conflict fails fast instead of claiming to listen.
func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.opts.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- s.httpSrv.Serve(ln) }()
	s.opts.Logger.InfoContext(ctx, "markethunt listening", slog.String("addr", ln.Addr().String()))

	select {
	case <-ctx.Done():
		s.opts.Logger.InfoContext(ctx, "shutdown signalled, draining")
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.opts.ShutdownTimeout)
	defer cancel()
	if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	s.opts.Logger.Info("shutdown complete")
	return nil
}

// --- Middleware -----------------------------------------------------------------

type nonceKey struct{}

type nonces struct {
	Script string
	Style  string
}

func newNonces() (nonces, error) {
	script, err := randomHex(16)
	if err != nil {
		return nonces{}, err
	}
	style, err := randomHex(16)
	if err != nil {
		return nonces{}, err
	}
	return nonces{Script: script, Style: style}, nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func contentSecurityPolicy(n nonces) string {
	return strings.Join([]string{
		"default-src 'self'",
		"base-uri 'none'",
		"script-src 'self' " + cspScriptHashes() + " 'nonce-" + n.Script + "'",
		"script-src-attr 'none'",
		"style-src 'self' 'nonce-" + n.Style + "'",
		"img-src 'self' data:",
		"font-src 'self'",
		"connect-src 'self' https://cdn.jsdelivr.net",
		"form-action 'self'",
		"frame-ancestors 'none'",
		"frame-src 'none'",
		"object-src 'none'",
		"media-src 'none'",
		"worker-src 'none'",
		"manifest-src 'self'",
	}, "; ")
}

// secureHeaders sets one nonce pair per request and the security headers, and
// exposes the nonces to the handler via context. There is deliberately no CORS
// header: the app is same-origin, and an absent Access-Control-Allow-Origin is
// what makes browsers block cross-origin reads.
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, err := newNonces()
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy(n))
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=(), usb=()")
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), nonceKey{}, n)))
	})
}

func cacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		next.ServeHTTP(w, r)
	})
}

func gzipHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		gw := &gzipWriter{ResponseWriter: w, gz: gzip.NewWriter(w)}
		next.ServeHTTP(gw, r)
		if gw.compressed {
			_ = gw.gz.Close()
		}
	})
}

type gzipWriter struct {
	http.ResponseWriter
	gz          *gzip.Writer
	compressed  bool
	wroteHeader bool
}

func (w *gzipWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	if compressible(w.Header().Get("Content-Type")) {
		w.compressed = true
		w.Header().Del("Content-Length")
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.compressed {
		return w.gz.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

func compressible(contentType string) bool {
	if i := strings.IndexByte(contentType, ';'); i >= 0 {
		contentType = contentType[:i]
	}
	switch strings.TrimSpace(contentType) {
	case "text/html", "text/css", "text/plain", "text/xml",
		"application/javascript", "application/json", "application/xml", "image/svg+xml":
		return true
	default:
		return false
	}
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		s.opts.Logger.InfoContext(r.Context(), "request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", sw.status),
			slog.Duration("dur", time.Since(start)),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// --- Handlers -------------------------------------------------------------------

// index serves the full page, or just the table fragment for an htmx request.
// Keeping both on "/" means sorting pushes a URL that survives a refresh.
func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	name := "index.html"
	if r.Header.Get("HX-Request") == "true" {
		name = "table.html"
	}
	s.render(w, r, name, s.buildView(r))
}

func (s *Server) detail(w http.ResponseWriter, r *http.Request) {
	iso := r.PathValue("iso")
	snap := s.store.Snapshot()
	var found *model.Country
	if snap != nil {
		for i := range snap.Countries {
			if snap.Countries[i].ISO3 == iso {
				found = &snap.Countries[i]
				break
			}
		}
	}
	if found == nil {
		http.Error(w, "unknown country", http.StatusNotFound)
		return
	}
	charts := make([]chart, 0, 2)
	if m := found.Macro; m != nil {
		if g := m.Growth; g != nil && len(g.History) > 1 {
			charts = append(charts, chart{ID: "growth", Label: "GDP growth, % a year", Unit: "%", Points: g.History})
		}
		if f := m.Freedom; f != nil && len(f.History) > 1 {
			charts = append(charts, chart{ID: "freedom", Label: "Economic freedom, 0-10", Unit: "", Points: f.History})
		}
		if d := m.Debt; d != nil && len(d.History) > 1 {
			charts = append(charts, chart{ID: "debt", Label: "Government debt, % of GDP", Unit: "%", Points: d.History})
		}
	}
	raw, err := json.Marshal(charts)
	if err != nil {
		s.opts.Logger.ErrorContext(r.Context(), "chart encode failed", slog.Any("error", err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	s.render(w, r, "detail.html", detailView{
		Country:   *found,
		Total:     len(snap.Countries),
		Charts:    charts,
		ChartJSON: string(raw),
	})
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		s.opts.Logger.ErrorContext(r.Context(), "render failed", slog.String("template", name), slog.Any("error", err))
	}
}
