#!/bin/sh
set -eu

escaped_password=$(printf '%s' "$ARTALK_DB_PASSWORD" | sed "s/'/''/g")
mysql --protocol=socket -uroot -p"$MYSQL_ROOT_PASSWORD" <<SQL
CREATE DATABASE IF NOT EXISTS artalk CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'artalk_user'@'%' IDENTIFIED BY '${escaped_password}';
ALTER USER 'artalk_user'@'%' IDENTIFIED BY '${escaped_password}';
GRANT ALL PRIVILEGES ON artalk.* TO 'artalk_user'@'%';
FLUSH PRIVILEGES;
SQL
