#!/bin/sh
set -eu

psql \
  -v ON_ERROR_STOP=1 \
  -v app_user="$DB_USER" \
  -v app_password="$DB_PASSWORD" \
  -v db_name="$POSTGRES_DB" \
  --username "$POSTGRES_USER" \
  --dbname "$POSTGRES_DB" <<'SQL'
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

SELECT format('CREATE ROLE %I LOGIN PASSWORD %L', :'app_user', :'app_password')
WHERE NOT EXISTS (
  SELECT 1
  FROM pg_roles
  WHERE rolname = :'app_user'
)
\gexec

SELECT format('ALTER ROLE %I WITH LOGIN PASSWORD %L', :'app_user', :'app_password')
WHERE EXISTS (
  SELECT 1
  FROM pg_roles
  WHERE rolname = :'app_user'
)
\gexec

GRANT CONNECT ON DATABASE :"db_name" TO :"app_user";
GRANT USAGE, CREATE ON SCHEMA public TO :"app_user";
SQL
