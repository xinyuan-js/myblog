#!/bin/sh
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

sh -n \
  "$project_dir/deploy/backup.sh" \
  "$project_dir/deploy/verify-backup.sh" \
  "$project_dir/deploy/check-repository.sh" \
  "$project_dir/deploy/check-secrets.sh" \
  "$project_dir/deploy/reload-web-certificate.sh" \
  "$project_dir/deploy/mysql/init-artalk.sh" \
  "$project_dir/deploy/mysql/init-artalk-bridge.sh" \
  "$project_dir/deploy/minio/init.sh"

run_secret_check() {
  APP_ENV=$1 \
  APP_ORIGIN=$2 \
  NGINX_SERVER_NAME=$3 \
  SESSION_COOKIE_SECURE=$4 \
  MYSQL_ROOT_PASSWORD=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa \
  BLOG_DB_PASSWORD=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb \
  ARTALK_DB_PASSWORD=cccccccccccccccccccccccccccccccc \
  ARTALK_BRIDGE_DB_PASSWORD=dddddddddddddddddddddddddddddddd \
  MINIO_ROOT_USER=test-root \
  MINIO_ROOT_PASSWORD=eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee \
  MINIO_ACCESS_KEY=test-app \
  MINIO_SECRET_KEY=ffffffffffffffffffffffffffffffff \
  ARTALK_APP_KEY=gggggggggggggggggggggggggggggggg \
  "$project_dir/deploy/check-secrets.sh"
}

run_secret_check production https://blog.example.com blog.example.com true
run_secret_check development http://127.0.0.1:8088 localhost false
if run_secret_check production https://wrong.example.com blog.example.com true >/dev/null 2>&1; then
  echo "production origin mismatch was accepted" >&2
  exit 1
fi

fixture_dir=$(mktemp -d "${TMPDIR:-/tmp}/myblog-script-test-XXXXXX")
cleanup() {
  rm -rf -- "$fixture_dir"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

source_dir="$fixture_dir/source"
mkdir -p "$source_dir/databases" "$source_dir/media" "$source_dir/configuration"
printf '%s\n' "-- blog dump" > "$source_dir/databases/blog.sql"
printf '%s\n' "-- artalk dump" > "$source_dir/databases/artalk.sql"
for file in environment.example compose.yml compose.https.yml backup-manifest.txt container-images.txt; do
  printf '%s\n' "fixture" > "$source_dir/configuration/$file"
done

archive="$fixture_dir/myblog-test.tar.gz"
tar -C "$source_dir" -czf "$archive" .
archive_name=${archive##*/}
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$fixture_dir" && sha256sum "$archive_name") > "$archive.sha256"
else
  (cd "$fixture_dir" && shasum -a 256 "$archive_name") > "$archive.sha256"
fi
"$project_dir/deploy/verify-backup.sh" "$archive" >/dev/null

printf '%064d  %s\n' 0 "$archive_name" > "$archive.sha256"
if "$project_dir/deploy/verify-backup.sh" "$archive" >/dev/null 2>&1; then
  echo "tampered backup checksum was accepted" >&2
  exit 1
fi

echo "deployment script tests passed"
