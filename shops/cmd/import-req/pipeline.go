package main

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"sort"
	"strconv"
	"strings"
)

// Estab is one physical location from Etablissements.csv.
type Estab struct {
	EtabNo    int
	Name      string
	AddrLines []string
	MatchTags []string // tags earned by this establishment's own CAE codes
}

// Candidate is an enterprise (NEQ) that might own one or more shops.
type Candidate struct {
	Status    string   // COD_STAT_IMMAT
	LegalName string   // current legal name (TYP_NOM N/M)
	TradeName string   // current "autre nom" (TYP_NOM A)
	Tags      []string // tags earned at the enterprise level (enterprise CAE)
	Domicile  []string // domicile address lines
}

// ShopRow is one row ready for the shops table.
type ShopRow struct {
	NEQ    string
	EtabNo int // 0 = from the enterprise domicile address
	Name   string
	Status string
	Addr   Address
	Tags   []string
}

// statusFromCode maps REQ registration statuses to the shop_status enum.
// Empty string means the record should be skipped.
func statusFromCode(code string) string {
	switch code {
	case "IM":
		return "active"
	case "RD", "RO", "RX":
		return "closed"
	}
	return ""
}

// pickName chooses the display name for a shop: the establishment's own name if
// it matches our keywords, then the trade name, then the establishment name,
// then the legal name.
func pickName(estabName, tradeName, legalName string) string {
	if matchName(estabName) != nil {
		return estabName
	}
	if tradeName != "" {
		return tradeName
	}
	if estabName != "" {
		return estabName
	}
	return legalName
}

// addTags appends new tags, keeping the list unique.
func addTags(dst []string, tags []string) []string {
	for _, t := range tags {
		found := false
		for _, d := range dst {
			if d == t {
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, t)
		}
	}
	return dst
}

// openCSV opens one file inside the ZIP and returns a reader plus a
// column-name → index map (the dump files start with a UTF-8 BOM).
func openCSV(z *zip.ReadCloser, name string) (*csv.Reader, map[string]int, io.Closer, error) {
	f, err := z.Open(name)
	if err != nil {
		return nil, nil, nil, err
	}
	r := csv.NewReader(f)
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		f.Close()
		return nil, nil, nil, fmt.Errorf("%s: reading header: %w", name, err)
	}
	idx := make(map[string]int, len(header))
	for i, col := range header {
		idx[strings.TrimPrefix(col, "\ufeff")] = i
	}
	return r, idx, f, nil
}

// loadEstablishments reads all of Etablissements.csv into memory, keyed by NEQ,
// tagging each establishment that matches by CAE code or name.
func loadEstablishments(z *zip.ReadCloser) (map[string][]Estab, error) {
	r, idx, closer, err := openCSV(z, "Etablissements.csv")
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	estabs := make(map[string][]Estab)
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Etablissements.csv: skipping row: %v", err)
			continue
		}
		etabNo, _ := strconv.Atoi(row[idx["NO_SUF_ETAB"]])
		e := Estab{
			EtabNo: etabNo,
			Name:   row[idx["NOM_ETAB"]],
			AddrLines: []string{
				row[idx["LIGN1_ADR"]], row[idx["LIGN2_ADR"]],
				row[idx["LIGN3_ADR"]], row[idx["LIGN4_ADR"]],
			},
		}
		e.MatchTags = addTags(e.MatchTags, matchCAE(row[idx["COD_ACT_ECON"]]))
		e.MatchTags = addTags(e.MatchTags, matchCAE(row[idx["COD_ACT_ECON2"]]))
		neq := row[idx["NEQ"]]
		estabs[neq] = append(estabs[neq], e)
	}
	return estabs, nil
}

// scanEnterprises streams Entreprise.csv. For known candidates it fills in the
// registration status and domicile. Enterprises whose own CAE codes match
// become new candidates.
func scanEnterprises(z *zip.ReadCloser, cands map[string]*Candidate) error {
	r, idx, closer, err := openCSV(z, "Entreprise.csv")
	if err != nil {
		return err
	}
	defer closer.Close()

	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Entreprise.csv: skipping row: %v", err)
			continue
		}
		neq := row[idx["NEQ"]]
		caeTags := addTags(matchCAE(row[idx["COD_ACT_ECON_CAE"]]), matchCAE(row[idx["COD_ACT_ECON_CAE2"]]))
		c := cands[neq]
		if c == nil {
			if caeTags == nil {
				continue
			}
			c = &Candidate{}
			cands[neq] = c
		}
		c.Status = row[idx["COD_STAT_IMMAT"]]
		c.Tags = addTags(c.Tags, caeTags)
		c.Domicile = []string{
			row[idx["ADR_DOMCL_LIGN1_ADR"]], row[idx["ADR_DOMCL_LIGN2_ADR"]],
			row[idx["ADR_DOMCL_LIGN3_ADR"]], row[idx["ADR_DOMCL_LIGN4_ADR"]],
		}
	}
	return nil
}

// collectNames streams Nom.csv to pick up the legal and trade names of every
// candidate. Names currently in force (STAT_NOM "V") win, but
// struck-off enterprises only have former names ("A" antérieur), so the most
// recent former name is kept as a fallback.
func collectNames(z *zip.ReadCloser, cands map[string]*Candidate) error {
	r, idx, closer, err := openCSV(z, "Nom.csv")
	if err != nil {
		return err
	}
	defer closer.Close()

	type former struct {
		name    string
		matches bool
		end     string // DAT_FIN_NOM_ASSUJ, ISO date so string order is date order
	}
	formerLegal := make(map[string]former)
	formerTrade := make(map[string]former)

	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Nom.csv: skipping row: %v", err)
			continue
		}
		neq := row[idx["NEQ"]]
		c := cands[neq]
		stat := row[idx["STAT_NOM"]]
		if c == nil || (stat != "V" && stat != "A") {
			continue
		}
		name := row[idx["NOM_ASSUJ"]]
		end := row[idx["DAT_FIN_NOM_ASSUJ"]]
		switch row[idx["TYP_NOM_ASSUJ"]] {
		case "N", "M": // legal name / dénomination sociale
			if stat == "V" {
				if c.LegalName == "" {
					c.LegalName = name
				}
			} else if f := formerLegal[neq]; f.name == "" || end > f.end {
				formerLegal[neq] = former{name: name, end: end}
			}
		case "A": // autre nom (trade name); prefer one that matches our keywords
			matches := matchName(name) != nil || matchName(row[idx["NOM_ASSUJ_LANG_ETRNG"]]) != nil
			if stat == "V" {
				if c.TradeName == "" || (matches && matchName(c.TradeName) == nil) {
					c.TradeName = name
				}
			} else if f := formerTrade[neq]; f.name == "" || (matches && !f.matches) || (matches == f.matches && end > f.end) {
				formerTrade[neq] = former{name: name, matches: matches, end: end}
			}
		}
	}

	for neq, c := range cands {
		if c.LegalName == "" {
			c.LegalName = formerLegal[neq].name
		}
		if c.TradeName == "" {
			c.TradeName = formerTrade[neq].name
		}
	}
	return nil
}

// buildRows turns candidates into shop rows: one row per matching on-island
// establishment, falling back to the enterprise domicile when the enterprise
// matched but has no establishment row on the island.
func buildRows(cands map[string]*Candidate, estabs map[string][]Estab) []ShopRow {
	var rows []ShopRow
	for neq, c := range cands {
		status := statusFromCode(c.Status)
		if status == "" {
			continue
		}
		placed := false
		for _, e := range estabs[neq] {
			if len(e.MatchTags) == 0 && len(c.Tags) == 0 {
				continue // this location matches nothing itself, and neither does its owner
			}
			addr, ok := parseAddress(e.AddrLines)
			if !ok || !onIsland(addr.PostalCode) {
				continue
			}
			rows = append(rows, ShopRow{
				NEQ:    neq,
				EtabNo: e.EtabNo,
				Name:   pickName(e.Name, c.TradeName, c.LegalName),
				Status: status,
				Addr:   addr,
				Tags:   addTags(append([]string{}, c.Tags...), e.MatchTags),
			})
			placed = true
		}
		if !placed && len(c.Tags) > 0 {
			addr, ok := parseAddress(c.Domicile)
			if !ok || !onIsland(addr.PostalCode) {
				continue
			}
			rows = append(rows, ShopRow{
				NEQ:    neq,
				EtabNo: 0,
				Name:   pickName("", c.TradeName, c.LegalName),
				Status: status,
				Addr:   addr,
				Tags:   append([]string{}, c.Tags...),
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Name != rows[j].Name {
			return rows[i].Name < rows[j].Name
		}
		return rows[i].EtabNo < rows[j].EtabNo
	})
	return rows
}
