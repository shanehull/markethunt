package source

import "testing"

func TestPickPB(t *testing.T) {
	cases := []struct {
		header []string
		want   int
	}{
		{[]string{"Country", "count", "median(Trailing PE)", "median(PBV)"}, 3},
		{[]string{"Country", "Average of PBV", "Aggregate PBV"}, 1},
		{[]string{"Country", "Aggregate PBV"}, 1},
		{[]string{"Country", "count"}, -1},
	}
	for _, c := range cases {
		if got := pickPB(c.header); got != c.want {
			t.Fatalf("pickPB(%v) = %d, want %d", c.header, got, c.want)
		}
	}
}

func TestPickEVEBIT(t *testing.T) {
	cases := []struct {
		header []string
		want   int
	}{
		{[]string{"Country", "median(EV/EBIT)", "median(EV/EBITDA)"}, 1},
		// EV/EBITDA shares the "EV/EBIT" prefix and must never be chosen.
		{[]string{"Country", "median(EV/EBITDA)", "median(EV/EBIT)"}, 2},
		{[]string{"Country", "Aggregate EV/EBIT"}, 1},
		{[]string{"Country", "count"}, -1},
	}
	for _, c := range cases {
		if got := pickEVEBIT(c.header); got != c.want {
			t.Fatalf("pickEVEBIT(%v) = %d, want %d", c.header, got, c.want)
		}
	}
}

func TestPickPE(t *testing.T) {
	cases := []struct {
		header []string
		want   int
	}{
		{[]string{"Country", "median(Current PE)", "median(Trailing PE)", "median(Forward PE)", "median(PEG)"}, 2},
		{[]string{"Country", "Average of Trailing PE", "Average of Forward PE"}, 1},
		{[]string{"Country", "Median Trailing PE"}, 1},
		// The trailing column wins even when PEG and forward columns appear first.
		{[]string{"Country", "median(PEG)", "median(Forward PE)", "median(Trailing PE)"}, 3},
	}
	for _, c := range cases {
		if got := pickPE(c.header); got != c.want {
			t.Fatalf("pickPE(%v) = %d, want %d", c.header, got, c.want)
		}
	}
}

func TestParseNumber(t *testing.T) {
	for _, s := range []string{"", "NA", "-", "n/a"} {
		if _, ok := parseNumber(s); ok {
			t.Fatalf("parseNumber(%q) should be missing", s)
		}
	}
	if v, ok := parseNumber("1,234.5"); !ok || v != 1234.5 {
		t.Fatalf("want 1234.5, got %v ok=%v", v, ok)
	}
}
