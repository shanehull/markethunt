package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/shanehull/markethunt/internal/country"
	"github.com/shanehull/markethunt/internal/model"
)

// WorldBank fetches the macro series behind the growth, savings, and trade
// factors from the keyless World Bank API: GDP growth, gross savings, and the
// external balance on goods and services.
type WorldBank struct{}

func (WorldBank) Name() string { return "World Bank" }

const (
	wbGrowth  = "NY.GDP.MKTP.KD.ZG" // GDP growth, annual %
	wbSavings = "NY.GNS.ICTR.ZS"    // gross savings, % of GDP
	wbTrade   = "NE.RSB.GNFS.ZS"    // external balance on goods and services, % of GDP
)

// growthWindowYears is the averaging window behind the growth factor. Five
// years keeps the reading current: a ten-year average lags a turning point by
// roughly five years, so it reads a recovering market as its old decade.
const growthWindowYears = 5

func (WorldBank) Fetch(ctx context.Context, cfg Config) (Dataset, error) {
	year := time.Now().UTC().Year()
	span := fmt.Sprintf("%d:%d", year-10, year)

	specs := []struct {
		code  string
		query string
	}{
		{wbGrowth, "&date=" + span},
		{wbSavings, "&mrnev=1"},
		{wbTrade, "&mrnev=1"},
	}

	series := make([]map[string][]wbObs, len(specs))
	errs := make([]error, len(specs))
	var wg sync.WaitGroup
	for i, s := range specs {
		wg.Go(func() {
			obs, err := wbIndicator(ctx, cfg, s.code, s.query)
			if err != nil {
				errs[i] = err
				return
			}
			series[i] = groupByISO(obs)
		})
	}
	wg.Wait()

	growth, savings, trade := series[0], series[1], series[2]
	macros := map[string]model.Macro{}
	for iso := range country.ByISO3 {
		var m model.Macro
		if s := growth[iso]; len(s) > 0 {
			m.Growth = buildGrowth(s)
		}
		if v, ok := latest(savings[iso]); ok {
			m.Savings = &model.Savings{Rate: v}
		}
		if v, ok := latest(trade[iso]); ok {
			m.Trade = &model.Trade{Balance: v}
		}
		if m.Growth != nil || m.Savings != nil || m.Trade != nil {
			macros[iso] = m
		}
	}

	if len(macros) == 0 {
		return Dataset{}, errors.Join(append([]error{errors.New("world bank: no observations")}, errs...)...)
	}
	return Dataset{Macros: macros}, errors.Join(errs...)
}

// wbObs is one World Bank observation.
type wbObs struct {
	ISO3  string   `json:"countryiso3code"`
	Date  string   `json:"date"`
	Value *float64 `json:"value"`
}

func wbIndicator(ctx context.Context, cfg Config, code, query string) ([]wbObs, error) {
	base := "https://api.worldbank.org/v2/country/all/indicator/" + code
	u := base + "?format=json&per_page=30000" + query
	body, err := getCached(ctx, cfg, u, "application/json", cfg.ttl())
	if err != nil {
		return nil, err
	}
	obs, err := parseWBObservations(body)
	if err != nil {
		return nil, fmt.Errorf("world bank %s: %w", code, err)
	}
	return obs, nil
}

// parseWBObservations reads the [metadata, observations] envelope the World Bank
// v2 API returns.
func parseWBObservations(body []byte) ([]wbObs, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if len(raw) < 2 {
		return nil, errors.New("unexpected response")
	}
	var obs []wbObs
	if err := json.Unmarshal(raw[1], &obs); err != nil {
		return nil, fmt.Errorf("observations: %w", err)
	}
	return obs, nil
}

// groupByISO keeps only known countries, with dated points sorted oldest first.
func groupByISO(obs []wbObs) map[string][]wbObs {
	out := map[string][]wbObs{}
	for _, o := range obs {
		if o.Value == nil {
			continue
		}
		if _, ok := country.ByISO3[o.ISO3]; !ok {
			continue
		}
		out[o.ISO3] = append(out[o.ISO3], o)
	}
	for iso := range out {
		sort.Slice(out[iso], func(i, j int) bool { return out[iso][i].Date < out[iso][j].Date })
	}
	return out
}

func latest(obs []wbObs) (float64, bool) {
	if len(obs) == 0 || obs[len(obs)-1].Value == nil {
		return 0, false
	}
	return *obs[len(obs)-1].Value, true
}

// window returns the most recent n observations.
func window(obs []wbObs, n int) []wbObs {
	if len(obs) > n {
		return obs[len(obs)-n:]
	}
	return obs
}

func buildGrowth(obs []wbObs) *model.Growth {
	w := window(obs, growthWindowYears)
	if len(w) == 0 {
		return nil
	}
	vals := make([]float64, len(w))
	history := make([]model.Point, len(w))
	for i, o := range w {
		vals[i] = *o.Value
		history[i] = model.Point{Date: yearDate(o.Date), Value: *o.Value}
	}
	mean := meanOf(vals)
	return &model.Growth{
		Latest:     vals[len(vals)-1],
		Average:    mean,
		Volatility: stdDev(vals, mean),
		Years:      len(vals),
		History:    history,
	}
}

func yearDate(s string) time.Time {
	y, _ := strconv.Atoi(s)
	return time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC)
}

func meanOf(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	var sum float64
	for _, x := range v {
		sum += x
	}
	return sum / float64(len(v))
}

func stdDev(v []float64, mean float64) float64 {
	if len(v) == 0 {
		return 0
	}
	var sum float64
	for _, x := range v {
		d := x - mean
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(v)))
}
