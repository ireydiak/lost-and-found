# analysis

Exploratory analysis of the tagged Facebook posts. `stolen_bikes.ipynb` covers:

1. Number of posts per tag (descending).
2. Monthly evolution of `stolen` and `abandoned` posts, with a seasonal index for bike theft.

The notebook reads the same Postgres database the rest of the pipeline uses, so posts must be loaded and tagged first.

## Quickstart

From the repo root:

```sh
make up                                  # start Postgres (docker compose)
make migrate                             # apply migrations
make load DIR=sourcing/raw/<snapshot>    # load posts (skip if already loaded)
make classify                            # tag posts at :8080 (skip if already tagged)
```

Then install the Python dependencies and open the notebook:

```sh
cd analysis
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
jupyter lab stolen_bikes.ipynb
```

Run all cells (Kernel > Restart & Run All). To execute it headlessly and save the outputs into the notebook:

```sh
jupyter nbconvert --to notebook --execute --inplace stolen_bikes.ipynb
```

## Configuration

- `DATABASE_URL` — defaults to the docker-compose Postgres
  (`postgres://postgres:admin@localhost:5433/findmybike`). Export your own before launching Jupyter to point elsewhere.
- `WINDOW` (in the seasonality cell) — the months used for the seasonal index. Only months with reasonably complete scraping coverage should be included; widen it as older history is sourced.

## Caveats

- Posts are deduplicated on `facebook_id`, falling back to `dedup_key` (see `extractor/docs/DATABASE.md`).
- Seasonality can't be estimated reliably with under two full years of data, and months before May 2026 are sparse because of how far back the scraper reached.
