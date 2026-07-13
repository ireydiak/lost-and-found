# Shops

Listing of existing bike shops in Montreal.

# Build

```bash
make build
```

# Local development

Set environment variables

```bash
DATABASE_URL=postgres://postgres:admin@localhost:5432/shops?sslmode=disable
```

## Start docker containers

```bash
make up
```

This will run database migrations by default.

## Import data from the Registre des entreprises du Québec

Download the dump (browser only) from the [Données Québec dataset](https://www.donneesquebec.ca/recherche/dataset/entreprises) 
and save it as `data/JeuDonnees.zip`.

Then, run this command to start the import process:
```bash

```bash
   DATABASE_URL=postgres://postgres:admin@localhost:5432/shops?sslmode=disable make import-req
```
```
```


## Create migrations

```bash
???
```
   ```

## Misc

### Connect to local database

Requires `pgcli` (`brew install pgcli`)

```
pgcli "postgres://postgres:admin@localhost:5432/shops?sslmode=disable"
```
