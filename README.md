# Afrigoals API

Go + Fiber + GORM API backing the Afrigoals football league platform. Serves the
Next.js frontend and brokers match video between the browser and the analysis
model server.

## Requirements

- Go 1.24+
- PostgreSQL 13+ (`gen_random_uuid()` must be available; on PostgreSQL 12 and
  earlier run `CREATE EXTENSION pgcrypto;` first)
- `ffmpeg` on `PATH` — required for video transcoding and clip cutting
- [`golang-migrate`](https://github.com/golang-migrate/migrate) for schema changes:
  `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`

## Running locally

```bash
cp .env.example .env     # then fill in the required values
createdb afrigoals
migrate -path migrations -database "$DATABASE_URL" up
go run main.go           # listens on :6767
```

The server builds its connection string from the individual `DB_*` variables in
`.env`; `golang-migrate` takes a single URL. Keep them pointed at the same
database:

```bash
# must match DB_NAME / DB_USER / DB_HOST / DB_PORT in .env
export DATABASE_URL="postgres://postgres:password@localhost:5432/afrigoals?sslmode=disable"
```

If they disagree, migrations apply to one database and the server starts against
another, and the first symptom is `relation "users" does not exist` at boot.

The server refuses to start unless `JWT_SECRET_KEY`, `ADMIN_EMAIL` and
`ADMIN_PASSWORD` are set. That is deliberate: booting with a default signing key
or an unusable administrator account is worse than not booting at all.

## Database migrations

The schema is owned by `migrations/`. `000001_initial_schema` is the baseline,
generated with `pg_dump` from the schema GORM produces, so it matches what the
models expect rather than a hand-written approximation.

```bash
migrate -path migrations -database "$DATABASE_URL" up        # apply
migrate -path migrations -database "$DATABASE_URL" down 1    # roll back one
migrate -path migrations -database "$DATABASE_URL" version   # current version
migrate create -ext sql -dir migrations -seq add_something   # new migration
```

Rules:

1. Migrations are append-only. To change something, add a new migration; never
   edit one that has been applied anywhere.
2. Every `.up.sql` needs a matching `.down.sql`.
3. Test both directions locally before opening a pull request.
4. Adding a `NOT NULL` column to a populated table needs a `DEFAULT` or a
   backfill step.
5. Write files without a UTF-8 BOM. `golang-migrate` fails to parse one.

### AutoMigrate

`AutoMigrate` still exists but only runs when `AUTO_MIGRATE=true`, and it is
intended for local development. It adds columns but never drops or renames them,
cannot backfill data, and leaves no record of what changed. Do not enable it
against a deployed database.

### Adopting migrations on a database that already has tables

The baseline uses plain `CREATE TABLE`, so running `migrate up` against a
database that predates migrations fails on the first statement with
`relation already exists` and leaves `schema_migrations.dirty = true`.

Tell `migrate` the baseline is already applied instead of running it:

```bash
migrate -path migrations -database "$DATABASE_URL" force 1
```

`force` records the version without executing anything. Use it only for this
baselining step. If a run has already gone dirty, `force 1` also clears the flag.

Such databases may also differ from the baseline, because several video tables
were created by hand. In particular `analysis_events` may still have a
`NOT NULL` constraint on `video_id` and no `created_by` column. `AutoMigrate`
will not relax an existing constraint, so repair those manually:

```sql
ALTER TABLE analysis_events ALTER COLUMN video_id DROP NOT NULL;
ALTER TABLE analysis_events ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
```

### Known follow-up

The baseline reproduces GORM's output faithfully, which means several tables have
no foreign key constraints because the models never declared them. Adding those
constraints is worth doing, but it needs a separate migration and a check that
existing rows do not already violate them.

## Layout

```
main.go        entry point
database/      connection, migrations bootstrap, admin seeding
models/        GORM models (one per table)
middleware/    auth, roles, CORS
routes/        URL to handler mapping
services/      HTTP handlers
migrations/    SQL schema migrations
```

Routes registered above `api.Use(middleware.AuthMiddleware())` in
`routes/routes.go` are public; everything below it requires a valid token. That
ordering is positional, so check where you are adding an endpoint.
