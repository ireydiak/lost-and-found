package main

import "strings"

// shopRow is one shop joined with its geocoded address.
type shopRow struct {
	ID           int64
	Name         string
	Status       string
	StreetNumber string
	StreetName   string
	City         string
	PostalCode   string
	Lon, Lat     float64
	Tags         []string
}

type featureCollection struct {
	Type     string    `json:"type"`
	Features []feature `json:"features"`
}

type feature struct {
	Type       string   `json:"type"`
	Geometry   geometry `json:"geometry"`
	Properties props    `json:"properties"`
}

type geometry struct {
	Type        string     `json:"type"`
	Coordinates [2]float64 `json:"coordinates"` // GeoJSON order: lon, lat
}

type props struct {
	ID      int64    `json:"id"`
	Name    string   `json:"name"`
	Status  string   `json:"status"`
	Address string   `json:"address"`
	Tags    []string `json:"tags"`
}

func toFeatureCollection(rows []shopRow) featureCollection {
	fc := featureCollection{Type: "FeatureCollection", Features: []feature{}}
	for _, r := range rows {
		fc.Features = append(fc.Features, feature{
			Type:     "Feature",
			Geometry: geometry{Type: "Point", Coordinates: [2]float64{r.Lon, r.Lat}},
			Properties: props{
				ID:      r.ID,
				Name:    r.Name,
				Status:  r.Status,
				Address: formatAddress(r.StreetNumber, r.StreetName, r.City, r.PostalCode),
				Tags:    r.Tags,
			},
		})
	}
	return fc
}

// formatAddress joins address parts the way shops.html does: number and
// street together, city only when known, postal code always last.
func formatAddress(streetNumber, streetName, city, postalCode string) string {
	parts := []string{streetNumber + " " + streetName}
	if city != "" {
		parts = append(parts, city)
	}
	parts = append(parts, postalCode)
	return strings.Join(parts, ", ")
}
