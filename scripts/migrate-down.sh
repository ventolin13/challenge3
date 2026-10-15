#!/bin/sh
set -eu

docker compose exec -T postgres psql -U challenge -d challenge -v ON_ERROR_STOP=1 < migrations/down/all.sql
echo "dropped all tables"
