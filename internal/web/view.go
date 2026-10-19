package web

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/shanehull/markethunt/internal/buildinfo"
	"github.com/shanehull/markethunt/internal/model"
)

// view is the template data for the index and table fragments.
type view struct {
	Snapshot *model.Snapshot
	Rows     []model.Country
	Regions  []string

	Region string
	Query  string
	Sort   string
	Dir    string

	Total       int
	Matched     int
	AsOf        string
	ValuationAt string
	Version     string
	CSS         template.CSS
	Scripts     []CDNScript
	ScriptNonce string
	StyleNonce  string
}

// detailView is the template data for the country drawer.
type detailView struct {
	Country   model.Country
	Total     int
	Charts    []chart
	ChartJSON string
}

// chart is one history line rendered by Chart.js in the drawer.
type chart struct {
	ID     string        `json:"id"`
	Label  string        `json:"label"`
	Unit   string        `json:"unit"`
	Points []model.Point `json:"points"`
}

// sortKeys are the columns offered for sorting, with their default direction.
var sortKeys = map[string]string{
	"composite": "desc",
	"pb":        "asc",
	"evebit":    "asc",
	"gdp":       "desc",
	"savings":   "desc",
	"trade":     "desc",
	"freedom":   "desc",
	"debt":      "asc",
	"name":      "asc",
}

func (s *Server) buildView(r *http.Request) view {
	q := r.URL.Query()
	v := view{
		Snapshot: s.store.Snapshot(),
		Region:   q.Get("region"),
		Query:    strings.TrimSpace(q.Get("q")),
		Sort:     q.Get("sort"),
		Dir:      q.Get("dir"),
		Version:  buildinfo.GitTag,
		CSS:      s.css,
		Scripts:  CDNScripts,
	}
	if v.Version == "" {
		v.Version = "dev"
	}
	if _, ok := sortKeys[v.Sort]; !ok {
		v.Sort = "composite"
	}
	if v.Dir != "asc" && v.Dir != "desc" {
		v.Dir = sortKeys[v.Sort]
	}
	n, _ := r.Context().Value(nonceKey{}).(nonces)
	v.ScriptNonce, v.StyleNonce = n.Script, n.Style

	if v.Snapshot == nil {
		return v
	}
	v.Total = len(v.Snapshot.Countries)
	v.AsOf = v.Snapshot.AsOf.Format("2 Jan 2006")
	if !v.Snapshot.ValuationAt.IsZero() {
		v.ValuationAt = v.Snapshot.ValuationAt.Format("Jan 2006")
	}

	query := strings.ToLower(v.Query)
	for _, c := range v.Snapshot.Countries {
		if v.Region != "" && c.Region != v.Region {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(c.Name), query) {
			continue
		}
		v.Rows = append(v.Rows, c)
	}
	v.Matched = len(v.Rows)
	v.Regions = regions(v.Snapshot.Countries)
	sortRows(v.Rows, v.Sort, v.Dir)
	return v
}

func regions(countries []model.Country) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range countries {
		if c.Region != "" && !seen[c.Region] {
			seen[c.Region] = true
			out = append(out, c.Region)
		}
	}
	sort.Strings(out)
	return out
}

// sortRows sorts in place. Missing metrics sort last either way; ties break on
// name.
func sortRows(rows []model.Country, key, dir string) {
	desc := dir == "desc"
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if key == "name" {
			if desc {
				return a.Name > b.Name
			}
			return a.Name < b.Name
		}
		x, okx := metric(a, key)
		y, oky := metric(b, key)
		if okx != oky {
			return okx
		}
		if okx && x != y {
			if desc {
				return x > y
			}
			return x < y
		}
		return a.Name < b.Name
	})
}

func metric(c model.Country, key string) (float64, bool) {
	if key == "composite" {
		return c.Scores.Composite, true
	}
	if v := c.Valuation; v != nil {
		switch key {
		case "pb":
			return v.PB, v.PB > 0
		case "evebit":
			return v.EVEBIT, v.EVEBIT > 0
		}
	}
	if m := c.Macro; m != nil {
		switch key {
		case "gdp":
			if m.Growth != nil {
				return m.Growth.Average, true
			}
		case "savings":
			if m.Savings != nil {
				return m.Savings.Rate, true
			}
		case "trade":
			if m.Trade != nil {
				return m.Trade.Balance, true
			}
		case "freedom":
			if m.Freedom != nil {
				return m.Freedom.Score, true
			}
		case "debt":
			if m.Debt != nil {
				return m.Debt.Ratio, true
			}
		}
	}
	return 0, false
}

// num formats a value with the given decimals, rendering zero as an em dash so
// absent data never reads as a real zero.
func num(v float64, decimals int) string {
	if v == 0 {
		return "—"
	}
	return strconv.FormatFloat(v, 'f', decimals, 64)
}

func signed(v float64, decimals int) string {
	return fmt.Sprintf("%+.*f", decimals, v)
}

func pct(v float64) string {
	return strconv.Itoa(int(v*100 + 0.5))
}

func scoreClass(v float64) string {
	switch {
	case v >= 0.66:
		return "good"
	case v >= 0.33:
		return "mid"
	default:
		return "poor"
	}
}

// sortURL links to the table sorted by key, toggling direction when the key is
// already active and preserving the active filters.
func sortURL(current view, key string) string {
	dir := sortKeys[key]
	if current.Sort == key {
		if current.Dir == "asc" {
			dir = "desc"
		} else {
			dir = "asc"
		}
	}
	return queryURL(current, map[string]string{"sort": key, "dir": dir})
}

func sortMark(current view, key string) string {
	if current.Sort != key {
		return ""
	}
	if current.Dir == "asc" {
		return "▲"
	}
	return "▼"
}

// queryURL keeps the current filters and applies the overrides; an empty
// override drops that filter.
func queryURL(current view, overrides map[string]string) string {
	values := filterValues(current)
	for k, v := range overrides {
		if v == "" {
			values.Del(k)
		} else {
			values.Set(k, v)
		}
	}
	return "/?" + values.Encode()
}

// exportURL is the CSV equivalent of the current table view.
func exportURL(current view) string {
	return "/export.csv?" + filterValues(current).Encode()
}

func filterValues(current view) url.Values {
	values := url.Values{"sort": {current.Sort}, "dir": {current.Dir}}
	if current.Region != "" {
		values.Set("region", current.Region)
	}
	if current.Query != "" {
		values.Set("q", current.Query)
	}
	return values
}
