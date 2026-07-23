#!/bin/sh
set -eu
umask 077

case "$MYSQL_ROOT_PASSWORD:$ARTALK_BRIDGE_DB_PASSWORD" in
  *replace-*|*change-me*)
    echo "Artalk bridge credentials must not use placeholder values" >&2
    exit 1
    ;;
esac
if [ "${#MYSQL_ROOT_PASSWORD}" -lt 32 ] || [ "${#ARTALK_BRIDGE_DB_PASSWORD}" -lt 32 ]; then
  echo "MySQL root and Artalk bridge passwords must contain at least 32 characters" >&2
  exit 1
fi
case "$ARTALK_BRIDGE_DB_PASSWORD" in
  *[!A-Za-z0-9]*)
    echo "The Artalk bridge password must use only ASCII letters and digits" >&2
    exit 1
    ;;
esac

# Artalk creates its users table on first startup, so this grant cannot run in
# MySQL's initial /docker-entrypoint-initdb.d phase. The one-shot Compose
# service mounts MySQL's private socket and runs only after Artalk is healthy.
MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql \
  --protocol=socket \
  --socket=/var/run/mysqld/mysqld.sock \
  -uroot <<SQL
CREATE USER IF NOT EXISTS 'blog_artalk_bridge'@'%' IDENTIFIED BY '${ARTALK_BRIDGE_DB_PASSWORD}';
ALTER USER 'blog_artalk_bridge'@'%' IDENTIFIED BY '${ARTALK_BRIDGE_DB_PASSWORD}';
REVOKE ALL PRIVILEGES, GRANT OPTION FROM 'blog_artalk_bridge'@'%';
GRANT SELECT, UPDATE ON artalk.users TO 'blog_artalk_bridge'@'%';
FLUSH PRIVILEGES;
SQL
