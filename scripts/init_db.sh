#!/usr/bin/env bash

set -x
set -eo pipefail

# DEPENDENCY
# ==========
# if ! [ -x "$(command -v sqlx)" ]; then
#     echo >&2 "Error: sqlx is not installed."
#     echo >&2 "Use:"
#     echo >&2 "    cargo install --version=0.5.7 sqlx-cli --no-default-features --features postgres"
#     exit 1
# fi
# ==========

# POSTGRES
# ========
# Check if a custom user has been set, else default to 'postgres'
DB_USER="${POSTGRES_USER:=greenlight}"
# Check if a custom password has been set, else default to 'password'
DB_PASSWORD="${POSTGRES_PASSWORD:=pa55word}"
# Check if a custom database name has been set, else default to 'newsletter'
DB_NAME="${POSTGRES_DB:=greenlight}"
# Check if a custom host has been set, else default to 'localhost'
DB_HOST="${POSTGRES_HOST:=localhost}"
# Check if a custom host has been set, else default to '5432'
DB_PORT="${POSTGRES_PORT:=5432}"
# ========

if [[ -z "${SKIP_DOCKER}" ]]
then
    RUNNING_POSTGRES_CONTAINER=$(docker ps --filter 'name=postgres' --format '{{.ID}}')
    if [[ -n $RUNNING_POSTGRES_CONTAINER ]]; then
        echo >&2 "there is a postgres container already running, kill it with"
        echo >&2 "    docker kill ${RUNNING_POSTGRES_CONTAINER}"
        exit 1
    fi
    docker run \
        -e POSTGRES_USER=${DB_USER} \
        -e POSTGRES_PASSWORD=${DB_PASSWORD} \
        -p "${DB_PORT}":5432 \
        -d \
        --name "postgres_$(date '+%s')" \
        postgres -N 1000
    # ^ Increase maximum number of connections for testing purposes
fi

# Keep pinging Postgres until it's ready to accept commands
until PGPASSWORD="${DB_PASSWORD}" psql -h "${DB_HOST}" -U "${DB_USER}" -p "${DB_PORT}" -d "postgres" -c '\q'; do
    >&2 echo "Postgres is still unavailable - sleeping"
    sleep 1
done

>&2 echo "Postgres is up and running on port ${DB_PORT} - running migrations now!"


export DATABASE_URL=postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}

>&2 echo "Postgres has been migrated, ready to go!"
