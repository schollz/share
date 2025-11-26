# Database Configuration

This project now supports both **PostgreSQL** and **SQLite3** databases, with automatic switching based on environment configuration.

## How It Works

The database type is determined by the `DATABASE_URL` environment variable in `.env`:

- **PostgreSQL**: Used when `DATABASE_URL` is set
- **SQLite**: Used when `DATABASE_URL` is not set or empty

## Environment Variables

### PostgreSQL Configuration

When using PostgreSQL, set these variables in `.env`:

```env
DATABASE_URL=postgresql://user:password@host:port/database
DATABASE_NAME=your_database_name
DATABASE_USER=your_username
DATABASE_PASSWORD=your_password
DATABASE_HOST=your_host
DATABASE_PORT=5432
DATABASE_SSLMODE=require
```

### SQLite Configuration

When using SQLite, either:
1. Leave `DATABASE_URL` unset or commented out in `.env`
2. Or don't include it at all

The SQLite database file location is specified via the `--db-path` flag (default: `relay_logs.db`)

## Running Migrations

### Using Make (Recommended)

```bash
# Applies migrations based on DATABASE_URL
make migrate
```

This will:
- Load `.env` file
- Detect if `DATABASE_URL` is set
- Apply PostgreSQL migrations to the configured database, OR
- Apply SQLite migrations to `relay_logs.db`

### Manual Migration

**PostgreSQL:**
```bash
migrate -path migrations/postgres -database "$DATABASE_URL" up
```

**SQLite:**
```bash
migrate -path migrations/sqlite -database "sqlite://relay_logs.db" up
```

## Creating New Migrations

```bash
make migrate:create name=your_migration_name
```

This creates migration files in both `migrations/sqlite/` and `migrations/postgres/` directories.

**Important:** You'll need to customize the SQL for each database type due to syntax differences:
- SQLite uses `INTEGER PRIMARY KEY AUTOINCREMENT`
- PostgreSQL uses `BIGSERIAL PRIMARY KEY`
- SQLite uses `?` for parameters
- PostgreSQL uses `$1, $2, $3` for parameters

## Migration Directory Structure

```
migrations/
├── migrations.go          # Embeds migration files
├── postgres/              # PostgreSQL-specific migrations
│   ├── 0001_init.up.sql
│   ├── 0001_init.down.sql
│   ├── 0002_add_subscriber.up.sql
│   ├── 0002_add_subscriber.down.sql
│   ├── 0003_email_verification.up.sql
│   └── 0003_email_verification.down.sql
└── sqlite/                # SQLite-specific migrations
    ├── 0001_init.up.sql
    ├── 0001_init.down.sql
    ├── 0002_add_subscriber.up.sql
    ├── 0002_add_subscriber.down.sql
    ├── 0003_email_verification.up.sql
    └── 0003_email_verification.down.sql
```

## Starting the Server

The server automatically detects and connects to the appropriate database:

```bash
# With PostgreSQL (DATABASE_URL set in .env)
./e2ecp serve

# With SQLite (DATABASE_URL not set)
./e2ecp serve --db-path relay_logs.db
```

## Switching Between Databases

### Switch to PostgreSQL
1. Uncomment or add `DATABASE_URL` in `.env`
2. Run `make migrate`
3. Start the server

### Switch to SQLite
1. Comment out `DATABASE_URL` in `.env`
2. Run `make migrate` (optional, if you need to set up a new SQLite database)
3. Start the server

## Code Generation (sqlc)

The `sqlc` configuration in `sqlc.yaml` is currently set up for PostgreSQL. To regenerate database code:

```bash
sqlc generate
```

## Dependencies

- **PostgreSQL driver:** `github.com/lib/pq`
- **SQLite driver:** `modernc.org/sqlite`
- **Migrations:** `github.com/golang-migrate/migrate/v4`

Both drivers are included in `go.mod` and compiled into the binary.

## Troubleshooting

### "unknown driver postgresql" error
Reinstall the migrate CLI with postgres support:
```bash
go install -tags 'postgres,sqlite' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

### Migrations not applying
1. Check that `.env` file exists and is properly formatted
2. Verify `DATABASE_URL` is set correctly (for PostgreSQL) or unset (for SQLite)
3. Check database connection and credentials
4. Ensure migration files exist in the correct subdirectory

### Connection issues
- PostgreSQL: Verify host, port, credentials, and SSL settings
- SQLite: Ensure directory permissions allow file creation
