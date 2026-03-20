#!/bin/bash
set -e

# This script is used as a Docker entrypoint init script for PostgreSQL.
# It creates separate databases for Ory Kratos and Ory Hydra alongside
# the main skillbox database.

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE DATABASE skillbox_kratos;
    CREATE DATABASE skillbox_hydra;
    GRANT ALL PRIVILEGES ON DATABASE skillbox_kratos TO $POSTGRES_USER;
    GRANT ALL PRIVILEGES ON DATABASE skillbox_hydra TO $POSTGRES_USER;
EOSQL

echo "Ory databases created: skillbox_kratos, skillbox_hydra"
