#!/bin/sh
set -eu
umask 077

case "$MINIO_ROOT_USER:$MINIO_ROOT_PASSWORD:$MINIO_ACCESS_KEY:$MINIO_SECRET_KEY" in
  *replace-*|*change-me*|*minioadmin*)
    echo "MinIO credentials must not use placeholder or default values" >&2
    exit 1
    ;;
esac
if [ "${#MINIO_ROOT_PASSWORD}" -lt 32 ] || [ "${#MINIO_SECRET_KEY}" -lt 32 ]; then
  echo "MinIO root and application secrets must contain at least 32 characters" >&2
  exit 1
fi
if [ "$MINIO_ROOT_USER" = "$MINIO_ACCESS_KEY" ] ||
   [ "$MINIO_ROOT_PASSWORD" = "$MINIO_SECRET_KEY" ]; then
  echo "MinIO root and application credentials must be different" >&2
  exit 1
fi

mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null
mc mb --ignore-existing "local/$MINIO_BUCKET" >/dev/null

public_policy_file=/tmp/myblog-public-read-policy.json
{
  printf '%s\n' '{"Version":"2012-10-17","Statement":['
  printf '%s\n' "{\"Effect\":\"Allow\",\"Principal\":{\"AWS\":[\"*\"]},\"Action\":[\"s3:GetObject\"],\"Resource\":[\"arn:aws:s3:::$MINIO_BUCKET/*\"]}"
  printf '%s\n' ']}'
} > "$public_policy_file"
mc anonymous set-json "$public_policy_file" "local/$MINIO_BUCKET" >/dev/null

policy_file=/tmp/myblog-app-policy.json
{
  printf '%s\n' '{"Version":"2012-10-17","Statement":['
  printf '%s\n' "{\"Effect\":\"Allow\",\"Action\":[\"s3:GetBucketLocation\",\"s3:ListBucket\"],\"Resource\":[\"arn:aws:s3:::$MINIO_BUCKET\"]},"
  printf '%s\n' "{\"Effect\":\"Allow\",\"Action\":[\"s3:GetObject\",\"s3:PutObject\",\"s3:DeleteObject\"],\"Resource\":[\"arn:aws:s3:::$MINIO_BUCKET/*\"]}"
  printf '%s\n' ']}'
} > "$policy_file"

mc admin user add local "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" >/dev/null
mc admin policy create local myblog-app "$policy_file" >/dev/null
mc admin policy attach local myblog-app --user "$MINIO_ACCESS_KEY" >/dev/null
