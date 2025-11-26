# Database Configuration (PostgreSQL Only)

The relay now uses **PostgreSQL exclusively**. SQLite support and migrations have been removed.

## Environment

Set a connection string in `DATABASE_URL` (or pass `--db-url` to `serve`):

```env
DATABASE_URL=postgresql://user:password@host:5432/database?sslmode=require
```

## Migrations

Apply migrations with the Make target (loads `.env` automatically):

```bash
make migrate
```

Manual run:

```bash
migrate -path migrations/postgres -database "$DATABASE_URL" up
```

Create a new migration:

```bash
make migrate:create name=add_table
```

This adds timestamped files under `migrations/postgres/`.

## Starting the Server

```bash
# Uses DATABASE_URL from the environment if --db-url is omitted
./e2ecp serve --db-url "$DATABASE_URL"
```

If no URL is provided, session logging is disabled.

## Dependencies

- PostgreSQL driver: `github.com/lib/pq`
- Migrations: `github.com/golang-migrate/migrate/v4` (install CLI with `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`)
