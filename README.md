# Fees API

Go + Encore + Temporal fees API. Each bill is a Temporal workflow: add line items while `OPEN`, then close (manually or at `period_end`) for the invoice (line items + per-currency totals). Currencies: `USD`, `GEL` (`amount_minor` integers, no FX).

## Prerequisites

- [Encore CLI](https://encore.dev/docs/install)
- [Temporal CLI](https://docs.temporal.io/cli) (`brew install temporal`)
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

## API

| Method | Path |
|--------|------|
| `POST` | `/fees/bills` |
| `POST` | `/fees/bills/:id/line-items` |
| `POST` | `/fees/bills/:id/close` |
| `GET` | `/fees/bills/:id` |
