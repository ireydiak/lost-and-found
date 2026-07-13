package main

import (
	"reflect"
	"testing"
)

func TestMatchName(t *testing.T) {
	cases := []struct {
		name string
		want []string
	}{
		// bike keywords, accent- and case-insensitive
		{"VÉLOSSERIE J.R. INC.", []string{"bike-shop"}},
		{"Vélo Espresso", []string{"bike-shop"}},
		{"DUMOULIN BICYCLETTES", []string{"bike-shop"}},
		{"McWhinnie's Bike Shop", []string{"bike-shop"}},
		{"E-Bike Montréal", []string{"bike-shop"}},
		{"CYCLE NÉRON INC.", []string{"bike-shop"}},
		{"LES CYCLES ST-ONGE", []string{"bike-shop"}},
		{"LA CYCLERIE", []string{"bike-shop"}},
		{"BIXI MONTRÉAL", []string{"bike-shop"}},

		// university "cycles supérieurs" (graduate studies) is not a bike shop
		{"ASSOCIATION DES ÉTUDIANTS AUX CYCLES SUPÉRIEURS DE L'UDEM", nil},
		{"ASSOCIATION ÉTUDIANTE DU 2E CYCLE EN DROIT", nil},
		{"Association étudiante de 2ème cycle de l'école de design", nil},
		{"ASSOCIATION DU PREMIER CYCLE EN GÉNIE", nil},
		{"ASSOCIATION DES ÉTUDIANTS DES CYCLES DE PHARMACIE", nil},

		// student orgs with a real bike word still match
		{"COOP VÉLO ÉTUDIANTE", []string{"bike-shop"}},

		// token boundaries: no substring false positives
		{"MOTOCYCLETTES LAPOINTE", nil},
		{"RECYCLAGE NOTRE-DAME INC.", nil},
		{"CYCLORAMA DE JÉRUSALEM", nil},

		// plain non-matches
		{"9123-4567 QUÉBEC INC.", nil},
		{"BOULANGERIE ST-VIATEUR", nil},
	}
	for _, c := range cases {
		got := matchName(c.name)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("matchName(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestMatchCAE(t *testing.T) {
	cases := []struct {
		code string
		want []string
	}{
		{"6542", []string{"bike-shop"}}, // retail: bicycles
		{"6540", nil},                   // retail: sport + bicycles — no longer imported
		{"6541", nil},                   // retail: sporting goods — no longer imported
		{"7311", nil},
		{"", nil},
	}
	for _, c := range cases {
		got := matchCAE(c.code)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("matchCAE(%q) = %v, want %v", c.code, got, c.want)
		}
	}
}
