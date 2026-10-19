// Package ingest orchestrates the fetchers and assembles the ranked snapshot.
package ingest

import (
	"context"
	"sync"
	"time"

	"github.com/shanehull/markethunt/internal/cache"
	"github.com/shanehull/markethunt/internal/ranking"
	"github.com/shanehull/markethunt/internal/source"
)

// Result reports what one fetcher produced.
type Result struct {
	Source string
	Err    error
}

// Run fetches every source concurrently, merges their datasets, ranks the
// countries, and stores the snapshot. A failing source never blocks the others;
// a partial dataset is still stored alongside its error.
func Run(ctx context.Context, store *cache.Store, cfg source.Config, fetchers []source.Fetcher) []Result {
	datasets := make([]source.Dataset, len(fetchers))
	results := make([]Result, len(fetchers))
	var wg sync.WaitGroup
	for i, f := range fetchers {
		wg.Go(func() {
			results[i] = Result{Source: f.Name()}
			ds, err := f.Fetch(ctx, cfg)
			datasets[i] = ds
			results[i].Err = err
		})
	}
	wg.Wait()

	merged := source.Merge(datasets)
	snap := ranking.Build(merged.Valuations, merged.Macros, time.Now().UTC())
	snap.ValuationAt = merged.ValuationAt
	store.Put(snap)
	return results
}
