package source

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shanehull/markethunt/internal/country"
	"github.com/shanehull/markethunt/internal/model"
	"github.com/shanehull/markethunt/internal/xlsx"
)

// Fraser fetches the Fraser Institute's Economic Freedom of the World master
// data. It is ISO-coded, includes Hong Kong and Taiwan, and carries the full
// history, so the overall rating and a five-year change come from one file.
type Fraser struct{}

func (Fraser) Name() string { return "Fraser" }

const fraserURL = "https://efotw.org/sites/all/modules/custom/ftw_maps_pages/files/efotw-%d-master-index-data-for-researchers-iso.xlsx"

// fraserChangeYears is how far back the improvement is measured, and
// fraserHistoryYears is how much history the chart receives.
const (
	fraserChangeYears  = 5
	fraserHistoryYears = 10
)

func (f Fraser) Fetch(ctx context.Context, cfg Config) (Dataset, error) {
	year := time.Now().UTC().Year()
	var body []byte
	var errs []error
	for y := year; y > year-4; y-- {
		b, err := getCached(ctx, cfg, fmt.Sprintf(fraserURL, y), "", cfg.ttl())
		if err != nil {
			errs = append(errs, err)
			continue
		}
		body = b
		break
	}
	if body == nil {
		return Dataset{}, errors.Join(append([]error{errors.New("fraser: no workbook found")}, errs...)...)
	}

	freedoms, err := parseFraser(body)
	if err != nil {
		return Dataset{}, fmt.Errorf("fraser: %w", err)
	}
	macros := make(map[string]model.Macro, len(freedoms))
	for iso, f := range freedoms {
		freedom := f
		macros[iso] = model.Macro{Freedom: &freedom}
	}
	return Dataset{Macros: macros}, nil
}

// parseFraser reads the EFW panel sheet into one Freedom per known country.
func parseFraser(body []byte) (map[string]model.Freedom, error) {
	rows, err := fraserRows(body)
	if err != nil {
		return nil, err
	}
	headerRow := -1
	for i, row := range rows {
		if headerIndex(row, func(s string) bool { return s == "iso_code" }) >= 0 {
			headerRow = i
			break
		}
	}
	if headerRow < 0 {
		return nil, errors.New("EFW header not found")
	}
	header := rows[headerRow]
	yearCol := headerIndex(header, func(s string) bool { return s == "year" })
	isoCol := headerIndex(header, func(s string) bool { return s == "iso_code" })
	scoreCol := headerIndex(header, func(s string) bool { return strings.Contains(s, "economic freedom all areas") })
	if yearCol < 0 || isoCol < 0 || scoreCol < 0 {
		return nil, errors.New("EFW columns not found")
	}

	history := map[string]map[int]float64{}
	for _, row := range rows[headerRow+1:] {
		if len(row) <= yearCol || len(row) <= isoCol || len(row) <= scoreCol {
			continue
		}
		iso := strings.TrimSpace(row[isoCol])
		if _, ok := country.ByISO3[iso]; !ok {
			continue
		}
		year, err := strconv.Atoi(strings.TrimSpace(row[yearCol]))
		if err != nil {
			continue
		}
		score, ok := parseNumber(row[scoreCol])
		if !ok {
			continue
		}
		if history[iso] == nil {
			history[iso] = map[int]float64{}
		}
		history[iso][year] = score
	}

	out := make(map[string]model.Freedom, len(history))
	for iso, byYear := range history {
		years := make([]int, 0, len(byYear))
		for y := range byYear {
			years = append(years, y)
		}
		sort.Ints(years)
		latest := years[len(years)-1]
		f := model.Freedom{
			Score:   byYear[latest],
			Years:   len(years),
			History: make([]model.Point, 0, fraserHistoryYears),
		}
		if prior, ok := byYear[latest-fraserChangeYears]; ok {
			f.Change = byYear[latest] - prior
		}
		start := 0
		if len(years) > fraserHistoryYears {
			start = len(years) - fraserHistoryYears
		}
		for _, y := range years[start:] {
			f.History = append(f.History, model.Point{
				Date:  time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC),
				Value: byYear[y],
			})
		}
		f.Years = len(f.History)
		out[iso] = f
	}
	return out, nil
}

// fraserRows returns the panel sheet, falling back to a scan if the sheet is
// renamed between editions.
func fraserRows(body []byte) ([][]string, error) {
	rows, err := xlsx.ReadSheet(body, "EFW Panel Dataset")
	if err == nil {
		return rows, nil
	}
	sheets, readErr := xlsx.Read(body)
	if readErr != nil {
		return nil, readErr
	}
	for _, candidate := range sheets {
		for _, row := range candidate {
			if headerIndex(row, func(s string) bool { return s == "iso_code" }) >= 0 {
				return candidate, nil
			}
		}
	}
	return nil, errors.New("EFW panel sheet not found")
}

func headerIndex(row []string, want func(string) bool) int {
	for i, cell := range row {
		if want(strings.ToLower(strings.TrimSpace(cell))) {
			return i
		}
	}
	return -1
}
