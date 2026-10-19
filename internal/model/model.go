// Package model defines the core data types shared across the packages.
package model

import "time"

// Point is a single dated observation.
type Point struct {
	Date  time.Time `json:"date"`
	Value float64   `json:"value"`
}

// Valuation holds Damodaran's country equity multiples for the latest snapshot.
type Valuation struct {
	PB        float64 `json:"pb"`        // median price-to-book
	PE        float64 `json:"pe"`        // median trailing price-to-earnings
	EVEBIT    float64 `json:"evEbit"`    // median EV/EBIT
	DivYield  float64 `json:"divYield"`  // aggregate dividend yield, %
	Firms     int     `json:"firms"`     // listed firms behind the aggregate
	MarketCap float64 `json:"marketCap"` // aggregate market cap, US$ millions
}

// Growth holds the GDP growth profile.
type Growth struct {
	Latest     float64 `json:"latest"`     // latest annual GDP growth, %
	Average    float64 `json:"average"`    // mean annual growth over the window, %
	Volatility float64 `json:"volatility"` // population standard deviation of annual growth, pp
	Years      int     `json:"years"`
	History    []Point `json:"history"`
}

// Savings holds gross national savings.
type Savings struct {
	Rate float64 `json:"rate"` // % of GDP, latest
}

// Trade holds the external balance on goods and services.
type Trade struct {
	Balance float64 `json:"balance"` // % of GDP, latest
}

// Freedom holds the Fraser Institute's Economic Freedom of the World overall
// rating, scored 0 to 10, plus its five-year change.
type Freedom struct {
	Score   float64 `json:"score"`
	Change  float64 `json:"change"`
	Years   int     `json:"years"`
	History []Point `json:"history"`
}

// Debt holds general government gross debt from the IMF WEO.
type Debt struct {
	Ratio   float64 `json:"ratio"` // % of GDP
	Year    int     `json:"year"`
	History []Point `json:"history"`
}

// Macro bundles the macro and governance metrics for one country.
type Macro struct {
	Growth  *Growth  `json:"growth,omitempty"`
	Savings *Savings `json:"savings,omitempty"`
	Trade   *Trade   `json:"trade,omitempty"`
	Freedom *Freedom `json:"freedom,omitempty"`
	Debt    *Debt    `json:"debt,omitempty"`
}

// Scores holds the factor scores and the composite. Factor scores are
// cross-sectional percentile ranks in 0..1; 1 is the most favourable.
type Scores struct {
	EVEBIT    float64 `json:"evEbit"`
	PB        float64 `json:"pb"`
	Growth    float64 `json:"growth"`
	Savings   float64 `json:"savings"`
	Trade     float64 `json:"trade"`
	Freedom   float64 `json:"freedom"`
	Debt      float64 `json:"debt"`
	Composite float64 `json:"composite"`
	Rank      int     `json:"rank"`
	Pillars   int     `json:"pillars"` // factors with data behind the composite
}

// Country is one ranked row: a market with its valuation and macro metrics.
// Any pillar may be nil when no source covered it.
type Country struct {
	ISO3      string     `json:"iso3"`
	Name      string     `json:"name"`
	Region    string     `json:"region"`
	Valuation *Valuation `json:"valuation,omitempty"`
	Macro     *Macro     `json:"macro,omitempty"`
	Scores    Scores     `json:"scores"`
}

// Snapshot is the assembled dataset the web layer renders.
type Snapshot struct {
	AsOf        time.Time `json:"asOf"`
	ValuationAt time.Time `json:"valuationAt"`
	Countries   []Country `json:"countries"`
}
