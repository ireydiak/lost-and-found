# REQ Import — Design

**Date:** 2026-07-06
**Goal:** Load every bike shop (and pawn shop) on the Island of Montreal from the Registre des entreprises du Québec (REQ) into the local Postgres database.

## Source

The REQ search website cannot be scraped (Cloudflare challenge, ASP.NET postback flow, terms of use prohibit it). Instead we use the official open-data dump published on Données Québec, updated twice a month:

- Dataset: https://www.donneesquebec.ca/recherche/dataset/entreprises
- Direct ZIP: https://www.registreentreprises.gouv.qc.ca/RQAnonymeGR/GR/GR03/GR03A2_22A_PIU_RecupDonnPub_PC/FichierDonneesOuvertes.aspx

The ZIP download endpoint is also behind Cloudflare, so **the user downloads the ZIP manually in a browser** and passes its path to the import command. This is acceptable: the file changes at most twice a month.

## Tool

One new Go command in this repo, no new abstractions, plain procedural code:

```bash
DATABASE_URL=postgres://... go run ./cmd/import-req -zip ~/Downloads/FichierDonneesOuvertes.zip
DATABASE_URL=postgres://... go run ./cmd/import-req -zip ... -dry-run   # writes preview CSV, no DB writes
```

A Makefile target wraps the common invocation.

## Pipeline

1. Open the ZIP and stream the needed CSVs with `encoding/csv` (no extraction to disk; memory stays flat).
   - `Entreprise.csv` — NEQ, registration status (immatriculée / radiée), legal form.
   - `Nom.csv` — legal names **and trade names** ("autres noms"). Trade names matter: many shops are legally "9123-4567 Québec inc." and only the trade name contains the bike word.
   - `Etablissements.csv` — one row per physical location: establishment sequence number, address lines, CAE economic activity codes.
   - `DomaineValeur.csv` — lookup table used once at implementation time to confirm the exact CAE codes and their labels.
2. Filter (see Matching below).
3. Upsert into `addresses`, `shops`, `shops_tags`.
4. Print a summary: rows scanned / matched / inserted / updated / skipped.

**Note:** exact CSV column names are confirmed against the real dump as the first implementation step (the official PDF guide does not extract cleanly). The design relies on the documented semantics above.

## Matching

An establishment qualifies if **either**:

- **CAE code** is in a curated list, confirmed against `DomaineValeur.csv` at implementation time:
  - bicycle / sporting-goods retail (e.g. 6541 "Vente au détail de bicyclettes et d'articles de sport"),
  - bike repair codes,
  - pawn shop / used-merchandise codes (prêteurs sur gages).
- **Name keyword** matches (case- and accent-insensitive, word-boundary-aware) on legal or trade name:
  - bike: `velo`, `bicycle`, `bicyclette`, `bike`, `cycle`, `cycles`, `cyclerie`, `bixi`
  - pawn: `pawn`, `pret sur gages`, `prets sur gages`
  - stoplist for known noise: `motocycle`, `recycl`, `cyclisme`-adjacent noise is caught in dry-run review.

**Geographic filter — Island of Montreal:** postal code starts with `H` and not `H7` (Laval). City-name check as sanity backup. Borough parsed from city strings like "Montréal (Lachine)" when present, else null.

## Schema mapping

| Target | Source |
|---|---|
| `shops.name` | trade name if present, else legal name |
| `shops.neq_id` | NEQ |
| `shops.neq_etab_id` | establishment sequence number (**new column**, see Migration) |
| `shops.status` | immatriculée → `active`, radiée → `closed` |
| `shops.phone/email/website/socials` | null — not in the dump; future data sources |
| `addresses.*` | parsed from establishment address lines (street number, street name, city, borough, postal code) |
| `addresses.location` | null — dump has no coordinates; geocoding is a separate future step |
| `shops_tags` | from match reason: bike CAE → `bike-shop` (+ `sporting-goods` when the code is the combined sports code, + `repair` for repair codes); pawn CAE/keyword → `pawn-shop`; bike keyword-only → `bike-shop`; otherwise `unknown` |

One shop row **per establishment** — a chain (e.g. Quilicot) yields one row per location.

## Migration

New goose migration (timestamp-named):

```sql
ALTER TABLE shops ADD COLUMN neq_etab_id INTEGER;
CREATE UNIQUE INDEX uq_shops_neq_id_etab ON shops(neq_id, neq_etab_id);
```

`(neq_id, neq_etab_id)` is the idempotent upsert key. Re-running the import refreshes existing rows and inserts new ones. Addresses are updated in place via the shop's `address_id`.

## Error handling

- Malformed CSV rows: log and skip, never abort.
- Unparseable addresses: import the shop anyway with the best-effort address; log it.
- The run always ends with the summary counts.

## Testing

- Unit tests: name matcher (accents, word boundaries, stoplist), address-line parser, island filter, status mapping — using fixture rows shaped like the real CSVs.
- Integration check: `-dry-run` against the real dump, human review of the preview CSV.

## Out of scope (later)

- Automated download of the ZIP.
- Geocoding `addresses.location`.
- Phone / email / website / social enrichment from other sources.
- Other data sources (Google Places, OSM, etc.).
