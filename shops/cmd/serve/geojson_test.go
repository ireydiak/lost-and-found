package main

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func TestToFeatureCollection(t *testing.T) {
	rows := []shopRow{
		{
			Name: "Vélo Oliver inc.", Status: "active",
			StreetNumber: "153", StreetName: "av. Millhaven", City: "Pointe-Claire", PostalCode: "H9R 3V9",
			Lon: -73.81, Lat: 45.46,
			Tags: []string{"bike-shop", "non-profit"},
		},
		{
			Name: "LE BIKE SHOP", Status: "closed",
			StreetNumber: "363", StreetName: "RUE BLEIGNIER", City: "", PostalCode: "H4N 1B1",
			Lon: -73.66, Lat: 45.53,
		},
	}
	fc := toFeatureCollection(rows)

	if fc.Type != "FeatureCollection" || len(fc.Features) != 2 {
		t.Fatalf("collection = %+v", fc)
	}
	f := fc.Features[0]
	if f.Type != "Feature" || f.Geometry.Type != "Point" {
		t.Errorf("feature envelope = %+v", f)
	}
	if f.Geometry.Coordinates != [2]float64{-73.81, 45.46} { // GeoJSON order: lon, lat
		t.Errorf("coordinates = %v", f.Geometry.Coordinates)
	}
	if got, want := f.Properties.Address, "153 av. Millhaven, Pointe-Claire, H9R 3V9"; got != want {
		t.Errorf("address = %q, want %q", got, want)
	}
	if got, want := f.Properties.Tags, []string{"bike-shop", "non-profit"}; !slices.Equal(got, want) {
		t.Errorf("tags = %v, want %v", got, want)
	}
	// empty city is skipped in the formatted address
	if got, want := fc.Features[1].Properties.Address, "363 RUE BLEIGNIER, H4N 1B1"; got != want {
		t.Errorf("address = %q, want %q", got, want)
	}

	// must marshal to valid GeoJSON field names
	b, err := json.Marshal(fc)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"FeatureCollection"`, `"geometry"`, `"coordinates"`, `"properties"`, `"name"`, `"status"`, `"address"`, `"tags"`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("marshaled GeoJSON missing %s: %s", key, b)
		}
	}
}
