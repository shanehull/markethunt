package web

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/shanehull/markethunt/internal/model"
)

type stubStore struct{ snap *model.Snapshot }

func (s stubStore) Snapshot() *model.Snapshot { return s.snap }

func TestHealthz(t *testing.T) {
	srv, err := New(stubStore{}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("healthz: %d %q", rec.Code, rec.Body.String())
	}
}

func TestReadyz(t *testing.T) {
	loading, err := New(stubStore{}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	loading.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 before ingest, got %d", rec.Code)
	}

	ready, err := New(stubStore{snap: &model.Snapshot{}}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	ready.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 after ingest, got %d", rec.Code)
	}
}

func TestLoadingStateBeforeIngest(t *testing.T) {
	srv, err := New(stubStore{}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("index: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Fetching country multiples") {
		t.Fatal("expected the loading state")
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "nonce-") || !strings.Contains(csp, "script-src") {
		t.Fatalf("unexpected CSP: %q", csp)
	}
}

func TestUnknownCountry(t *testing.T) {
	srv, err := New(stubStore{}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/country/ZZZ", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rec.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	srv, err := New(stubStore{}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	h := rec.Header()

	for k, want := range map[string]string{
		"X-Content-Type-Options":       "nosniff",
		"X-Frame-Options":              "DENY",
		"Referrer-Policy":              "no-referrer",
		"Cross-Origin-Opener-Policy":   "same-origin",
		"Cross-Origin-Resource-Policy": "same-origin",
	} {
		if got := h.Get(k); got != want {
			t.Fatalf("%s: want %q, got %q", k, want, got)
		}
	}
	if h.Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("CORS must be absent for a same-origin app")
	}
	if h.Get("Strict-Transport-Security") == "" {
		t.Fatal("missing HSTS")
	}
	if got := h.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("dynamic HTML should not be cached: %q", got)
	}
	csp := h.Get("Content-Security-Policy")
	for _, part := range []string{
		"default-src 'self'",
		"frame-ancestors 'none'",
		"script-src-attr 'none'",
		"connect-src 'self'",
		"object-src 'none'",
	} {
		if !strings.Contains(csp, part) {
			t.Fatalf("CSP missing %q: %s", part, csp)
		}
	}
}

// TestNonceMatchesBody checks the per-request nonces in the CSP are the ones the
// page actually uses, so the inline style block and JSON-LD are authorised.
func TestNonceMatchesBody(t *testing.T) {
	srv, err := New(stubStore{}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	csp := rec.Header().Get("Content-Security-Policy")
	matches := regexp.MustCompile(`'nonce-([a-f0-9]+)'`).FindAllStringSubmatch(csp, -1)
	if len(matches) < 2 {
		t.Fatalf("want script and style nonces, got %v", matches)
	}
	body := rec.Body.String()
	for _, m := range matches {
		if !strings.Contains(body, `nonce="`+m[1]+`"`) {
			t.Fatalf("CSP nonce %s does not appear in the page", m[1])
		}
	}
}

func TestMethodNotAllowed(t *testing.T) {
	srv, err := New(stubStore{}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d", rec.Code)
	}
}

func TestStaticCacheControl(t *testing.T) {
	srv, err := New(stubStore{}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/app.css", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "max-age=86400") {
		t.Fatalf("unexpected Cache-Control: %q", cc)
	}
}

// TestDetailCharts checks the drawer renders Chart.js canvases and the data
// payload, rather than the old server-side sparkline.
func TestDetailCharts(t *testing.T) {
	snap := &model.Snapshot{Countries: []model.Country{{
		ISO3:      "AUS",
		Name:      "Australia",
		Region:    "East Asia & Pacific",
		Valuation: &model.Valuation{PB: 2, EVEBIT: 16, Firms: 100},
		Macro: &model.Macro{Growth: &model.Growth{
			Average: 2.5,
			Years:   5,
			History: []model.Point{
				{Date: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC), Value: 2.1},
				{Date: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC), Value: 2.9},
			},
		}},
		Scores: model.Scores{Rank: 1, Composite: 0.5, Pillars: 6},
	}}}
	srv, err := New(stubStore{snap: snap}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/country/AUS", nil))
	body := rec.Body.String()
	if !strings.Contains(body, `id="drawer-chart-growth"`) {
		t.Fatal("missing Chart.js canvas")
	}
	if !strings.Contains(body, `data-charts="`) {
		t.Fatal("missing chart data payload")
	}
	if strings.Contains(body, "<svg class=\"spark\"") {
		t.Fatal("sparkline SVG should be gone")
	}
}

// TestShutdownDrainsInFlight proves Shutdown waits for a handler that is still
// running, rather than cutting the connection.
func TestShutdownDrainsInFlight(t *testing.T) {
	srv, err := New(stubStore{}, Options{ShutdownTimeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	srv.httpSrv.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.httpSrv.Serve(ln) }()
	go func() { _, _ = http.Get("http://" + ln.Addr().String()) }()

	<-started
	done := make(chan error, 1)
	go func() { done <- srv.httpSrv.Shutdown(context.Background()) }()

	select {
	case err := <-done:
		t.Fatalf("Shutdown returned before the in-flight request finished: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func row(iso string, composite, pb float64) model.Country {
	c := model.Country{ISO3: iso, Name: iso}
	c.Scores.Composite = composite
	if pb > 0 {
		c.Valuation = &model.Valuation{PB: pb}
	}
	return c
}

func TestSortRowsCompositeDesc(t *testing.T) {
	rows := []model.Country{
		row("AAA", 0.2, 0),
		row("BBB", 0.9, 0),
		row("CCC", 0.5, 0),
	}
	sortRows(rows, "composite", "desc")
	if rows[0].ISO3 != "BBB" || rows[2].ISO3 != "AAA" {
		t.Fatalf("want BBB..AAA, got %s..%s", rows[0].ISO3, rows[2].ISO3)
	}
}

func TestSortRowsPBAscending(t *testing.T) {
	rows := []model.Country{
		row("AAA", 0, 3.0),
		row("BBB", 0, 1.0),
		row("CCC", 0, 0), // missing metric sorts last
	}
	sortRows(rows, "pb", "asc")
	if rows[0].ISO3 != "BBB" || rows[1].ISO3 != "AAA" || rows[2].ISO3 != "CCC" {
		t.Fatalf("unexpected order: %s, %s, %s", rows[0].ISO3, rows[1].ISO3, rows[2].ISO3)
	}
}

func TestMetricMissing(t *testing.T) {
	if _, ok := metric(row("AAA", 0, 0), "pb"); ok {
		t.Fatal("expected pb to be missing")
	}
	if v, ok := metric(row("AAA", 0, 2.5), "pb"); !ok || v != 2.5 {
		t.Fatalf("want 2.5, got %v ok=%v", v, ok)
	}
}

func TestFormatting(t *testing.T) {
	if got := num(0, 2); got != "—" {
		t.Fatalf("zero should render as an em dash, got %q", got)
	}
	if got := num(1.5, 2); got != "1.50" {
		t.Fatalf("want 1.50, got %q", got)
	}
	if got := signed(-1.2, 1); got != "-1.2" {
		t.Fatalf("want -1.2, got %q", got)
	}
	if got := signed(1.2, 1); got != "+1.2" {
		t.Fatalf("want +1.2, got %q", got)
	}
	if got := pct(0.62); got != "62" {
		t.Fatalf("want 62, got %q", got)
	}
	if scoreClass(0.7) != "good" || scoreClass(0.4) != "mid" || scoreClass(0.1) != "poor" {
		t.Fatal("score bucketing is wrong")
	}
}

func TestExportCSV(t *testing.T) {
	snap := &model.Snapshot{Countries: []model.Country{
		{
			ISO3: "AUS", Name: "Australia", Region: "East Asia & Pacific",
			Valuation: &model.Valuation{PB: 2.35, EVEBIT: 16.5, PE: 21.6, DivYield: 0.025},
			Macro: &model.Macro{
				Growth:  &model.Growth{Average: 2.3, Volatility: 1.2},
				Savings: &model.Savings{Rate: 22},
				Trade:   &model.Trade{Balance: 1.2},
				Freedom: &model.Freedom{Score: 8, Change: -0.1},
			},
			Scores: model.Scores{Rank: 1, Composite: 0.7},
		},
		{
			ISO3: "JPN", Name: "Japan", Region: "East Asia & Pacific",
			Scores: model.Scores{Rank: 2, Composite: 0.5},
		},
	}}
	srv, err := New(stubStore{snap: snap}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/export.csv", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/csv") {
		t.Fatalf("content type: %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Fatalf("content disposition: %q", cd)
	}
	lines := strings.Split(strings.TrimSpace(rec.Body.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("want header plus two rows, got %d:\n%s", len(lines), rec.Body.String())
	}
	if !strings.Contains(lines[0], "ev_ebit") || !strings.Contains(lines[0], "debt_to_gdp_pct") {
		t.Fatalf("unexpected header: %s", lines[0])
	}
	if !strings.Contains(lines[1], "Australia") || !strings.Contains(lines[1], "16.5") {
		t.Fatalf("unexpected first row: %s", lines[1])
	}

	// The export honours the active filter.
	rec = httptest.NewRecorder()
	srv.routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/export.csv?q=Japan", nil))
	lines = strings.Split(strings.TrimSpace(rec.Body.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[1], "Japan") {
		t.Fatalf("filtered export: %s", rec.Body.String())
	}
}

func TestCSVSafe(t *testing.T) {
	for in, want := range map[string]string{
		"=1+1":   "'=1+1",
		"+cmd":   "'+cmd",
		"-cmd":   "'-cmd",
		"@cmd":   "'@cmd",
		"normal": "normal",
	} {
		if got := csvSafe(in); got != want {
			t.Fatalf("csvSafe(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSortURLTogglesDirection(t *testing.T) {
	current := view{Sort: "composite", Dir: "desc", Region: "Europe & Central Asia"}
	got := sortURL(current, "composite")
	if got != "/?dir=asc&region=Europe+%26+Central+Asia&sort=composite" {
		t.Fatalf("unexpected URL: %s", got)
	}
	// A new column takes its default direction and keeps the region.
	got = sortURL(current, "pb")
	if got != "/?dir=asc&region=Europe+%26+Central+Asia&sort=pb" {
		t.Fatalf("unexpected URL: %s", got)
	}
}
