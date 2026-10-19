// Command markethunt serves a dashboard that ranks every market by the
// conditions a long-horizon buyer wants: cheap valuations, positive growth,
// high savings, a trade surplus, manageable government debt, and open
// institutions.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/shanehull/markethunt/internal/buildinfo"
	"github.com/shanehull/markethunt/internal/cache"
	"github.com/shanehull/markethunt/internal/ingest"
	"github.com/shanehull/markethunt/internal/source"
	"github.com/shanehull/markethunt/internal/web"
)

func main() {
	if err := run(context.Background()); err != nil {
		slog.Error("markethunt exited", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	addr := flag.String("addr", env("MARKETHUNT_ADDR", ":8080"), "HTTP listen address")
	refresh := flag.Duration("refresh", time.Hour, "re-ingest interval (0 disables)")
	cacheDir := flag.String("cache", defaultCacheDir(), "cache directory for downloaded payloads (empty disables)")
	cacheTTL := flag.Duration("cache-ttl", 24*time.Hour, "how long a downloaded payload is reused before refetching")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})).
		With(slog.String("version", buildinfo.GitTag))
	slog.SetDefault(log)

	// A dev supervisor that dies without reaping its child (air, a closed
	// terminal) would otherwise leave this process holding the port forever.
	go watchParent(ctx, log, time.Second, os.Getppid, func() {
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
	})

	store := cache.New()
	cfg := source.Config{CacheDir: *cacheDir, CacheTTL: *cacheTTL}
	runIngest := func(ctx context.Context) error {
		results := ingest.Run(ctx, store, cfg, source.All())
		var failed int
		for _, r := range results {
			if r.Err != nil {
				failed++
				log.WarnContext(ctx, "source problem", slog.String("source", r.Source), slog.Any("error", r.Err))
				continue
			}
			log.InfoContext(ctx, "source ok", slog.String("source", r.Source))
		}
		if failed == len(results) {
			return errors.New("every source failed")
		}
		return nil
	}

	ingestCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	// The Damodaran snapshots download from a slow host, so bind and serve
	// straight away and let the first ingest fill the page in the background.
	go func() {
		if err := runIngest(ingestCtx); err != nil {
			log.WarnContext(ctx, "initial ingest", slog.Any("error", err))
		}
	}()

	if *refresh > 0 {
		go refreshLoop(ctx, *refresh, log, runIngest)
	}

	srv, err := web.New(store, web.Options{
		Addr:   *addr,
		Logger: log,
	})
	if err != nil {
		return err
	}
	return srv.Run(ctx)
}

// refreshLoop re-ingests on a ticker so a long-running process picks up the
// annual multiples update without a restart.
func refreshLoop(ctx context.Context, every time.Duration, log *slog.Logger, run func(context.Context) error) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
			if err := run(runCtx); err != nil {
				log.WarnContext(ctx, "refresh ingest", slog.Any("error", err))
			}
			cancel()
		}
	}
}

// watchParent shuts down when the parent process disappears. A supervisor that
// is killed without cleaning up (air, a closed shell) otherwise leaves the child
// running and holding its port. Processes started by init or as PID 1 are
// exempt, which covers systemd and containers.
func watchParent(ctx context.Context, log *slog.Logger, every time.Duration, getppid func() int, stop func()) {
	parent := getppid()
	if parent <= 1 {
		return
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if getppid() != parent {
				log.Warn("parent exited, shutting down", slog.Int("parent", parent))
				stop()
				return
			}
		}
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// defaultCacheDir keeps downloaded payloads between runs. It falls back to no
// cache when there is no writable user cache directory, as in a scratch image.
func defaultCacheDir() string {
	if v := os.Getenv("MARKETHUNT_CACHE"); v != "" {
		return v
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "markethunt")
}
