// Package analytics provides the pure computations behind the ranking: cross-
// sectional percentile ranks and simple summary statistics.
package analytics

// PercentileRank returns the share of the population below v, splitting ties
// evenly, in 0..1. An empty population ranks at 0.5 so a lone country is
// neutral rather than best or worst.
func PercentileRank(pop []float64, v float64) float64 {
	if len(pop) == 0 {
		return 0.5
	}
	var less, equal int
	for _, x := range pop {
		switch {
		case x < v:
			less++
		case x == v:
			equal++
		}
	}
	return (float64(less) + 0.5*float64(equal)) / float64(len(pop))
}

// Mean returns the arithmetic mean, or zero for an empty slice.
func Mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	var sum float64
	for _, x := range v {
		sum += x
	}
	return sum / float64(len(v))
}
