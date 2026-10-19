package source

import (
	"math"
	"os"
	"testing"
)

func TestParseCountryStats(t *testing.T) {
	data, err := os.ReadFile("testdata/damodaran.xls")
	if err != nil {
		t.Fatal(err)
	}
	vals, at, err := parseCountryStats(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := at.Format("2006-01-02"); got != "2026-01-05" {
		t.Fatalf("as-of: want 2026-01-05, got %s", got)
	}

	aus, ok := vals["AUS"]
	if !ok {
		t.Fatalf("AUS missing, got %v", vals)
	}
	if math.Abs(aus.PB-2.35) > 1e-9 {
		t.Fatalf("AUS P/B: want 2.35, got %v", aus.PB)
	}
	if math.Abs(aus.EVEBIT-16.55) > 1e-9 {
		t.Fatalf("AUS EV/EBIT: want 16.55 (not EV/EBITDA), got %v", aus.EVEBIT)
	}
	if math.Abs(aus.PE-21.57) > 1e-9 {
		t.Fatalf("AUS P/E: want 21.57, got %v", aus.PE)
	}
	if aus.Firms != 1454 {
		t.Fatalf("AUS firms: want 1454, got %d", aus.Firms)
	}
	if math.Abs(aus.MarketCap-1236145.39) > 1e-6 {
		t.Fatalf("AUS market cap: got %v", aus.MarketCap)
	}
	if math.Abs(aus.DivYield-0.0256) > 1e-9 {
		t.Fatalf("AUS dividend yield: got %v", aus.DivYield)
	}

	if _, ok := vals["JPN"]; !ok {
		t.Fatal("JPN missing")
	}
	// The fixture's "Atlantis" is not in the country table and must be skipped.
	if len(vals) != 2 {
		t.Fatalf("want only the two known countries, got %d", len(vals))
	}
}
