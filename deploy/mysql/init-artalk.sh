#!/bin/sh
set -eu

case "$MYSQL_ROOT_PASSWORD:$MYSQL_PASSWORD:$ARTALK_DB_PASSWORD" in
  *replace-*|*change-me*)
    echo "MySQL passwords must not use placeholder values" >&2
    exit 1
    ;;
esac
if [ "${#MYSQL_ROOT_PASSWORD}" -lt 32 ] ||
   [ "${#MYSQL_PASSWORD}" -lt 32 ] ||
   [ "${#ARTALK_DB_PASSWORD}" -lt 32 ]; then
  echo "MySQL root, blog and Artalk passwords must contain at least 32 characters" >&2
  exit 1
fi
for password in "$MYSQL_PASSWORD" "$ARTALK_DB_PASSWORD"; do
  case "$password" in
    *[!A-Za-z0-9]*)
      echo "Application MySQL passwords must use only ASCII letters and digits" >&2
      exit 1
      ;;
  esac
done
if [ "$MYSQL_ROOT_PASSWORD" = "$MYSQL_PASSWORD" ] ||
   [ "$MYSQL_ROOT_PASSWORD" = "$ARTALK_DB_PASSWORD" ] ||
   [ "$MYSQL_PASSWORD" = "$ARTALK_DB_PASSWORD" ]; then
  echo "MySQL root, blog and Artalk accounts must use different passwords" >&2
  exit 1
fi

escaped_password=$(printf '%s' "$ARTALK_DB_PASSWORD" | sed -e 's/\\/\\\\/g' -e "s/'/''/g")
mysql --protocol=socket -uroot -p"$MYSQL_ROOT_PASSWORD" <<SQL
CREATE DATABASE IF NOT EXISTS artalk CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'artalk_user'@'%' IDENTIFIED BY '${escaped_password}';
ALTER USER 'artalk_user'@'%' IDENTIFIED BY '${escaped_password}';
GRANT ALL PRIVILEGES ON artalk.* TO 'artalk_user'@'%';
FLUSH PRIVILEGES;
SQL
