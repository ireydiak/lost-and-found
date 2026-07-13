// Package nominatim geocodes Montreal street addresses with the
// OpenStreetMap Nominatim API.
package nominatim

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const userAgent = "find-my-bike-shops/0.1 (https://github.com/ireydiak/find-my-bike)"

// Island of Montreal plus a small margin; hits outside are treated as misses
// so a same-named street elsewhere in Quebec never lands on the map.
const (
	MinLat, MaxLat = 45.35, 45.75
	MinLon, MaxLon = -74.05, -73.40
)

// InBounds reports whether a point is in the Montreal area.
func InBounds(lat, lon float64) bool {
	return lat >= MinLat && lat <= MaxLat && lon >= MinLon && lon <= MaxLon
}

// streetAbbrev expands the abbreviated street types used in the REQ dumps to
// the full French generic that OpenStreetMap uses.
var streetAbbrev = map[string]string{
	"av.":   "avenue",
	"boul.": "boulevard",
	"ch.":   "chemin",
	"mtée":  "montée",
	"pl.":   "place",
	"prom.": "promenade",
	"rte":   "route",
}

// cardinal expands the trailing direction letter (rue Beaubien E) to the full
// word OpenStreetMap uses (rue Beaubien Est).
var cardinal = map[string]string{
	"E": "Est", "O": "Ouest", "N": "Nord", "S": "Sud",
}

// expandStreet replaces a leading street-type abbreviation and a trailing
// cardinal direction with their full forms.
func expandStreet(name string) string {
	first, rest, found := strings.Cut(name, " ")
	if full, ok := streetAbbrev[strings.ToLower(first)]; ok && found {
		name = full + " " + rest
	}
	if i := strings.LastIndexByte(name, ' '); i >= 0 {
		if full, ok := cardinal[name[i+1:]]; ok {
			name = name[:i+1] + full
		}
	}
	return name
}

// fsa returns the forward sortation area (first three characters) of a postal
// code, normalized for comparison.
func fsa(postalCode string) string {
	s := strings.ToUpper(strings.ReplaceAll(postalCode, " ", ""))
	if len(s) < 3 {
		return s
	}
	return s[:3]
}

// Address is a street address to resolve.
type Address struct {
	StreetNumber string
	StreetName   string
	City         string
	Borough      string
	PostalCode   string
}

// Point is a WGS84 coordinate.
type Point struct {
	Lat, Lon float64
}

type resultAddress struct {
	HouseNumber string `json:"house_number"`
	Postcode    string `json:"postcode"`
}

type result struct {
	Lat     string        `json:"lat"`
	Lon     string        `json:"lon"`
	Address resultAddress `json:"address"`
}

// Geocoder queries a Nominatim endpoint, calling Wait before every request.
type Geocoder struct {
	BaseURL string
	Client  *http.Client
	Wait    func()
}

// New returns a Geocoder against the public OpenStreetMap endpoint, rate
// limited to its usage policy of one request per second.
func New() *Geocoder {
	t := time.NewTicker(1100 * time.Millisecond)
	return &Geocoder{
		BaseURL: "https://nominatim.openstreetmap.org",
		Client:  &http.Client{Timeout: 15 * time.Second},
		Wait:    func() { <-t.C },
	}
}

// Geocode resolves an address to a point: first a structured query, then a
// free-form one. ok is false when Nominatim has no trustworthy match.
func (g *Geocoder) Geocode(a Address) (Point, bool, error) {
	street := a.StreetNumber + " " + expandStreet(a.StreetName)
	city := a.City
	if city == "" {
		city = a.Borough
	}
	if city == "" {
		city = "Montréal"
	}

	structured := url.Values{
		"street":     {street},
		"city":       {city},
		"postalcode": {a.PostalCode},
	}
	pt, ok, err := g.search(structured, a.StreetNumber, fsa(a.PostalCode))
	if err != nil || ok {
		return pt, ok, err
	}

	freeform := url.Values{
		"q": {fmt.Sprintf("%s, %s, QC %s", street, city, a.PostalCode)},
	}
	return g.search(freeform, a.StreetNumber, fsa(a.PostalCode))
}

// search runs one Nominatim query. An exact house-number hit is trusted as is
// (OSM postcodes often disagree with decades-old REQ ones), but a street-level
// hit whose postal code disagrees with wantFSA is a miss: same-named streets
// exist across the island's municipalities, and the bounding box alone cannot
// tell them apart.
func (g *Geocoder) search(q url.Values, wantNumber, wantFSA string) (Point, bool, error) {
	q.Set("format", "jsonv2")
	q.Set("limit", "1")
	q.Set("countrycodes", "ca")
	q.Set("addressdetails", "1")

	g.Wait()
	req, err := http.NewRequest(http.MethodGet, g.BaseURL+"/search?"+q.Encode(), nil)
	if err != nil {
		return Point{}, false, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := g.Client.Do(req)
	if err != nil {
		return Point{}, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Point{}, false, fmt.Errorf("nominatim: HTTP %s", resp.Status)
	}

	var results []result
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return Point{}, false, fmt.Errorf("nominatim: decoding response: %w", err)
	}
	if len(results) == 0 {
		return Point{}, false, nil
	}
	lat, err := strconv.ParseFloat(results[0].Lat, 64)
	if err != nil {
		return Point{}, false, fmt.Errorf("nominatim: bad lat %q", results[0].Lat)
	}
	lon, err := strconv.ParseFloat(results[0].Lon, 64)
	if err != nil {
		return Point{}, false, fmt.Errorf("nominatim: bad lon %q", results[0].Lon)
	}
	if !InBounds(lat, lon) {
		return Point{}, false, nil
	}
	houseMatch := wantNumber != "" && strings.EqualFold(results[0].Address.HouseNumber, wantNumber)
	if got := results[0].Address.Postcode; !houseMatch && got != "" && wantFSA != "" && fsa(got) != wantFSA {
		return Point{}, false, nil
	}
	return Point{Lat: lat, Lon: lon}, true, nil
}
