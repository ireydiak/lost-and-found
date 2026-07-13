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
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Address string `json:"address"`
}

func toFeatureCollection(rows []shopRow) featureCollection {
	fc := featureCollection{Type: "FeatureCollection", Features: []feature{}}
	for _, r := range rows {
		parts := []string{r.StreetNumber + " " + r.StreetName}
		if r.City != "" {
			parts = append(parts, r.City)
		}
		parts = append(parts, r.PostalCode)
		fc.Features = append(fc.Features, feature{
			Type:     "Feature",
			Geometry: geometry{Type: "Point", Coordinates: [2]float64{r.Lon, r.Lat}},
			Properties: props{
				ID:      r.ID,
				Name:    r.Name,
				Status:  r.Status,
				Address: strings.Join(parts, ", "),
			},
		})
	}
	return fc
}
