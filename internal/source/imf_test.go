package source

import (
	"math"
	"testing"
)

const imfFixture = `<?xml version='1.0' encoding='UTF-8'?>
<message:StructureSpecificData xmlns:message="http://www.sdmx.org/resources/sdmxml/schemas/v2_1/message">
  <message:DataSet>
    <Series COUNTRY="AUS" INDICATOR="GGXWDG_NGDP" FREQUENCY="A">
      <Obs TIME_PERIOD="2015" OBS_VALUE="30.0"/>
      <Obs TIME_PERIOD="2019" OBS_VALUE="40.0"/>
      <Obs TIME_PERIOD="2020" OBS_VALUE="45.0"/>
      <Obs TIME_PERIOD="2021" OBS_VALUE="46.0"/>
      <Obs TIME_PERIOD="2022" OBS_VALUE="50.0"/>
      <Obs TIME_PERIOD="2023" OBS_VALUE="51.0"/>
      <Obs TIME_PERIOD="2024" OBS_VALUE="52.0"/>
      <Obs TIME_PERIOD="2025" OBS_VALUE="53.0"/>
      <Obs TIME_PERIOD="2030" OBS_VALUE="60.0"/>
    </Series>
    <Series COUNTRY="ZZZ" INDICATOR="GGXWDG_NGDP" FREQUENCY="A">
      <Obs TIME_PERIOD="2024" OBS_VALUE="1.0"/>
    </Series>
  </message:DataSet>
</message:StructureSpecificData>`

func TestParseIMFDebt(t *testing.T) {
	got, err := parseIMFDebt([]byte(imfFixture), 2024)
	if err != nil {
		t.Fatal(err)
	}
	aus, ok := got["AUS"]
	if !ok {
		t.Fatalf("AUS missing, got %v", got)
	}
	if math.Abs(aus.Ratio-52) > 1e-9 || aus.Year != 2024 {
		t.Fatalf("AUS: want 52 at 2024, got %v at %d", aus.Ratio, aus.Year)
	}
	if len(aus.History) != 7 {
		t.Fatalf("history: want 7 points, got %d", len(aus.History))
	}
	// 2025 and 2030 are projections and must be excluded.
	for _, p := range aus.History {
		if p.Date.Year() > 2024 {
			t.Fatalf("projection leaked into history: %v", p.Date.Year())
		}
	}
	if _, ok := got["ZZZ"]; ok {
		t.Fatal("unknown ISO should be skipped")
	}
}
