package xls

import (
	"os"
	"strings"
	"testing"
)

// TestReadFixture parses a small BIFF8 workbook written by a third-party
// library, covering the shared string table, number, and blank-cell paths.
func TestReadFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/sample.xls")
	if err != nil {
		t.Fatal(err)
	}
	sheets, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}
	grid, ok := sheets["Sheet1"]
	if !ok {
		t.Fatalf("Sheet1 missing, got %v", keys(sheets))
	}
	if len(grid) != 5 {
		t.Fatalf("want 5 rows, got %d", len(grid))
	}
	if got := grid[0][0]; got != "Country" {
		t.Fatalf("header cell: want Country, got %q", got)
	}
	if got := grid[1][0]; got != "Australia" {
		t.Fatalf("row 1 name: want Australia, got %q", got)
	}
	if got := grid[1][1]; got != "1454" {
		t.Fatalf("row 1 count: want 1454, got %q", got)
	}
	if got := grid[1][2]; !strings.HasPrefix(got, "21.56") {
		t.Fatalf("row 1 PE: want ~21.56, got %q", got)
	}
	if got := grid[4][2]; got != "" {
		t.Fatalf("blank cell: want empty, got %q", got)
	}
}

// TestReadRejectsNonOLE guards the format check.
func TestReadRejectsNonOLE(t *testing.T) {
	if _, err := Read([]byte("not a workbook")); err == nil {
		t.Fatal("expected an error for a non-OLE2 stream")
	}
}

func keys(m map[string][][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
