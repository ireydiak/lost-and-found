package main

import "testing"

func TestParseAddress(t *testing.T) {
	cases := []struct {
		desc  string
		lines []string
		want  Address
		ok    bool
	}{
		{
			desc:  "typical establishment address",
			lines: []string{"9101 boul. Louis-H.-La Fontaine", "Montréal (Québec)", "", "H1J1Z1"},
			want:  Address{StreetNumber: "9101", StreetName: "boul. Louis-H.-La Fontaine", City: "Montréal", PostalCode: "H1J 1Z1"},
			ok:    true,
		},
		{
			desc:  "former municipality becomes borough",
			lines: []string{"2707, CAZENEUVE", "SAINT-LAURENT (QUÉBEC)", "", "H4R1Z7"},
			want:  Address{StreetNumber: "2707", StreetName: "CAZENEUVE", City: "Montréal", Borough: "Saint-Laurent", PostalCode: "H4R 1Z7"},
			ok:    true,
		},
		{
			desc:  "suite-civic number keeps the civic part",
			lines: []string{"602-1411 rue Peel", "Montréal (Québec)", "", "H3A1S5"},
			want:  Address{StreetNumber: "1411", StreetName: "rue Peel", City: "Montréal", PostalCode: "H3A 1S5"},
			ok:    true,
		},
		{
			desc:  "non-numeric unit prefix",
			lines: []string{"PS141-7141 RUE Sherbrooke O", "Montréal (Québec)", "", "H4B1R6"},
			want:  Address{StreetNumber: "7141", StreetName: "RUE Sherbrooke O", City: "Montréal", PostalCode: "H4B 1R6"},
			ok:    true,
		},
		{
			desc:  "on-island suburb stays a city",
			lines: []string{"120 av. Westminster N", "Montréal-Ouest (Québec)", "", "H4X1Z9"},
			want:  Address{StreetNumber: "120", StreetName: "av. Westminster N", City: "Montréal-Ouest", PostalCode: "H4X 1Z9"},
			ok:    true,
		},
		{
			desc:  "no postal code anywhere",
			lines: []string{"somewhere", "Montréal (Québec)", "", ""},
			ok:    false,
		},
	}
	for _, c := range cases {
		got, ok := parseAddress(c.lines)
		if ok != c.ok {
			t.Errorf("%s: parseAddress ok = %v, want %v", c.desc, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("%s: parseAddress = %+v, want %+v", c.desc, got, c.want)
		}
	}
}

func TestOnIsland(t *testing.T) {
	cases := []struct {
		pc   string
		want bool
	}{
		{"H1J 1Z1", true},  // Anjou
		{"H4X 1Z9", true},  // Montréal-Ouest
		{"H9X 3L4", true},  // Sainte-Anne-de-Bellevue
		{"H7T 2P6", false}, // Laval
		{"J4K 1A1", false}, // Longueuil
		{"G6X 3C7", false}, // Lévis
		{"", false},
	}
	for _, c := range cases {
		if got := onIsland(c.pc); got != c.want {
			t.Errorf("onIsland(%q) = %v, want %v", c.pc, got, c.want)
		}
	}
}
