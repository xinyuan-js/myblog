#!/bin/sh
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
env_file=${ENV_FILE:-"$project_dir/.env"}

if [ ! -f "$env_file" ]; then
  echo "missing environment file: $env_file" >&2
  exit 1
fi

docker compose --env-file "$env_file" \
  -f "$project_dir/compose.yml" \
  -f "$project_dir/compose.https.yml" \
  config --quiet

docker compose --env-file "$env_file" \
  -f "$project_dir/compose.yml" \
  -f "$project_dir/compose.https.yml" \
  up -d --no-deps --no-build --force-recreate web
