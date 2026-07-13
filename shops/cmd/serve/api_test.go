package main

import (
	"strings"
	"testing"
)

func strp(s string) *string { return &s }

func TestShopPayloadValidate(t *testing.T) {
	fullAddress := &addressPayload{
		StreetNumber: strp("4551"), StreetName: strp("rue Sainte-Catherine E"),
		City: strp("Montréal"), PostalCode: strp("H1V 1Y8"),
	}
	cases := []struct {
		desc    string
		p       shopPayload
		create  bool
		wantErr string
	}{
		{"valid create", shopPayload{Name: strp("Vélo Test"), Address: fullAddress}, true, ""},
		{"create without name", shopPayload{Address: fullAddress}, true, "name"},
		{"create with blank name", shopPayload{Name: strp("  "), Address: fullAddress}, true, "name"},
		{"create without address", shopPayload{Name: strp("Vélo Test")}, true, "address"},
		{"create with partial address", shopPayload{Name: strp("Vélo Test"),
			Address: &addressPayload{StreetName: strp("rue X")}}, true, "street_number"},
		{"bad status", shopPayload{Name: strp("Vélo Test"), Address: fullAddress,
			Status: strp("open")}, true, "status"},
		{"good status", shopPayload{Name: strp("Vélo Test"), Address: fullAddress,
			Status: strp("inactive")}, true, ""},
		{"location outside Montreal", shopPayload{Name: strp("Vélo Test"), Address: fullAddress,
			Location: &locationPayload{Lat: 46.81, Lon: -71.21}}, true, "outside"},
		{"patch with only location", shopPayload{
			Location: &locationPayload{Lat: 45.55, Lon: -73.53}}, false, ""},
		{"patch with blank name", shopPayload{Name: strp("")}, false, "name"},
		{"patch with bad status", shopPayload{Status: strp("dead")}, false, "status"},
		{"empty patch", shopPayload{}, false, "at least one"},
	}
	for _, c := range cases {
		err := c.p.validate(c.create)
		switch {
		case c.wantErr == "" && err != nil:
			t.Errorf("%s: unexpected error %v", c.desc, err)
		case c.wantErr != "" && err == nil:
			t.Errorf("%s: expected error mentioning %q, got nil", c.desc, c.wantErr)
		case c.wantErr != "" && !strings.Contains(err.Error(), c.wantErr):
			t.Errorf("%s: error %q does not mention %q", c.desc, err, c.wantErr)
		}
	}
}
