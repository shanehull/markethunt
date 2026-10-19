package source

import (
	"math"
	"os"
	"testing"
)

func TestParseFraser(t *testing.T) {
	data, err := os.ReadFile("testdata/efw.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseFraser(data)
	if err != nil {
		t.Fatal(err)
	}

	aus, ok := got["AUS"]
	if !ok {
		t.Fatalf("AUS missing, got %v", got)
	}
	if aus.Score != 8.0 {
		t.Fatalf("AUS score: want 8.0, got %v", aus.Score)
	}
	if math.Abs(aus.Change-0.2) > 1e-9 {
		t.Fatalf("AUS five-year change: want 0.2 (8.0 - 7.8), got %v", aus.Change)
	}
	if aus.Years != 10 || len(aus.History) != 10 {
		t.Fatalf("AUS history: want 10 points, got years=%d points=%d", aus.Years, len(aus.History))
	}
	if aus.History[0].Value != 7.5 || aus.History[9].Value != 8.0 {
		t.Fatalf("AUS history endpoints: %+v", aus.History)
	}

	// Hong Kong is present in EFW, which is why this source won over Heritage.
	if hkg, ok := got["HKG"]; !ok || hkg.Score != 8.53 {
		t.Fatalf("HKG: want 8.53, got %+v ok=%v", hkg, ok)
	}
	if _, ok := got["JPN"]; !ok {
		t.Fatal("JPN missing")
	}
	// Atlantis is not in the country table and must be skipped.
	if _, ok := got["ZZZ"]; ok {
		t.Fatal("unknown ISO should be skipped")
	}
}
