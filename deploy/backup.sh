#!/bin/sh
set -eu
umask 077

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
env_file=${ENV_FILE:-"$project_dir/.env"}
backup_root=${BACKUP_DIR:-"$project_dir/backups"}
retention_days=${BACKUP_RETENTION_DAYS:-14}

if [ ! -f "$env_file" ]; then
  echo "missing environment file: $env_file" >&2
  exit 1
fi

timestamp=$(date -u +%Y%m%dT%H%M%SZ)
work_dir="$backup_root/.work-$timestamp"
archive="$backup_root/myblog-$timestamp.tar.gz"
mkdir -p "$work_dir/databases" "$work_dir/media" "$work_dir/configuration/deploy/nginx" "$work_dir/configuration/deploy/mysql"

cleanup() {
  rm -rf -- "$work_dir"
}
trap cleanup EXIT INT TERM

compose() {
  docker compose --env-file "$env_file" -f "$project_dir/compose.yml" "$@"
}

compose exec -T mysql sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysqldump -uroot --single-transaction --routines --events --hex-blob blog' > "$work_dir/databases/blog.sql"
compose exec -T mysql sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysqldump -uroot --single-transaction --routines --events --hex-blob artalk' > "$work_dir/databases/artalk.sql"

minio_container=$(compose ps -q minio)
minio_image=$(docker inspect --format '{{.Config.Image}}' "$minio_container")
minio_user=$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$minio_container" | sed -n 's/^MINIO_ROOT_USER=//p')
minio_password=$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$minio_container" | sed -n 's/^MINIO_ROOT_PASSWORD=//p')
if [ -z "$minio_user" ] || [ -z "$minio_password" ]; then
  echo "cannot read MinIO backup credentials from the running container" >&2
  exit 1
fi
docker run --rm --network "container:$minio_container" --read-only --tmpfs /tmp:size=16m,noexec,nosuid,nodev \
  --cap-drop ALL --security-opt no-new-privileges:true -e MC_CONFIG_DIR=/tmp/mc \
  -e "MINIO_ROOT_USER=$minio_user" -e "MINIO_ROOT_PASSWORD=$minio_password" \
  -v "$work_dir/media:/backup" --entrypoint /bin/sh "$minio_image" -c \
  'mc alias set local http://127.0.0.1:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null && mc mirror --overwrite local/blog-media /backup'

# Never put live credentials into an unencrypted backup archive.  Deployment
# manifests and the environment template are sufficient to reconstruct the
# stack; operators should keep production secrets in a separate encrypted
# password manager or secret backup.
cp "$project_dir/.env.example" "$work_dir/configuration/environment.example"
cp "$project_dir/compose.yml" "$project_dir/compose.https.yml" "$work_dir/configuration/"
cp "$project_dir/deploy/nginx/default.conf" "$project_dir/deploy/nginx/https.conf" "$work_dir/configuration/deploy/nginx/"
cp "$project_dir/deploy/mysql/init-artalk.sh" "$work_dir/configuration/deploy/mysql/"
tar -C "$work_dir" -czf "$archive" .
if command -v sha256sum >/dev/null 2>&1; then sha256sum "$archive" > "$archive.sha256"; else shasum -a 256 "$archive" > "$archive.sha256"; fi

find "$backup_root" -type f \( -name 'myblog-*.tar.gz' -o -name 'myblog-*.tar.gz.sha256' \) -mtime "+$retention_days" -delete
echo "$archive"
