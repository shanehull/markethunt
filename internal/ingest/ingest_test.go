package ingest

import (
	"context"
	"errors"
	"testing"

	"github.com/shanehull/markethunt/internal/cache"
	"github.com/shanehull/markethunt/internal/model"
	"github.com/shanehull/markethunt/internal/source"
)

type stub struct {
	name string
	ds   source.Dataset
	err  error
}

func (s stub) Name() string { return s.name }

func (s stub) Fetch(context.Context, source.Config) (source.Dataset, error) {
	return s.ds, s.err
}

func datasets() (source.Dataset, source.Dataset) {
	vals := source.Dataset{Valuations: map[string]model.Valuation{
		"AUS": {PB: 1, EVEBIT: 10, Firms: 100},
	}}
	macros := source.Dataset{Macros: map[string]model.Macro{
		"AUS": {
			Growth:  &model.Growth{Average: 2},
			Savings: &model.Savings{Rate: 20},
			Trade:   &model.Trade{Balance: 3},
			Freedom: &model.Freedom{Score: 7},
		},
	}}
	return vals, macros
}

func TestRunMergesAndRanks(t *testing.T) {
	vals, macros := datasets()
	store := cache.New()
	results := Run(context.Background(), store, source.Config{}, []source.Fetcher{
		stub{name: "damodaran", ds: vals},
		stub{name: "world bank", ds: macros},
	})
	for _, r := range results {
		if r.Err != nil {
			t.Fatalf("%s: %v", r.Source, r.Err)
		}
	}
	snap := store.Snapshot()
	if snap == nil || len(snap.Countries) != 1 || snap.Countries[0].ISO3 != "AUS" {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
}

func TestRunStoresPartialDatasetOnError(t *testing.T) {
	vals, macros := datasets()
	store := cache.New()
	results := Run(context.Background(), store, source.Config{}, []source.Fetcher{
		stub{name: "damodaran", ds: vals, err: errors.New("boom")},
		stub{name: "world bank", ds: macros},
	})
	if results[0].Err == nil {
		t.Fatal("expected the source error to be reported")
	}
	if store.Snapshot() == nil {
		t.Fatal("partial data should still be stored and ranked")
	}
}
