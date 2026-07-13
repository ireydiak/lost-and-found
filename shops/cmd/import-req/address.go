package main

import (
	"regexp"
	"strings"
)

// Address is a parsed REQ address, ready for the addresses table.
type Address struct {
	StreetNumber string
	StreetName   string
	City         string
	Borough      string
	PostalCode   string
}

// boroughs are former municipalities that are boroughs of Ville de Montréal today.
// The registry still uses their old names as the city. Keys are accent-folded uppercase.
var boroughs = map[string]string{
	"ANJOU":            "Anjou",
	"LACHINE":          "Lachine",
	"LASALLE":          "LaSalle",
	"L'ILE-BIZARD":     "L'Île-Bizard",
	"MONTREAL-NORD":    "Montréal-Nord",
	"OUTREMONT":        "Outremont",
	"PIERREFONDS":      "Pierrefonds",
	"ROXBORO":          "Roxboro",
	"SAINT-LAURENT":    "Saint-Laurent",
	"SAINT-LEONARD":    "Saint-Léonard",
	"SAINTE-GENEVIEVE": "Sainte-Geneviève",
	"VERDUN":           "Verdun",
}

// islandCities are on-island municipalities (canonical spelling), keyed accent-folded uppercase.
var islandCities = map[string]string{
	"MONTREAL":                "Montréal",
	"BAIE-D'URFE":             "Baie-D'Urfé",
	"BEACONSFIELD":            "Beaconsfield",
	"COTE-SAINT-LUC":          "Côte-Saint-Luc",
	"DOLLARD-DES-ORMEAUX":     "Dollard-des-Ormeaux",
	"DORVAL":                  "Dorval",
	"HAMPSTEAD":               "Hampstead",
	"KIRKLAND":                "Kirkland",
	"MONTREAL-EST":            "Montréal-Est",
	"MONTREAL-OUEST":          "Montréal-Ouest",
	"MONT-ROYAL":              "Mont-Royal",
	"POINTE-CLAIRE":           "Pointe-Claire",
	"SAINTE-ANNE-DE-BELLEVUE": "Sainte-Anne-de-Bellevue",
	"SENNEVILLE":              "Senneville",
	"WESTMOUNT":               "Westmount",
}

var (
	postalRe = regexp.MustCompile(`^([A-Z]\d[A-Z])\s?(\d[A-Z]\d)$`)
	cityRe   = regexp.MustCompile(`^(.+?)\s*\(QUEBEC\)$`)
)

// parseAddress extracts an Address from the dump's free-form address lines.
// It returns ok=false when no postal code is found.
func parseAddress(lines []string) (Address, bool) {
	var a Address
	streetIdx := -1
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		folded := accentFold.Replace(strings.ToUpper(line))
		if m := postalRe.FindStringSubmatch(folded); m != nil {
			a.PostalCode = m[1] + " " + m[2]
			continue
		}
		if m := cityRe.FindStringSubmatch(folded); m != nil {
			name := strings.TrimSpace(m[1])
			if b, ok := boroughs[name]; ok {
				a.City = "Montréal"
				a.Borough = b
			} else if c, ok := islandCities[name]; ok {
				a.City = c
			} else {
				// keep the source spelling, minus the "(Québec)" suffix
				a.City = strings.TrimSpace(line[:strings.LastIndex(line, "(")])
			}
			continue
		}
		if streetIdx == -1 {
			streetIdx = i
		}
	}
	if a.PostalCode == "" {
		return Address{}, false
	}
	if streetIdx >= 0 {
		a.StreetNumber, a.StreetName = splitStreet(strings.TrimSpace(lines[streetIdx]))
	}
	return a, true
}

// splitStreet separates the civic number from the street name.
// Handles "602-1411 rue Peel" (suite-civic) and "PS141-7141 RUE X" (unit prefix).
func splitStreet(line string) (number, name string) {
	first, rest, found := strings.Cut(line, " ")
	if !found {
		return "", line
	}
	first = strings.TrimSuffix(first, ",")
	if i := strings.LastIndex(first, "-"); i >= 0 {
		first = first[i+1:]
	}
	if first == "" || first[0] < '0' || first[0] > '9' || len(first) > 10 {
		return "", line
	}
	return first, strings.TrimSpace(rest)
}

// onIsland reports whether a postal code is on the Island of Montreal:
// forward sortation area H, excluding H7 (Laval).
func onIsland(postalCode string) bool {
	pc := strings.ToUpper(strings.ReplaceAll(postalCode, " ", ""))
	return len(pc) >= 2 && pc[0] == 'H' && pc[1] != '7'
}
