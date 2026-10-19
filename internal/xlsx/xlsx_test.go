package xlsx

import (
	"os"
	"testing"
)

func TestReadAndReadSheet(t *testing.T) {
	data, err := os.ReadFile("testdata/sample.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	sheets, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(sheets) != 2 {
		t.Fatalf("want 2 sheets, got %d", len(sheets))
	}
	if got := sheets["Data"][0][0]; got != "Country" {
		t.Fatalf("header: want Country, got %q", got)
	}
	if got := sheets["Data"][1][0]; got != "Australia" {
		t.Fatalf("row 1: want Australia, got %q", got)
	}

	rows, err := ReadSheet(data, "Data")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	if rows[1][1] != "1454" {
		t.Fatalf("numeric cell: want 1454, got %q", rows[1][1])
	}
	if _, err := ReadSheet(data, "Missing"); err == nil {
		t.Fatal("expected an error for a missing sheet")
	}
}

func TestReadRejectsNonZip(t *testing.T) {
	if _, err := Read([]byte("not a workbook")); err == nil {
		t.Fatal("expected an error")
	}
}
