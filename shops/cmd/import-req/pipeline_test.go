package main

import (
	"archive/zip"
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
)

func TestStatusFromCode(t *testing.T) {
	cases := []struct {
		code string
		want string
	}{
		{"IM", "active"}, // immatriculée
		{"RD", "closed"}, // radiée sur demande
		{"RO", "closed"}, // radiée d'office
		{"RX", "closed"}, // radiée d'office (article 59)
		{"AI", ""},       // avis d'intention: not registered, skip
		{"NI", ""},       // non immatriculée: skip
		{"", ""},
	}
	for _, c := range cases {
		if got := statusFromCode(c.code); got != c.want {
			t.Errorf("statusFromCode(%q) = %q, want %q", c.code, got, c.want)
		}
	}
}

// writeNomZip builds a ZIP containing a Nom.csv with the given rows.
func writeNomZip(t *testing.T, rows [][]string) *zip.ReadCloser {
	t.Helper()
	path := filepath.Join(t.TempDir(), "req.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("Nom.csv")
	if err != nil {
		t.Fatal(err)
	}
	cw := csv.NewWriter(w)
	header := []string{"NEQ", "NOM_ASSUJ", "NOM_ASSUJ_LANG_ETRNG", "STAT_NOM", "TYP_NOM_ASSUJ", "DAT_INIT_NOM_ASSUJ", "DAT_FIN_NOM_ASSUJ"}
	if err := cw.Write(header); err != nil {
		t.Fatal(err)
	}
	if err := cw.WriteAll(rows); err != nil {
		t.Fatal(err)
	}
	cw.Flush()
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { z.Close() })
	return z
}

func TestCollectNamesFallsBackToFormerNames(t *testing.T) {
	z := writeNomZip(t, [][]string{
		// Closed enterprise: the registry flags every name "A" (antérieur).
		// The most recent former name (latest DAT_FIN) should be kept.
		{"111", "ANCIENNE DÉNOMINATION INC.", "", "A", "N", "2005-01-01", "2010-09-08"},
		{"111", "CYCLES CITÉVERT INC.", "GREENCITY CYCLES INC.", "A", "N", "2010-09-08", "2016-08-15"},
		{"111", "LE VIEUX MAGASIN", "", "A", "A", "2005-01-01", "2012-01-01"},
		// Active enterprise: the in-force name must still win over former ones.
		{"222", "OLD NAME INC.", "", "A", "N", "2000-01-01", "2018-05-05"},
		{"222", "CURRENT NAME INC.", "", "V", "N", "2018-05-05", ""},
	})
	cands := map[string]*Candidate{"111": {}, "222": {}}
	if err := collectNames(z, cands); err != nil {
		t.Fatal(err)
	}
	if got, want := cands["111"].LegalName, "CYCLES CITÉVERT INC."; got != want {
		t.Errorf("closed enterprise LegalName = %q, want most recent former name %q", got, want)
	}
	if got, want := cands["111"].TradeName, "LE VIEUX MAGASIN"; got != want {
		t.Errorf("closed enterprise TradeName = %q, want former trade name %q", got, want)
	}
	if got, want := cands["222"].LegalName, "CURRENT NAME INC."; got != want {
		t.Errorf("active enterprise LegalName = %q, want in-force name %q", got, want)
	}
}

func TestPickName(t *testing.T) {
	cases := []struct {
		desc                          string
		estabName, trade, legal, want string
	}{
		{"establishment name matching keywords wins", "VÉLO MTL", "AUTRE NOM", "9123-4567 QUÉBEC INC.", "VÉLO MTL"},
		{"trade name beats numbered legal name", "9123-4567 QUÉBEC INC.", "CYCLES ST-ONGE", "9123-4567 QUÉBEC INC.", "CYCLES ST-ONGE"},
		{"establishment name beats legal when no trade name", "DUMOULIN BICYCLETTES", "", "GESTION DUMOULIN INC.", "DUMOULIN BICYCLETTES"},
		{"legal name as last resort", "", "", "VÉLOSSERIE J.R. INC.", "VÉLOSSERIE J.R. INC."},
	}
	for _, c := range cases {
		if got := pickName(c.estabName, c.trade, c.legal); got != c.want {
			t.Errorf("%s: pickName = %q, want %q", c.desc, got, c.want)
		}
	}
}
