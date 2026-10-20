// Package ranking turns the valuation and macro data into one ranked list of
// countries, scoring each favourable condition as a cross-sectional percentile.
package ranking

import (
	"sort"
	"time"

	"github.com/shanehull/markethunt/internal/analytics"
	"github.com/shanehull/markethunt/internal/country"
	"github.com/shanehull/markethunt/internal/model"
)

// Weights for the composite. Valuation leads because a cheap price can outweigh
// a lot, but the macro factors still score. EV/EBIT is the larger valuation
// factor because it is capital-structure and tax neutral. Growth is a trailing
// average and a weak predictor of equity returns, so it carries less than
// savings: domestic capital funds investment without borrowing abroad, while
// past growth does not compound.
const (
	weightEVEBIT  = 0.30
	weightPB      = 0.20
	weightGrowth  = 0.10
	weightSavings = 0.15
	weightTrade   = 0.10
	weightFreedom = 0.05
	weightDebt    = 0.10
)

// MinFirms is the liquidity floor. A market needs at least this many listed
// firms in Damodaran's sample to be ranked, so a three-firm "market" cannot top
// the table on a noisy multiple.
const MinFirms = 30

// Build assembles the ranked snapshot. A country is ranked only when it clears
// the liquidity floor, has both valuation multiples, and has all five macro
// factors, so every rank is built from the same seven inputs.
func Build(vals map[string]model.Valuation, macros map[string]model.Macro, at time.Time) *model.Snapshot {
	var rows []model.Country
	for iso, info := range country.ByISO3 {
		v, ok := vals[iso]
		if !ok || v.Firms < MinFirms || v.PB <= 0 || v.EVEBIT <= 0 {
			continue
		}
		m := macros[iso]
		if m.Growth == nil || m.Savings == nil || m.Trade == nil || m.Debt == nil || m.Freedom == nil {
			continue
		}
		mm := m
		rows = append(rows, model.Country{
			ISO3:      iso,
			Name:      info.Name,
			Region:    info.Region,
			Valuation: &v,
			Macro:     &mm,
		})
	}

	pop := collect(rows)
	for i := range rows {
		rows[i].Scores = score(rows[i], pop)
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Scores.Composite != rows[j].Scores.Composite {
			return rows[i].Scores.Composite > rows[j].Scores.Composite
		}
		return rows[i].Name < rows[j].Name
	})
	for i := range rows {
		rows[i].Scores.Rank = i + 1
	}
	return &model.Snapshot{AsOf: at, Countries: rows}
}

// populations holds one slice per metric, each across every ranked country, so a
// percentile rank is always within the same universe.
type populations struct {
	pb, evebit             []float64
	growth                 []float64
	savings                []float64
	trade                  []float64
	freedom, freedomChange []float64
	debt                   []float64
}

func collect(rows []model.Country) populations {
	var p populations
	for _, c := range rows {
		if v := c.Valuation; v != nil {
			if v.PB > 0 {
				p.pb = append(p.pb, v.PB)
			}
			if v.EVEBIT > 0 {
				p.evebit = append(p.evebit, v.EVEBIT)
			}
		}
		if c.Macro == nil {
			continue
		}
		if g := c.Macro.Growth; g != nil {
			p.growth = append(p.growth, g.Average)
		}
		if s := c.Macro.Savings; s != nil {
			p.savings = append(p.savings, s.Rate)
		}
		if t := c.Macro.Trade; t != nil {
			p.trade = append(p.trade, t.Balance)
		}
		if f := c.Macro.Freedom; f != nil {
			p.freedom = append(p.freedom, f.Score)
			p.freedomChange = append(p.freedomChange, f.Change)
		}
		if d := c.Macro.Debt; d != nil {
			p.debt = append(p.debt, d.Ratio)
		}
	}
	return p
}

func score(c model.Country, p populations) model.Scores {
	var s model.Scores
	if v := c.Valuation; v != nil {
		if v.PB > 0 {
			s.PB = 1 - analytics.PercentileRank(p.pb, v.PB)
		}
		if v.EVEBIT > 0 {
			s.EVEBIT = 1 - analytics.PercentileRank(p.evebit, v.EVEBIT)
		}
	}
	if c.Macro != nil && c.Macro.Growth != nil {
		s.Growth = analytics.PercentileRank(p.growth, c.Macro.Growth.Average)
	}
	if c.Macro != nil && c.Macro.Savings != nil {
		s.Savings = analytics.PercentileRank(p.savings, c.Macro.Savings.Rate)
	}
	if c.Macro != nil && c.Macro.Trade != nil {
		s.Trade = analytics.PercentileRank(p.trade, c.Macro.Trade.Balance)
	}
	if c.Macro != nil && c.Macro.Freedom != nil {
		f := c.Macro.Freedom
		s.Freedom = analytics.Mean([]float64{
			analytics.PercentileRank(p.freedom, f.Score),
			analytics.PercentileRank(p.freedomChange, f.Change),
		})
	}
	if c.Macro != nil && c.Macro.Debt != nil {
		s.Debt = 1 - analytics.PercentileRank(p.debt, c.Macro.Debt.Ratio)
	}

	var sum, weight float64
	v := c.Valuation
	for _, factor := range []struct {
		score  float64
		weight float64
		ok     bool
	}{
		{s.EVEBIT, weightEVEBIT, v != nil && v.EVEBIT > 0},
		{s.PB, weightPB, v != nil && v.PB > 0},
		{s.Growth, weightGrowth, c.Macro != nil && c.Macro.Growth != nil},
		{s.Savings, weightSavings, c.Macro != nil && c.Macro.Savings != nil},
		{s.Trade, weightTrade, c.Macro != nil && c.Macro.Trade != nil},
		{s.Freedom, weightFreedom, c.Macro != nil && c.Macro.Freedom != nil},
		{s.Debt, weightDebt, c.Macro != nil && c.Macro.Debt != nil},
	} {
		if !factor.ok {
			continue
		}
		sum += factor.score * factor.weight
		weight += factor.weight
		s.Pillars++
	}
	if weight > 0 {
		s.Composite = sum / weight
	}
	return s
}
