package source

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/shanehull/markethunt/internal/country"
	"github.com/shanehull/markethunt/internal/model"
)

// IMF fetches general government gross debt as a share of GDP from the official
// IMF SDMX API (World Economic Outlook). The WEO carries a couple of projection
// years, so only completed years up to the target are kept.
type IMF struct{}

func (IMF) Name() string { return "IMF" }

const imfDebtURL = "https://api.imf.org/external/sdmx/2.1/data/IMF.RES,WEO/.GGXWDG_NGDP.A"

// debtHistoryYears is how much history the chart receives.
const debtHistoryYears = 10

func (IMF) Fetch(ctx context.Context, cfg Config) (Dataset, error) {
	body, err := getCached(ctx, cfg, imfDebtURL, "application/xml", cfg.ttl())
	if err != nil {
		return Dataset{}, fmt.Errorf("imf: %w", err)
	}
	debts, err := parseIMFDebt(body, time.Now().UTC().Year()-2)
	if err != nil {
		return Dataset{}, fmt.Errorf("imf: %w", err)
	}
	macros := make(map[string]model.Macro, len(debts))
	for iso, d := range debts {
		debt := d
		macros[iso] = model.Macro{Debt: &debt}
	}
	return Dataset{Macros: macros}, nil
}

// parseIMFDebt reads the SDMX structure-specific response and keeps the latest
// completed year at or before target.
func parseIMFDebt(body []byte, target int) (map[string]model.Debt, error) {
	var msg struct {
		DataSet struct {
			Series []struct {
				Country string `xml:"COUNTRY,attr"`
				Obs     []struct {
					Period string `xml:"TIME_PERIOD,attr"`
					Value  string `xml:"OBS_VALUE,attr"`
				} `xml:"Obs"`
			} `xml:"Series"`
		} `xml:"DataSet"`
	}
	if err := xml.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if len(msg.DataSet.Series) == 0 {
		return nil, errors.New("no series")
	}

	out := make(map[string]model.Debt)
	for _, s := range msg.DataSet.Series {
		if _, ok := country.ByISO3[s.Country]; !ok {
			continue
		}
		byYear := map[int]float64{}
		for _, o := range s.Obs {
			year, err := strconv.Atoi(o.Period)
			if err != nil || year > target {
				continue
			}
			v, ok := parseNumber(o.Value)
			if !ok {
				continue
			}
			byYear[year] = v
		}
		if len(byYear) == 0 {
			continue
		}
		years := make([]int, 0, len(byYear))
		for y := range byYear {
			years = append(years, y)
		}
		sort.Ints(years)
		latest := years[len(years)-1]

		start := 0
		if len(years) > debtHistoryYears {
			start = len(years) - debtHistoryYears
		}
		history := make([]model.Point, 0, debtHistoryYears)
		for _, y := range years[start:] {
			history = append(history, model.Point{
				Date:  time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC),
				Value: byYear[y],
			})
		}
		out[s.Country] = model.Debt{Ratio: byYear[latest], Year: latest, History: history}
	}
	return out, nil
}
