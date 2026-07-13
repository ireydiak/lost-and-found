package main

import (
	"regexp"
	"strings"
)

// accentFold maps French accented letters to their ASCII base letter.
var accentFold = strings.NewReplacer(
	"À", "A", "Â", "A", "Ä", "A",
	"É", "E", "È", "E", "Ê", "E", "Ë", "E",
	"Î", "I", "Ï", "I",
	"Ô", "O", "Ö", "O",
	"Ù", "U", "Û", "U", "Ü", "U",
	"Ç", "C",
)

// tokenize uppercases, strips accents, and splits a name into letter/digit tokens.
func tokenize(name string) []string {
	s := accentFold.Replace(strings.ToUpper(name))
	return strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	})
}

// academicCycles marks names about university study cycles ("cycles supérieurs",
// "2e cycle", student associations...), which would otherwise match the CYCLE token.
var academicCycles = regexp.MustCompile(`(CYCLES? SUPERIEURS?|(1ERE?|2E(ME)?|3E(ME)?|PREMIER|DEUXIEME|TROISIEME) CYCLE|ETUDIANT)`)

// matchName returns the tag names implied by a business name, or nil. It is
// only used to prefer a recognizable bike-shop name when a shop has several.
// Matching is per token so substrings like MOTOCYCLETTE or RECYCLAGE don't hit.
func matchName(name string) []string {
	folded := accentFold.Replace(strings.ToUpper(name))
	academic := academicCycles.MatchString(folded)
	for _, tok := range tokenize(name) {
		switch {
		case tok == "CYCLE", tok == "CYCLES":
			if academic {
				continue
			}
			return []string{"bike-shop"}
		case strings.HasPrefix(tok, "VELO"),
			strings.HasPrefix(tok, "BICYCLE"),
			tok == "BIKE", tok == "BIKES", tok == "EBIKE", tok == "EBIKES",
			tok == "CYCLERIE", tok == "CYCLERIES",
			tok == "BIXI":
			return []string{"bike-shop"}
		}
	}
	return nil
}

// matchCAE returns the tag names implied by a CAE economic activity code, or
// nil. Only 6542 (retail: bicycles) qualifies a record for import.
func matchCAE(code string) []string {
	if code == "6542" {
		return []string{"bike-shop"}
	}
	return nil
}
