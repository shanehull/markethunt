package source

import (
	"testing"

	"github.com/shanehull/markethunt/internal/model"
)

// TestMergeKeepsSeparateMacroFields guards the seam between the World Bank and
// Fraser: a later dataset must add fields, not replace the whole Macro.
func TestMergeKeepsSeparateMacroFields(t *testing.T) {
	var d Dataset
	d.merge(Dataset{Macros: map[string]model.Macro{
		"AUS": {Growth: &model.Growth{Average: 2}},
	}})
	d.merge(Dataset{Macros: map[string]model.Macro{
		"AUS": {Freedom: &model.Freedom{Score: 8}},
	}})
	d.merge(Dataset{Macros: map[string]model.Macro{
		"AUS": {Debt: &model.Debt{Ratio: 60}},
	}})

	m := d.Macros["AUS"]
	if m.Growth == nil || m.Growth.Average != 2 {
		t.Fatalf("growth lost: %+v", m.Growth)
	}
	if m.Freedom == nil || m.Freedom.Score != 8 {
		t.Fatalf("freedom lost: %+v", m.Freedom)
	}
	if m.Debt == nil || m.Debt.Ratio != 60 {
		t.Fatalf("debt lost: %+v", m.Debt)
	}
}
