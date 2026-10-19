package ranking

import (
	"testing"
	"time"

	"github.com/shanehull/markethunt/internal/model"
)

func macro(growth, savings, trade, freedom, debt float64) *model.Macro {
	return &model.Macro{
		Growth:  &model.Growth{Average: growth},
		Savings: &model.Savings{Rate: savings},
		Trade:   &model.Trade{Balance: trade},
		Freedom: &model.Freedom{Score: freedom},
		Debt:    &model.Debt{Ratio: debt},
	}
}

func TestValuationDrivesRank(t *testing.T) {
	vals := map[string]model.Valuation{
		"AUS": {PB: 1.0, EVEBIT: 10, Firms: 100},
		"JPN": {PB: 3.0, EVEBIT: 30, Firms: 100},
	}
	macros := map[string]model.Macro{
		"AUS": *macro(2, 20, 0, 6, 60),
		"JPN": *macro(2, 20, 0, 6, 60),
	}
	snap := Build(vals, macros, time.Now())
	if snap.Countries[0].ISO3 != "AUS" {
		t.Fatalf("want the cheaper market first, got %s", snap.Countries[0].ISO3)
	}
}

// TestMacroScoresToo is the guard against a valuation-only ranking: with
// identical multiples, the better macro reading must rank higher.
func TestMacroScoresToo(t *testing.T) {
	vals := map[string]model.Valuation{
		"AUS": {PB: 1.0, EVEBIT: 10, Firms: 100},
		"JPN": {PB: 1.0, EVEBIT: 10, Firms: 100},
	}
	macros := map[string]model.Macro{
		"AUS": *macro(-1, 5, -10, 2, 120),
		"JPN": *macro(5, 40, 10, 9, 30),
	}
	snap := Build(vals, macros, time.Now())
	if snap.Countries[0].ISO3 != "JPN" {
		t.Fatalf("want the better macro reading first, got %s", snap.Countries[0].ISO3)
	}
}

func TestDebtScoresLower(t *testing.T) {
	vals := map[string]model.Valuation{
		"AUS": {PB: 1.0, EVEBIT: 10, Firms: 100},
		"JPN": {PB: 1.0, EVEBIT: 10, Firms: 100},
	}
	macros := map[string]model.Macro{
		"AUS": *macro(2, 20, 0, 6, 120),
		"JPN": *macro(2, 20, 0, 6, 30),
	}
	snap := Build(vals, macros, time.Now())
	if snap.Countries[0].ISO3 != "JPN" {
		t.Fatalf("want the lower-debt market first, got %s", snap.Countries[0].ISO3)
	}
}

func TestBuildDropsBelowFloor(t *testing.T) {
	vals := map[string]model.Valuation{
		"AUS": {PB: 1.0, EVEBIT: 10, Firms: 100},
		"BMU": {PB: 0.5, EVEBIT: 5, Firms: 5},
	}
	macros := map[string]model.Macro{"AUS": *macro(2, 20, 0, 6, 60), "BMU": *macro(2, 20, 0, 6, 60)}
	snap := Build(vals, macros, time.Now())
	if len(snap.Countries) != 1 || snap.Countries[0].ISO3 != "AUS" {
		t.Fatalf("want only AUS ranked, got %+v", snap.Countries)
	}
}

func TestBuildRequiresGrowth(t *testing.T) {
	vals := map[string]model.Valuation{"AUS": {PB: 1, EVEBIT: 10, Firms: 100}}
	if snap := Build(vals, map[string]model.Macro{}, time.Now()); len(snap.Countries) != 0 {
		t.Fatalf("want no ranked countries, got %d", len(snap.Countries))
	}
}

func TestBuildRequiresAValuationMultiple(t *testing.T) {
	vals := map[string]model.Valuation{"AUS": {Firms: 100}}
	macros := map[string]model.Macro{"AUS": *macro(2, 20, 0, 6, 60)}
	if snap := Build(vals, macros, time.Now()); len(snap.Countries) != 0 {
		t.Fatalf("want no ranked countries, got %d", len(snap.Countries))
	}
}

func TestScoresAreBoundedAndRanked(t *testing.T) {
	vals := map[string]model.Valuation{
		"AUS": {PB: 1.0, EVEBIT: 10, Firms: 100},
		"JPN": {PB: 2.0, EVEBIT: 20, Firms: 100},
		"CHE": {PB: 3.0, EVEBIT: 30, Firms: 100},
	}
	macros := map[string]model.Macro{
		"AUS": *macro(2, 20, 0, 6, 60),
		"JPN": *macro(3, 30, 5, 7, 80),
		"CHE": *macro(1, 40, 10, 9, 30),
	}
	snap := Build(vals, macros, time.Now())
	for i, c := range snap.Countries {
		if c.Scores.Composite < 0 || c.Scores.Composite > 1 {
			t.Fatalf("composite out of range: %v", c.Scores.Composite)
		}
		if c.Scores.Pillars != 7 {
			t.Fatalf("want 7 conditions, got %d", c.Scores.Pillars)
		}
		if c.Scores.Rank != i+1 {
			t.Fatalf("rank should be dense and ordered: got %d at %d", c.Scores.Rank, i)
		}
	}
}
