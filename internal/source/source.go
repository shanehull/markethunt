// Package source fetches raw data from external providers and normalises it into
// the model types the ranking layer assembles.
package source

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shanehull/markethunt/internal/model"
)

// Config controls how sources fetch.
type Config struct {
	Client *http.Client
	// CacheDir holds downloaded payloads so restarts and refreshes skip the
	// slow providers. Empty disables the cache.
	CacheDir string
	// CacheTTL bounds how long a mutable payload (the current multiples file
	// and the macro series) is reused. Zero means 24 hours.
	CacheTTL time.Duration
}

// defaultCacheTTL is the freshness bound for payloads that change.
const defaultCacheTTL = 24 * time.Hour

func (c Config) ttl() time.Duration {
	if c.CacheTTL > 0 {
		return c.CacheTTL
	}
	return defaultCacheTTL
}

// Dataset is the partial data one source contributes, keyed by ISO3.
type Dataset struct {
	Valuations  map[string]model.Valuation
	Macros      map[string]model.Macro
	ValuationAt time.Time // the as-of date of the equity multiples
}

// merge folds another dataset into this one.
func (d *Dataset) merge(o Dataset) {
	for k, v := range o.Valuations {
		if d.Valuations == nil {
			d.Valuations = map[string]model.Valuation{}
		}
		d.Valuations[k] = v
	}
	for k, v := range o.Macros {
		if d.Macros == nil {
			d.Macros = map[string]model.Macro{}
		}
		m := d.Macros[k]
		if v.Growth != nil {
			m.Growth = v.Growth
		}
		if v.Savings != nil {
			m.Savings = v.Savings
		}
		if v.Trade != nil {
			m.Trade = v.Trade
		}
		if v.Freedom != nil {
			m.Freedom = v.Freedom
		}
		if v.Debt != nil {
			m.Debt = v.Debt
		}
		d.Macros[k] = m
	}
	if !o.ValuationAt.IsZero() && o.ValuationAt.After(d.ValuationAt) {
		d.ValuationAt = o.ValuationAt
	}
}

// Merge folds several datasets into one.
func Merge(datasets []Dataset) Dataset {
	var out Dataset
	for _, d := range datasets {
		out.merge(d)
	}
	return out
}

// Fetcher retrieves a set of metrics from one provider.
type Fetcher interface {
	Name() string
	Fetch(ctx context.Context, cfg Config) (Dataset, error)
}

// All returns the default set of fetchers. Every source is public and keyless.
func All() []Fetcher {
	return []Fetcher{Damodaran{}, WorldBank{}, Fraser{}, IMF{}}
}

func (c Config) client() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return &http.Client{Timeout: 60 * time.Second}
}

// get fetches a URL with a browser-like User-Agent, which every provider here
// expects, and rejects non-200 responses with a short body excerpt.
func get(ctx context.Context, c Config, url, accept string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d: %s", url, resp.StatusCode, truncate(body, 200))
	}
	return body, nil
}

func truncate(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		return s[:n]
	}
	return s
}

// getCached returns a payload from the on-disk cache when it is younger than
// ttl, otherwise it fetches and stores it. The Damodaran host is slow, so this
// is what keeps restarts and background refreshes fast; a cache write failure is
// ignored, since the cache is an optimisation and never a source of truth.
func getCached(ctx context.Context, c Config, url, accept string, ttl time.Duration) ([]byte, error) {
	if c.CacheDir == "" || ttl <= 0 {
		return get(ctx, c, url, accept)
	}
	path := filepath.Join(c.CacheDir, cacheKey(url))
	if info, err := os.Stat(path); err == nil && time.Since(info.ModTime()) < ttl {
		if body, err := os.ReadFile(path); err == nil && len(body) > 0 {
			return body, nil
		}
	}
	body, err := get(ctx, c, url, accept)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(c.CacheDir, 0o755); err == nil {
		writeAtomic(path, body)
	}
	return body, nil
}

// writeAtomic writes through a temp file and rename, so a crash cannot leave a
// partially written cache entry that would then be reused until the TTL expires.
func writeAtomic(path string, body []byte) {
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return
	}
	name := f.Name()
	_, werr := f.Write(body)
	cerr := f.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(name)
		return
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
	}
}

func cacheKey(url string) string {
	sum := sha256.Sum256([]byte(url))
	return hex.EncodeToString(sum[:]) + ".bin"
}

// parseNumber reads a numeric cell, treating blanks and "NA" as missing.
func parseNumber(raw string) (float64, bool) {
	s := strings.TrimSpace(strings.ReplaceAll(raw, ",", ""))
	if s == "" || strings.EqualFold(s, "NA") || strings.EqualFold(s, "N/A") || s == "-" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}
