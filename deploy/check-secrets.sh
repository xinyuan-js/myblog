#!/bin/sh
set -eu

case "$NGINX_SERVER_NAME" in
  ""|*[!a-z0-9.-]*|.*|*.|*..*|-*|*-)
    echo "NGINX_SERVER_NAME must be a lowercase DNS hostname without a scheme, port or path" >&2
    exit 1
    ;;
esac

if [ "$APP_ENV" = "production" ]; then
  if [ "$APP_ORIGIN" != "https://$NGINX_SERVER_NAME" ]; then
    echo "production APP_ORIGIN must exactly match https://NGINX_SERVER_NAME" >&2
    exit 1
  fi
  if [ "$SESSION_COOKIE_SECURE" != "true" ]; then
    echo "production SESSION_COOKIE_SECURE must be true" >&2
    exit 1
  fi
fi

case "$MYSQL_ROOT_PASSWORD:$BLOG_DB_PASSWORD:$ARTALK_DB_PASSWORD:$ARTALK_BRIDGE_DB_PASSWORD:$MINIO_ROOT_USER:$MINIO_ROOT_PASSWORD:$MINIO_ACCESS_KEY:$MINIO_SECRET_KEY:$ARTALK_APP_KEY" in
  *replace-*|*change-me*|*minioadmin*)
    echo "deployment credentials must not use placeholder or default values" >&2
    exit 1
    ;;
esac

for password in "$MYSQL_ROOT_PASSWORD" "$BLOG_DB_PASSWORD" "$ARTALK_DB_PASSWORD" "$ARTALK_BRIDGE_DB_PASSWORD" "$MINIO_ROOT_PASSWORD" "$MINIO_SECRET_KEY" "$ARTALK_APP_KEY"; do
  if [ "${#password}" -lt 32 ]; then
    echo "database, MinIO and Artalk secrets must contain at least 32 characters" >&2
    exit 1
  fi
done

for password in "$BLOG_DB_PASSWORD" "$ARTALK_DB_PASSWORD" "$ARTALK_BRIDGE_DB_PASSWORD"; do
  case "$password" in
    *[!A-Za-z0-9]*)
      echo "application MySQL passwords must use only ASCII letters and digits" >&2
      exit 1
      ;;
  esac
done

if [ "$MYSQL_ROOT_PASSWORD" = "$BLOG_DB_PASSWORD" ] ||
   [ "$MYSQL_ROOT_PASSWORD" = "$ARTALK_DB_PASSWORD" ] ||
   [ "$MYSQL_ROOT_PASSWORD" = "$ARTALK_BRIDGE_DB_PASSWORD" ] ||
   [ "$BLOG_DB_PASSWORD" = "$ARTALK_DB_PASSWORD" ] ||
   [ "$BLOG_DB_PASSWORD" = "$ARTALK_BRIDGE_DB_PASSWORD" ] ||
   [ "$ARTALK_DB_PASSWORD" = "$ARTALK_BRIDGE_DB_PASSWORD" ]; then
  echo "MySQL root, blog, Artalk and bridge accounts must use different passwords" >&2
  exit 1
fi

if [ "$MINIO_ROOT_USER" = "$MINIO_ACCESS_KEY" ] ||
   [ "$MINIO_ROOT_PASSWORD" = "$MINIO_SECRET_KEY" ]; then
  echo "MinIO root and application credentials must be different" >&2
  exit 1
fi
