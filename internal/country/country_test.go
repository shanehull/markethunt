package country

import "testing"

func TestReferenceTable(t *testing.T) {
	if len(All) < 100 {
		t.Fatalf("expected a broad table, got %d entries", len(All))
	}
	iso := map[string]bool{}
	name := map[string]bool{}
	for _, c := range All {
		if c.ISO3 == "" || c.Name == "" || c.Region == "" {
			t.Fatalf("incomplete entry: %+v", c)
		}
		if iso[c.ISO3] {
			t.Fatalf("duplicate ISO3 %s", c.ISO3)
		}
		if name[c.Name] {
			t.Fatalf("duplicate name %s", c.Name)
		}
		iso[c.ISO3] = true
		name[c.Name] = true
	}
}

func TestLookups(t *testing.T) {
	if got := ByISO3["AUS"].Name; got != "Australia" {
		t.Fatalf("ByISO3[AUS]: got %q", got)
	}
	if got := ByDamodaranName["South Korea"].ISO3; got != "KOR" {
		t.Fatalf("ByDamodaranName[South Korea]: got %q", got)
	}
	if _, ok := ByISO3["ZZZ"]; ok {
		t.Fatal("unexpected ISO3 ZZZ")
	}
	if len(Regions()) == 0 {
		t.Fatal("expected regions")
	}
}
