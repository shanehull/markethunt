package source

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shanehull/markethunt/internal/country"
	"github.com/shanehull/markethunt/internal/model"
	"github.com/shanehull/markethunt/internal/xls"
)

// Damodaran fetches Aswath Damodaran's country equity multiples for the latest
// snapshot.
type Damodaran struct{}

func (Damodaran) Name() string { return "Damodaran" }

const damodaranCurrent = "https://pages.stern.nyu.edu/~adamodar/pc/datasets/countrystats.xls"

func (d Damodaran) Fetch(ctx context.Context, cfg Config) (Dataset, error) {
	body, err := getCached(ctx, cfg, damodaranCurrent, "", cfg.ttl())
	if err != nil {
		return Dataset{}, fmt.Errorf("damodaran: %w", err)
	}
	vals, at, err := parseCountryStats(body)
	if err != nil {
		return Dataset{}, fmt.Errorf("damodaran: %w", err)
	}
	return Dataset{Valuations: vals, ValuationAt: at}, nil
}

// parseCountryStats reads the Sheet1 data grid from one country-stats workbook,
// returning the per-country multiples and the file's as-of date.
func parseCountryStats(body []byte) (map[string]model.Valuation, time.Time, error) {
	sheets, err := xls.Read(body)
	if err != nil {
		return nil, time.Time{}, err
	}
	grid := dataSheet(sheets)
	if len(grid) == 0 {
		return nil, time.Time{}, errors.New("no data sheet")
	}
	at := updatedAt(grid)
	headerRow := -1
	for i, row := range grid {
		if len(row) > 0 && strings.TrimSpace(row[0]) == "Country" {
			headerRow = i
			break
		}
	}
	if headerRow < 0 {
		return nil, time.Time{}, errors.New("header row not found")
	}
	header := grid[headerRow]
	pbCol := pickPB(header)
	evCol := pickEVEBIT(header)
	peCol := pickPE(header)
	divCol := pickColumn(header, func(s string) bool { return strings.Contains(s, "dividend yield") })
	if pbCol < 0 && evCol < 0 {
		return nil, time.Time{}, errors.New("no P/B or EV/EBIT column")
	}

	out := make(map[string]model.Valuation)
	for _, row := range grid[headerRow+1:] {
		if len(row) == 0 || strings.TrimSpace(row[0]) == "" {
			continue
		}
		info, ok := country.ByDamodaranName[strings.TrimSpace(row[0])]
		if !ok {
			continue
		}
		v := model.Valuation{}
		if pbCol >= 0 {
			v.PB, _ = cell(row, pbCol)
		}
		if evCol >= 0 {
			v.EVEBIT, _ = cell(row, evCol)
		}
		if peCol >= 0 {
			v.PE, _ = cell(row, peCol)
		}
		if divCol >= 0 {
			v.DivYield, _ = cell(row, divCol)
		}
		if f, ok := cell(row, 1); ok {
			v.Firms = int(f)
		}
		if mc, ok := cell(row, 2); ok {
			v.MarketCap = mc
		}
		out[info.ISO3] = v
	}
	return out, at, nil
}

// updatedAt reads the "Date updated:" Excel serial from the top-left cells.
func updatedAt(grid [][]string) time.Time {
	if len(grid) == 0 || len(grid[0]) < 2 {
		return time.Time{}
	}
	serial, ok := parseNumber(grid[0][1])
	if !ok || serial <= 0 {
		return time.Time{}
	}
	return time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(serial))
}

// dataSheet returns the worksheet that holds the country table.
func dataSheet(sheets map[string][][]string) [][]string {
	if s, ok := sheets["Sheet1"]; ok && len(s) > 0 {
		return s
	}
	var best [][]string
	for _, rows := range sheets {
		if len(rows) > len(best) {
			best = rows
		}
	}
	return best
}

func cell(row []string, col int) (float64, bool) {
	if col < 0 || col >= len(row) {
		return 0, false
	}
	return parseNumber(row[col])
}

// pickPB chooses the price-to-book column, preferring the median over an
// average and avoiding the aggregate ratio.
func pickPB(header []string) int {
	for _, want := range []func(string) bool{
		func(s string) bool { return strings.Contains(s, "median") && strings.Contains(s, "pbv") },
		func(s string) bool { return strings.Contains(s, "average") && strings.Contains(s, "pbv") },
		func(s string) bool { return strings.Contains(s, "pbv") && !strings.Contains(s, "aggregate") },
		func(s string) bool { return strings.Contains(s, "pbv") },
	} {
		if c := pickColumn(header, want); c >= 0 {
			return c
		}
	}
	return -1
}

// pickEVEBIT chooses the EV/EBIT column, preferring the median. It must never
// match EV/EBITDA, which shares the "EV/EBIT" prefix.
func pickEVEBIT(header []string) int {
	for _, want := range []func(string) bool{
		func(s string) bool { return strings.Contains(s, "median") && isEVEBIT(s) },
		func(s string) bool { return strings.Contains(s, "average") && isEVEBIT(s) },
		func(s string) bool { return isEVEBIT(s) && !strings.Contains(s, "aggregate") },
		func(s string) bool { return isEVEBIT(s) },
	} {
		if c := pickColumn(header, want); c >= 0 {
			return c
		}
	}
	return -1
}

func isEVEBIT(s string) bool {
	return strings.Contains(s, "ev/ebit") && !strings.Contains(s, "ebitda")
}

// pickPE chooses the trailing price-to-earnings column, preferring the median
// and avoiding PEG and forward P/E.
func pickPE(header []string) int {
	for _, want := range []func(string) bool{
		func(s string) bool { return strings.Contains(s, "trailing") && strings.Contains(s, "median") },
		func(s string) bool { return strings.Contains(s, "trailing") },
		func(s string) bool {
			return strings.Contains(s, "median") && strings.Contains(s, "pe") &&
				!strings.Contains(s, "peg") && !strings.Contains(s, "forward")
		},
		func(s string) bool {
			return strings.Contains(s, "pe") && !strings.Contains(s, "peg") &&
				!strings.Contains(s, "forward") && !strings.Contains(s, "aggregate")
		},
	} {
		if c := pickColumn(header, want); c >= 0 {
			return c
		}
	}
	return -1
}

func pickColumn(header []string, want func(string) bool) int {
	for i, h := range header {
		if want(strings.ToLower(strings.TrimSpace(h))) {
			return i
		}
	}
	return -1
}
