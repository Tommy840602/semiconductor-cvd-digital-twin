#!/bin/sh
# Only operates the disposable cvd-disposition-test project and maintenance_check databases.
set -eu
cd "$(dirname "$0")/.."
COMPOSE=.runtime/maintenance-compose.json
[ "$(python3 -c 'import json;print(json.load(open(".runtime/maintenance-compose.json"))["name"])')" = cvd-disposition-test ]
for service in postgres timescaledb; do
 docker compose -f "$COMPOSE" exec -T "$service" psql -U cvd -d postgres -v ON_ERROR_STOP=1 -c 'DROP DATABASE IF EXISTS maintenance_check WITH (FORCE)' -c 'CREATE DATABASE maintenance_check' >/dev/null
done
container=$(docker compose -f "$COMPOSE" ps -q backend)
docker cp backend/target/cvd-0.1.0.jar "$container":/tmp/maintenance-check.jar >/dev/null
docker exec "$container" sh -c 'mkdir -p /tmp/disposition-check; cd /tmp/disposition-check; unzip -qo /tmp/maintenance-check.jar'
docker cp backend/target/test-classes "$container":/tmp/disposition-tests >/dev/null
docker exec "$container" java -cp '/tmp/disposition-tests:/tmp/disposition-check/BOOT-INF/classes:/tmp/disposition-check/BOOT-INF/lib/*' com.example.cvd.MaintenanceMigrationCheck
