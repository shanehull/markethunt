package web

import "github.com/shanehull/markethunt/internal/model"

// The accessors below format one metric per country, rendering an em dash when
// the source did not cover it, so templates never touch a nil pointer.

func priceBook(c model.Country) string {
	if c.Valuation == nil {
		return "—"
	}
	return num(c.Valuation.PB, 2)
}

func evEbit(c model.Country) string {
	if c.Valuation == nil {
		return "—"
	}
	return num(c.Valuation.EVEBIT, 1)
}

func priceEarnings(c model.Country) string {
	if c.Valuation == nil {
		return "—"
	}
	return num(c.Valuation.PE, 1)
}

func dividend(c model.Country) string {
	if c.Valuation == nil || c.Valuation.DivYield == 0 {
		return "—"
	}
	return signed(c.Valuation.DivYield*100, 1)
}

func gdp(c model.Country) string {
	if c.Macro == nil || c.Macro.Growth == nil {
		return "—"
	}
	return signed(c.Macro.Growth.Average, 1)
}

func gdpVol(c model.Country) string {
	if c.Macro == nil || c.Macro.Growth == nil {
		return "—"
	}
	return num(c.Macro.Growth.Volatility, 1)
}

func growthYears(c model.Country) string {
	if c.Macro == nil || c.Macro.Growth == nil {
		return "—"
	}
	return num(float64(c.Macro.Growth.Years), 0)
}

func savingsRate(c model.Country) string {
	if c.Macro == nil || c.Macro.Savings == nil {
		return "—"
	}
	return num(c.Macro.Savings.Rate, 1)
}

func tradeBalance(c model.Country) string {
	if c.Macro == nil || c.Macro.Trade == nil {
		return "—"
	}
	return signed(c.Macro.Trade.Balance, 1)
}

func freedomScore(c model.Country) string {
	if c.Macro == nil || c.Macro.Freedom == nil {
		return "—"
	}
	return num(c.Macro.Freedom.Score, 1)
}

func freedomChange(c model.Country) string {
	if c.Macro == nil || c.Macro.Freedom == nil {
		return "—"
	}
	return signed(c.Macro.Freedom.Change, 1)
}

func freedomYears(c model.Country) string {
	if c.Macro == nil || c.Macro.Freedom == nil {
		return "—"
	}
	return num(float64(c.Macro.Freedom.Years), 0)
}

func debt(c model.Country) string {
	if c.Macro == nil || c.Macro.Debt == nil {
		return "—"
	}
	return num(c.Macro.Debt.Ratio, 1)
}

func debtYear(c model.Country) string {
	if c.Macro == nil || c.Macro.Debt == nil {
		return "—"
	}
	return num(float64(c.Macro.Debt.Year), 0)
}

func firms(c model.Country) string {
	if c.Valuation == nil {
		return "—"
	}
	return num(float64(c.Valuation.Firms), 0)
}
