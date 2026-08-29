package main

import (
	"bytes"
	"strings"
	"testing"
)

func sampleExportRows() []exportRow {
	return []exportRow{
		{Name: "Vélo Oliver inc.", Status: "active", StreetNumber: "153", StreetName: "av. Millhaven",
			City: "Pointe-Claire", PostalCode: "H9R 3V9", Tags: []string{"bike-shop"}},
		{Name: "LE BIKE SHOP", Status: "closed", StreetNumber: "363", StreetName: "RUE BLEIGNIER",
			PostalCode: "H4N 1B1"},
		{Name: "Coop Vélo Communautaire", Status: "active", StreetNumber: "1", StreetName: "rue Ontario",
			City: "Montréal", PostalCode: "H2X 1Y4", Tags: []string{"bike-shop", "non-profit"}},
		{Name: "Ancien Atelier", Status: "inactive", StreetNumber: "9", StreetName: "rue Rachel",
			PostalCode: "H2J 2J2"},
	}
}

func TestFilterExportRowsStatus(t *testing.T) {
	rows := sampleExportRows()

	got := filterExportRows(rows, nil, nil, "")
	if len(got) != 4 {
		t.Fatalf("nil status filter: got %d rows, want 4", len(got))
	}

	got = filterExportRows(rows, map[string]bool{"active": true}, nil, "")
	if len(got) != 2 || got[0].Name != "Vélo Oliver inc." || got[1].Name != "Coop Vélo Communautaire" {
		t.Errorf("active filter = %+v", got)
	}

	got = filterExportRows(rows, map[string]bool{"closed": true, "inactive": true}, nil, "")
	if len(got) != 2 || got[0].Name != "LE BIKE SHOP" || got[1].Name != "Ancien Atelier" {
		t.Errorf("closed+inactive filter = %+v", got)
	}
}

func TestFilterExportRowsCategory(t *testing.T) {
	rows := sampleExportRows()

	got := filterExportRows(rows, nil, map[string]bool{"non-profit": true}, "")
	if len(got) != 1 || got[0].Name != "Coop Vélo Communautaire" {
		t.Errorf("non-profit filter = %+v", got)
	}

	got = filterExportRows(rows, nil, map[string]bool{"for-profit": true}, "")
	if len(got) != 3 {
		t.Errorf("for-profit filter = %+v, want 3 rows", got)
	}
}

func TestFilterExportRowsSearch(t *testing.T) {
	rows := sampleExportRows()

	// Accent- and case-insensitive substring match on name, like shops.html's fold().
	got := filterExportRows(rows, nil, nil, "velo")
	if len(got) != 2 || got[0].Name != "Vélo Oliver inc." || got[1].Name != "Coop Vélo Communautaire" {
		t.Errorf("search 'velo' = %+v", got)
	}

	got = filterExportRows(rows, nil, nil, "BIKE SHOP")
	if len(got) != 1 || got[0].Name != "LE BIKE SHOP" {
		t.Errorf("search 'BIKE SHOP' = %+v", got)
	}
}

func TestFilterExportRowsCombined(t *testing.T) {
	rows := sampleExportRows()
	got := filterExportRows(rows,
		map[string]bool{"active": true}, map[string]bool{"non-profit": true}, "coop")
	if len(got) != 1 || got[0].Name != "Coop Vélo Communautaire" {
		t.Errorf("combined filter = %+v", got)
	}
}

func TestWriteCSV(t *testing.T) {
	rows := []exportRow{
		{
			Name: "Vélo Oliver inc.", Status: "active",
			StreetNumber: "153", StreetName: "av. Millhaven", City: "Pointe-Claire", PostalCode: "H9R 3V9",
			Phone: strp("514-555-1234"), Email: strp("info@velooliver.example"), Website: strp("https://velooliver.example"),
			NeqID: strp("1234567890"), NeqEtabID: 2,
		},
		{
			Name: "LE BIKE SHOP", Status: "closed",
			StreetNumber: "363", StreetName: "RUE BLEIGNIER", PostalCode: "H4N 1B1",
		},
	}
	var buf bytes.Buffer
	if err := writeCSV(&buf, rows); err != nil {
		t.Fatalf("writeCSV: %v", err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (header + 2 rows): %q", len(lines), buf.String())
	}
	if want := "Name,Address,Phone,Email,Website,NEQ ID,NEQ Establishment ID"; lines[0] != want {
		t.Errorf("header = %q, want %q", lines[0], want)
	}
	if want := "Vélo Oliver inc.,\"153 av. Millhaven, Pointe-Claire, H9R 3V9\",514-555-1234,info@velooliver.example,https://velooliver.example,1234567890,2"; lines[1] != want {
		t.Errorf("row 1 = %q, want %q", lines[1], want)
	}
	if want := "LE BIKE SHOP,\"363 RUE BLEIGNIER, H4N 1B1\",,,,,"; lines[2] != want {
		t.Errorf("row 2 = %q, want %q", lines[2], want)
	}
}
