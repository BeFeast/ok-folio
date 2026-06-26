#!/bin/sh
set -eu

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<'SQL'
CREATE EXTENSION IF NOT EXISTS vector;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM pg_available_extensions
    WHERE name = 'vchord'
  ) THEN
    EXECUTE 'CREATE EXTENSION IF NOT EXISTS vchord CASCADE';
  END IF;
END
$$;
SQL
