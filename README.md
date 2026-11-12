# Fees API

Go + Encore + Temporal fees API.

## Prerequisites

- [Encore CLI](https://encore.dev/docs/install)
- [Temporal CLI](https://docs.temporal.io/cli) (`brew install temporal`)
- [Docker](https://docs.docker.com/get-docker/) (running — Encore uses it for local Postgres)
- Go 1.22+

## Run

Terminal 1 — Temporal:

```bash
temporal server start-dev --db-filename ./.temporal/dev.db
```

Terminal 2 — API:

```bash
encore run
```

API: `http://127.0.0.1:4000`

Development Dashboard: `http://127.0.0.1:9400/mbvms`

Temporal UI: `http://localhost:8233`

If the DB schema is stale: `encore db reset pave_db`

## Test

```bash
go test ./fees/domain/ ./fees/workflows/
```

Optional — run those checks before each commit:

```bash
cp .githooks/pre-commit .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

## API

| Method | Path |
|--------|------|
| `POST` | `/fees/bill` |
| `POST` | `/fees/bill/:id/line-item` |
| `POST` | `/fees/bill/:id/close` |
| `GET` | `/fees/bill/:id` |
