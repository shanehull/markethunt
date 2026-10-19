package web

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// exportCSV writes the current view (filters and sort included) as one row per
// market. Text fields are guarded against spreadsheet formula injection.
func (s *Server) exportCSV(w http.ResponseWriter, r *http.Request) {
	v := s.buildView(r)
	if v.Snapshot == nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition",
		`attachment; filename="markethunt-`+time.Now().UTC().Format("2006-01-02")+`.csv"`)
	w.Header().Set("Cache-Control", "no-store")

	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"rank", "country", "iso3", "region", "score",
		"p_b", "ev_ebit", "p_e", "dividend_yield_pct",
		"gdp_growth_5y", "growth_volatility", "savings_pct", "trade_pct",
		"debt_to_gdp_pct", "efw_score", "efw_change",
	})
	for _, c := range v.Rows {
		var pb, ev, pe, div, gdp, vol, sav, trade, debt, efw, change float64
		hasVal := c.Valuation != nil
		if hasVal {
			pb, ev, pe, div = c.Valuation.PB, c.Valuation.EVEBIT, c.Valuation.PE, c.Valuation.DivYield*100
		}
		hasGrowth := c.Macro != nil && c.Macro.Growth != nil
		if hasGrowth {
			gdp, vol = c.Macro.Growth.Average, c.Macro.Growth.Volatility
		}
		hasSavings := c.Macro != nil && c.Macro.Savings != nil
		if hasSavings {
			sav = c.Macro.Savings.Rate
		}
		hasTrade := c.Macro != nil && c.Macro.Trade != nil
		if hasTrade {
			trade = c.Macro.Trade.Balance
		}
		hasDebt := c.Macro != nil && c.Macro.Debt != nil
		if hasDebt {
			debt = c.Macro.Debt.Ratio
		}
		hasFreedom := c.Macro != nil && c.Macro.Freedom != nil
		if hasFreedom {
			efw, change = c.Macro.Freedom.Score, c.Macro.Freedom.Change
		}

		_ = cw.Write([]string{
			strconv.Itoa(c.Scores.Rank),
			csvSafe(c.Name),
			c.ISO3,
			csvSafe(c.Region),
			strconv.Itoa(int(c.Scores.Composite*100 + 0.5)),
			csvNum(pb, hasVal, 2, false),
			csvNum(ev, hasVal, 1, false),
			csvNum(pe, hasVal, 1, false),
			csvNum(div, hasVal, 1, false),
			csvNum(gdp, hasGrowth, 1, true),
			csvNum(vol, hasGrowth, 1, false),
			csvNum(sav, hasSavings, 1, false),
			csvNum(trade, hasTrade, 1, true),
			csvNum(debt, hasDebt, 1, false),
			csvNum(efw, hasFreedom, 1, false),
			csvNum(change, hasFreedom, 1, true),
		})
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		s.opts.Logger.ErrorContext(r.Context(), "csv write failed", slog.Any("error", err))
	}
}

func csvNum(v float64, ok bool, decimals int, signed bool) string {
	if !ok {
		return ""
	}
	if signed {
		return fmt.Sprintf("%+.*f", decimals, v)
	}
	return strconv.FormatFloat(v, 'f', decimals, 64)
}

// csvSafe prefixes spreadsheet formula triggers, so a text cell cannot execute
// when the export is opened in Excel or Sheets.
func csvSafe(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}
