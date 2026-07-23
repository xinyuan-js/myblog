#!/bin/sh
set -eu
umask 077

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
env_file=${ENV_FILE:-"$project_dir/.env"}
backup_root=${BACKUP_DIR:-"$project_dir/backups"}
retention_days=${BACKUP_RETENTION_DAYS:-14}

case "$retention_days" in
  ''|*[!0-9]*)
    echo "BACKUP_RETENTION_DAYS must be a non-negative integer" >&2
    exit 1
    ;;
esac

if [ ! -f "$env_file" ]; then
  echo "missing environment file: $env_file" >&2
  exit 1
fi

timestamp=$(date -u +%Y%m%dT%H%M%SZ)
archive="$backup_root/myblog-$timestamp.tar.gz"
mkdir -p "$backup_root"
legacy_lock_dir="$backup_root/.backup.lock"
lock_file="$backup_root/.backup.flock"
lock_mode=

release_lock() {
  if [ "$lock_mode" = "directory" ]; then
    rmdir "$legacy_lock_dir" 2>/dev/null || true
  fi
}

if command -v flock >/dev/null 2>&1; then
  # A kernel lock is released even after SIGKILL or a machine reboot. Keep the
  # inode in place while locked; unlinking it would let a second process lock a
  # newly-created inode concurrently.
  if [ -e "$legacy_lock_dir" ] || [ -L "$legacy_lock_dir" ]; then
    echo "legacy backup lock exists; confirm no old backup is running, then remove: $legacy_lock_dir" >&2
    exit 1
  fi
  if [ -L "$lock_file" ] || { [ -e "$lock_file" ] && [ ! -f "$lock_file" ]; }; then
    echo "backup lock must be a regular non-symlink file: $lock_file" >&2
    exit 1
  fi
  exec 9>"$lock_file"
  if ! flock -n 9; then
    echo "another backup is already running" >&2
    exit 1
  fi
  lock_mode=flock
else
  # Portable fallback for development hosts without flock. A crash can leave
  # this directory behind; refusing manual ambiguity is safer than racing two
  # backups while attempting automatic stale-lock removal.
  if ! mkdir "$legacy_lock_dir" 2>/dev/null; then
    echo "another backup may be running; inspect lock manually: $legacy_lock_dir" >&2
    exit 1
  fi
  lock_mode=directory
fi
if [ -e "$archive" ] || [ -e "$archive.sha256" ]; then
  echo "backup already exists for timestamp: $timestamp" >&2
  release_lock
  exit 1
fi
work_dir=$(mktemp -d "$backup_root/.work-XXXXXX")
archive_tmp=$(mktemp "$backup_root/.archive-XXXXXX")
mkdir -p "$work_dir/databases" "$work_dir/media" "$work_dir/configuration/apps/api" "$work_dir/configuration/apps/web"
archive_verified=0

compose() {
  docker compose --env-file "$env_file" -f "$project_dir/compose.yml" "$@"
}

resume_api=0
resume_artalk=0

wait_healthy() {
  service=$1
  attempts=0
  while [ "$attempts" -lt 30 ]; do
    container=$(compose ps -q "$service")
    if [ -n "$container" ]; then
      health=$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$container" 2>/dev/null || true)
      if [ "$health" = "healthy" ]; then
        return 0
      fi
    fi
    attempts=$((attempts + 1))
    sleep 2
  done
  return 1
}

resume_writers() {
  failed=0
  if [ "$resume_artalk" -eq 1 ]; then
    if compose start artalk >/dev/null && wait_healthy artalk; then
      resume_artalk=0
    else
      failed=1
    fi
  fi
  if [ "$resume_api" -eq 1 ]; then
    if compose start api >/dev/null && wait_healthy api; then
      resume_api=0
    else
      failed=1
    fi
  fi
  return "$failed"
}

cleanup() {
  if ! resume_writers; then
    echo "warning: failed to restart one or more write services; check Docker immediately" >&2
  fi
  if [ "$archive_verified" -ne 1 ]; then
    rm -f -- "$archive" "$archive.sha256"
  fi
  rm -rf -- "$work_dir"
  rm -f -- "$archive_tmp"
  release_lock
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# MySQL and MinIO cannot share a cross-system snapshot. Briefly stop the two
# public writers so database rows and media objects describe the same state.
api_container=$(compose ps -q api)
if [ -z "$api_container" ]; then
  echo "API container does not exist; start the stack before creating a backup" >&2
  exit 1
fi
if [ -n "$api_container" ] && [ "$(docker inspect --format '{{.State.Running}}' "$api_container")" = "true" ]; then
  resume_api=1
  compose stop -t 30 api >/dev/null
fi
artalk_container=$(compose ps -q artalk)
if [ -n "$artalk_container" ] && [ "$(docker inspect --format '{{.State.Running}}' "$artalk_container")" = "true" ]; then
  resume_artalk=1
  compose stop -t 30 artalk >/dev/null
fi

compose exec -T mysql sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysqldump -uroot --single-transaction --routines --events --hex-blob blog' > "$work_dir/databases/blog.sql"
compose exec -T mysql sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysqldump -uroot --single-transaction --routines --events --hex-blob artalk' > "$work_dir/databases/artalk.sql"

minio_container=$(compose ps -q minio)
if [ -z "$minio_container" ]; then
  echo "MinIO container does not exist; start the stack before creating a backup" >&2
  exit 1
fi
minio_image=$(docker inspect --format '{{.Config.Image}}' "$minio_container")
minio_user=$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$api_container" | sed -n 's/^MINIO_ACCESS_KEY=//p')
minio_password=$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$api_container" | sed -n 's/^MINIO_SECRET_KEY=//p')
if [ -z "$minio_user" ] || [ -z "$minio_password" ]; then
  echo "cannot read MinIO application credentials from the API container" >&2
  exit 1
fi
docker run --rm --network "container:$minio_container" --read-only --tmpfs /tmp:size=16m,noexec,nosuid,nodev \
  --cap-drop ALL --security-opt no-new-privileges:true -e MC_CONFIG_DIR=/tmp/mc \
  -e "MINIO_ACCESS_KEY=$minio_user" -e "MINIO_SECRET_KEY=$minio_password" \
  -v "$work_dir/media:/backup" --entrypoint /bin/sh "$minio_image" -c \
  'mc alias set local http://127.0.0.1:9000 "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" >/dev/null && mc mirror --overwrite local/blog-media /backup'

# Never put live credentials into an unencrypted backup archive.  Deployment
# manifests and the environment template are sufficient to reconstruct the
# stack; operators should keep production secrets in a separate encrypted
# password manager or secret backup.
cp "$project_dir/.env.example" "$work_dir/configuration/environment.example"
cp "$project_dir/compose.yml" "$project_dir/compose.https.yml" "$work_dir/configuration/"
cp -R "$project_dir/deploy" "$work_dir/configuration/deploy"
cp "$project_dir/apps/api/Dockerfile" "$work_dir/configuration/apps/api/"
cp "$project_dir/apps/web/Dockerfile" "$work_dir/configuration/apps/web/"
{
  printf 'backup-format=1\n'
  printf 'created-at=%s\n' "$timestamp"
  printf 'databases=blog,artalk\n'
  printf 'media-bucket=blog-media\n'
} > "$work_dir/configuration/backup-manifest.txt"
for service in api web mysql minio artalk; do
  container=$(compose ps -q "$service")
  if [ -n "$container" ]; then
    docker inspect --format '{{.Name}} config-image={{.Config.Image}} image-id={{.Image}}' "$container"
  fi
done > "$work_dir/configuration/container-images.txt"
if command -v git >/dev/null 2>&1 && git -C "$project_dir" rev-parse --verify HEAD > "$work_dir/configuration/git-commit.txt" 2>/dev/null; then
  if [ -n "$(git -C "$project_dir" status --porcelain --untracked-files=normal)" ]; then
    printf '%s\n' 'working-tree-dirty=true' >> "$work_dir/configuration/git-commit.txt"
  fi
fi
tar -C "$work_dir" -czf "$archive_tmp" .
mv "$archive_tmp" "$archive"
archive_name=${archive##*/}
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$backup_root" && sha256sum "$archive_name") > "$archive.sha256"
else
  (cd "$backup_root" && shasum -a 256 "$archive_name") > "$archive.sha256"
fi
"$project_dir/deploy/verify-backup.sh" "$archive" >/dev/null
archive_verified=1

find "$backup_root" -maxdepth 1 -type f \( -name 'myblog-*.tar.gz' -o -name 'myblog-*.tar.gz.sha256' \) -mtime "+$retention_days" -delete
if ! resume_writers; then
  echo "backup created, but failed to restart one or more write services; check Docker immediately" >&2
  exit 1
fi
echo "$archive"
