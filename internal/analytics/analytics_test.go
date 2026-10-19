package analytics

import (
	"math"
	"testing"
)

func closeTo(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestPercentileRank(t *testing.T) {
	pop := []float64{1, 2, 3, 4, 5}
	if got := PercentileRank(pop, 3); !closeTo(got, 0.5) {
		t.Fatalf("want 0.5, got %v", got)
	}
	if got := PercentileRank(pop, 1); !closeTo(got, 0.1) {
		t.Fatalf("want 0.1, got %v", got)
	}
	if got := PercentileRank(pop, 99); !closeTo(got, 1.0) {
		t.Fatalf("want 1.0, got %v", got)
	}
	if got := PercentileRank(nil, 1); !closeTo(got, 0.5) {
		t.Fatalf("empty population should be neutral, got %v", got)
	}
}

func TestPercentileRankTies(t *testing.T) {
	// Two of four values are equal to the probe: rank is the mid-point.
	if got := PercentileRank([]float64{1, 2, 2, 4}, 2); !closeTo(got, 0.5) {
		t.Fatalf("want 0.5, got %v", got)
	}
}

func TestMean(t *testing.T) {
	if got := Mean([]float64{2, 4, 6}); !closeTo(got, 4) {
		t.Fatalf("want 4, got %v", got)
	}
	if got := Mean(nil); got != 0 {
		t.Fatalf("want 0, got %v", got)
	}
}
