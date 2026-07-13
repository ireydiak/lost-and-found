package nominatim

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestExpandStreet(t *testing.T) {
	cases := []struct{ in, want string }{
		{"boul. Métropolitain", "boulevard Métropolitain"},
		{"av. Millhaven", "avenue Millhaven"},
		{"ch. de la Côte-des-Neiges", "chemin de la Côte-des-Neiges"},
		{"RUE BLEIGNIER", "RUE BLEIGNIER"},
		{"Av. du Parc", "avenue du Parc"},
		{"côte du Beaver Hall", "côte du Beaver Hall"},
		{"av. du Mont-Royal E", "avenue du Mont-Royal Est"},
		{"rue Notre-Dame O", "rue Notre-Dame Ouest"},
		{"boul. Gouin E", "boulevard Gouin Est"},
	}
	for _, c := range cases {
		if got := expandStreet(c.in); got != c.want {
			t.Errorf("expandStreet(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// fakeNominatim answers /search with the queued responses, in order, and
// records the query values of each request.
func fakeNominatim(t *testing.T, responses ...[]result) (*Geocoder, *[]url.Values) {
	t.Helper()
	var seen []url.Values
	i := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Query())
		if i >= len(responses) {
			t.Fatalf("unexpected extra request: %s", r.URL)
		}
		json.NewEncoder(w).Encode(responses[i])
		i++
	}))
	t.Cleanup(srv.Close)
	g := &Geocoder{BaseURL: srv.URL, Client: srv.Client(), Wait: func() {}}
	return g, &seen
}

func TestGeocodeStructuredHit(t *testing.T) {
	g, seen := fakeNominatim(t, []result{{Lat: "45.5017", Lon: "-73.5673"}})
	a := Address{StreetNumber: "5727", StreetName: "boul. Métropolitain", City: "Montréal", PostalCode: "H1P 1X3"}
	pt, ok, err := g.Geocode(a)
	if err != nil || !ok {
		t.Fatalf("geocode: ok=%v err=%v", ok, err)
	}
	if pt.Lat != 45.5017 || pt.Lon != -73.5673 {
		t.Errorf("point = %+v", pt)
	}
	q := (*seen)[0]
	if got, want := q.Get("street"), "5727 boulevard Métropolitain"; got != want {
		t.Errorf("street = %q, want %q", got, want)
	}
	if got, want := q.Get("postalcode"), "H1P 1X3"; got != want {
		t.Errorf("postalcode = %q, want %q", got, want)
	}
	if got, want := q.Get("city"), "Montréal"; got != want {
		t.Errorf("city = %q, want %q", got, want)
	}
}

func TestGeocodeFallsBackToFreeform(t *testing.T) {
	g, seen := fakeNominatim(t,
		[]result{}, // structured query misses
		[]result{{Lat: "45.52", Lon: "-73.60"}},
	)
	a := Address{StreetNumber: "363", StreetName: "RUE BLEIGNIER", PostalCode: "H4N 1B1"}
	pt, ok, err := g.Geocode(a)
	if err != nil || !ok {
		t.Fatalf("geocode: ok=%v err=%v", ok, err)
	}
	if pt.Lat != 45.52 {
		t.Errorf("point = %+v", pt)
	}
	if len(*seen) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(*seen))
	}
	// empty city falls back to Montréal in the free-form query
	if got, want := (*seen)[1].Get("q"), "363 RUE BLEIGNIER, Montréal, QC H4N 1B1"; got != want {
		t.Errorf("q = %q, want %q", got, want)
	}
}

func TestGeocodeRejectsPostalCodeMismatch(t *testing.T) {
	// The structured query matches a same-named street in Montréal-Est
	// (in bounds, but postal H1B instead of H1V) — it must be rejected,
	// and the free-form result with the right postal code accepted.
	g, seen := fakeNominatim(t,
		[]result{{Lat: "45.6311", Lon: "-73.5070", Address: resultAddress{Postcode: "H1B 2V7"}}},
		[]result{{Lat: "45.5533", Lon: "-73.5326", Address: resultAddress{Postcode: "H1V 1Y8"}}},
	)
	a := Address{StreetNumber: "4551", StreetName: "rue Sainte-Catherine E", City: "Montréal", PostalCode: "H1V 1Y8"}
	pt, ok, err := g.Geocode(a)
	if err != nil || !ok {
		t.Fatalf("geocode: ok=%v err=%v", ok, err)
	}
	if pt.Lat != 45.5533 {
		t.Errorf("point = %+v, want the free-form result", pt)
	}
	if len(*seen) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(*seen))
	}
}

func TestGeocodeTrustsExactHouseNumberOverPostcode(t *testing.T) {
	// OSM's postcode for a building often disagrees with the decades-old REQ
	// one (H2J vs H2G); an exact house-number match on the street is trusted.
	g, seen := fakeNominatim(t, []result{
		{Lat: "45.5389", Lon: "-73.5863", Address: resultAddress{HouseNumber: "1833", Postcode: "H2J 1A9"}},
	})
	a := Address{StreetNumber: "1833", StreetName: "rue Dandurand", City: "Montréal", PostalCode: "H2G 1Y6"}
	pt, ok, err := g.Geocode(a)
	if err != nil || !ok {
		t.Fatalf("geocode: ok=%v err=%v", ok, err)
	}
	if pt.Lat != 45.5389 || len(*seen) != 1 {
		t.Errorf("point = %+v after %d requests, want the first hit", pt, len(*seen))
	}
}

func TestGeocodeAcceptsResultWithoutPostcode(t *testing.T) {
	// Street-level matches can lack a postcode; only a definite mismatch rejects.
	g, _ := fakeNominatim(t, []result{{Lat: "45.52", Lon: "-73.60"}})
	a := Address{StreetNumber: "100", StreetName: "rue Rachel E", City: "Montréal", PostalCode: "H2W 1C8"}
	_, ok, err := g.Geocode(a)
	if err != nil || !ok {
		t.Fatalf("geocode: ok=%v err=%v", ok, err)
	}
}

func TestGeocodeRejectsResultOutsideMontreal(t *testing.T) {
	// Same street name exists in Quebec City; the hit must be discarded.
	g, _ := fakeNominatim(t,
		[]result{{Lat: "46.8139", Lon: "-71.2080"}},
		[]result{},
	)
	a := Address{StreetNumber: "1", StreetName: "rue Saint-Jean", City: "Montréal", PostalCode: "H2X 1Z9"}
	_, ok, err := g.Geocode(a)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected out-of-bounds result to be rejected")
	}
}
