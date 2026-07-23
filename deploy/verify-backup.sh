#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: $0 /path/to/myblog-TIMESTAMP.tar.gz" >&2
  exit 2
fi

archive=$1
checksum_file="$archive.sha256"
case "$archive" in
  *.tar.gz) ;;
  *)
    echo "backup archive must end with .tar.gz" >&2
    exit 1
    ;;
esac
if [ ! -f "$archive" ] || [ -L "$archive" ]; then
  echo "backup archive must be a regular non-symlink file: $archive" >&2
  exit 1
fi
if [ ! -f "$checksum_file" ] || [ -L "$checksum_file" ]; then
  echo "missing regular checksum file: $checksum_file" >&2
  exit 1
fi

archive_name=${archive##*/}
checksum_lines=$(wc -l < "$checksum_file" | tr -d ' ')
expected_checksum=
expected_name=
unexpected_checksum_field=
IFS=' ' read -r expected_checksum expected_name unexpected_checksum_field < "$checksum_file" || true
case "$expected_checksum" in
  *[!0-9A-Fa-f]*|"")
    echo "checksum file contains an invalid SHA-256 digest" >&2
    exit 1
    ;;
esac
if [ "${#expected_checksum}" -ne 64 ] || [ "$checksum_lines" -ne 1 ] ||
   [ "$expected_name" != "$archive_name" ] || [ -n "$unexpected_checksum_field" ]; then
  echo "checksum file must contain exactly one digest for $archive_name" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  actual_checksum=$(sha256sum "$archive" | sed 's/[[:space:]].*$//')
else
  actual_checksum=$(shasum -a 256 "$archive" | sed 's/[[:space:]].*$//')
fi
if [ "$actual_checksum" != "$expected_checksum" ]; then
  echo "backup checksum does not match" >&2
  exit 1
fi

verify_dir=$(mktemp -d "${TMPDIR:-/tmp}/myblog-verify-XXXXXX")
cleanup() {
  rm -rf -- "$verify_dir"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

entries_file="$verify_dir/entries.txt"
verbose_file="$verify_dir/verbose.txt"
tar -tzf "$archive" > "$entries_file"
tar -tvzf "$archive" > "$verbose_file"

while IFS= read -r entry; do
  case "$entry" in
    .|./|./databases|./databases/|./media|./media/|./configuration|./configuration/|./databases/*|./media/*|./configuration/*) ;;
    *)
      echo "backup contains an unexpected path: $entry" >&2
      exit 1
      ;;
  esac
  case "$entry" in
    /*|../*|*/../*|*/..|*//*)
      echo "backup contains an unsafe path: $entry" >&2
      exit 1
      ;;
  esac
done < "$entries_file"

while IFS= read -r listing; do
  entry_type=$(printf '%s' "$listing" | cut -c1)
  case "$entry_type" in
    -|d) ;;
    *)
      echo "backup contains a link or unsupported archive entry" >&2
      exit 1
      ;;
  esac
done < "$verbose_file"

for required in \
  ./databases/blog.sql \
  ./databases/artalk.sql \
  ./configuration/environment.example \
  ./configuration/compose.yml \
  ./configuration/compose.https.yml \
  ./configuration/backup-manifest.txt \
  ./configuration/container-images.txt
do
  if ! grep -Fqx "$required" "$entries_file"; then
    echo "backup is missing required entry: $required" >&2
    exit 1
  fi
done

mkdir "$verify_dir/extracted"
tar -xzf "$archive" -C "$verify_dir/extracted"
for database_dump in "$verify_dir/extracted/databases/blog.sql" "$verify_dir/extracted/databases/artalk.sql"; do
  if [ ! -s "$database_dump" ]; then
    echo "backup contains an empty database dump: ${database_dump##*/}" >&2
    exit 1
  fi
done

echo "backup verified: $archive"
