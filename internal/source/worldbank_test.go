package source

import "testing"

func ptr(v float64) *float64 { return &v }

func TestGroupByISO(t *testing.T) {
	obs := []wbObs{
		{ISO3: "AUS", Date: "2024", Value: ptr(1)},
		{ISO3: "AUS", Date: "2022", Value: ptr(3)},
		{ISO3: "ZZZ", Date: "2024", Value: ptr(9)}, // not a known country
		{ISO3: "JPN", Date: "2024", Value: nil},    // missing value
	}
	got := groupByISO(obs)
	if len(got) != 1 {
		t.Fatalf("want only AUS, got %v", got)
	}
	if len(got["AUS"]) != 2 {
		t.Fatalf("want 2 AUS points, got %d", len(got["AUS"]))
	}
	if got["AUS"][0].Date != "2022" {
		t.Fatalf("points should sort oldest first, got %s", got["AUS"][0].Date)
	}
}

func TestBuildGrowth(t *testing.T) {
	obs := []wbObs{
		{Date: "2021", Value: ptr(1)},
		{Date: "2022", Value: ptr(2)},
		{Date: "2023", Value: ptr(3)},
		{Date: "2024", Value: ptr(4)},
	}
	g := buildGrowth(obs)
	if g == nil {
		t.Fatal("expected growth")
	}
	if g.Latest != 4 || g.Average != 2.5 || g.Years != 4 {
		t.Fatalf("latest/average/years: got %v/%v/%d", g.Latest, g.Average, g.Years)
	}
	// Population standard deviation of 1,2,3,4 is sqrt(1.25).
	if diff := g.Volatility - 1.118033988749895; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("volatility: got %v", g.Volatility)
	}
	if len(g.History) != 4 {
		t.Fatalf("history: got %d", len(g.History))
	}
}

func TestWindow(t *testing.T) {
	obs := make([]wbObs, 12)
	for i := range obs {
		obs[i] = wbObs{Value: ptr(float64(i))}
	}
	if got := window(obs, 10); len(got) != 10 || *got[0].Value != 2 {
		t.Fatalf("window should keep the newest 10, got %d starting at %v", len(got), *got[0].Value)
	}
	if got := window(obs[:3], 10); len(got) != 3 {
		t.Fatalf("short slice should pass through, got %d", len(got))
	}
}

func TestParseWBObservations(t *testing.T) {
	body := []byte(`[{"page":1,"pages":1},[
		{"countryiso3code":"AUS","date":"2024","value":2.1},
		{"countryiso3code":"JPN","date":"2024","value":null}
	]]`)
	obs, err := parseWBObservations(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 2 || obs[0].ISO3 != "AUS" || obs[0].Value == nil || *obs[0].Value != 2.1 {
		t.Fatalf("unexpected observations: %+v", obs)
	}
	if obs[1].Value != nil {
		t.Fatal("null value should stay nil")
	}
	if _, err := parseWBObservations([]byte(`{"message":"bad"}`)); err == nil {
		t.Fatal("expected an error for a non-envelope response")
	}
}
