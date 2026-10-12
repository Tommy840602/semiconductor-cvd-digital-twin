#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
COMPOSE=.runtime/utilization-compose.json
[ "$(python3 -c 'import json;print(json.load(open(".runtime/utilization-compose.json"))["name"])')" = cvd-disposition-test ]
for service in postgres timescaledb; do
 docker compose -f "$COMPOSE" exec -T "$service" psql -U cvd -d postgres -v ON_ERROR_STOP=1 -c 'DROP DATABASE IF EXISTS utilization_check WITH (FORCE)' -c 'CREATE DATABASE utilization_check' >/dev/null
done
container=$(docker compose -f "$COMPOSE" ps -q backend)
docker exec "$container" sh -c 'mkdir -p /tmp/utilization-check; cd /tmp/utilization-check; unzip -qo /app.jar'
docker cp backend/target/test-classes "$container":/tmp/utilization-tests >/dev/null
docker exec "$container" java -cp '/tmp/utilization-tests:/tmp/utilization-check/BOOT-INF/classes:/tmp/utilization-check/BOOT-INF/lib/*' com.example.cvd.UtilizationDatabaseCheck
