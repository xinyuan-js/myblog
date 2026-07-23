#!/bin/sh
set -eu

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$project_dir"

fail=0
report_matches() {
  label=$1
  matches=$2
  if [ -n "$matches" ]; then
    echo "$label" >&2
    printf '%s\n' "$matches" >&2
    fail=1
  fi
}

tracked_env=$(git ls-files | grep -E '(^|/)\.env($|\.)' | grep -Ev '(^|/)\.env\.example$' || true)
report_matches "tracked private environment files:" "$tracked_env"

tracked_credentials=$(git ls-files | grep -Ei '\.(pem|key|p12|pfx|jks|keystore)$' || true)
report_matches "tracked credential or private-key files:" "$tracked_credentials"

tracked_runtime=$(git ls-files | grep -E '^(data|backups)/|(^|/)(node_modules|dist|coverage|\.cache)/' || true)
report_matches "tracked runtime, dependency or generated directories:" "$tracked_runtime"

tracked_binaries=$(git ls-files | grep -Ei '(^|/)(server|blog-api)(\.exe)?$|\.(test|prof|coverprofile|sqlite|db)$' || true)
report_matches "tracked binary, profile or database artifacts:" "$tracked_binaries"

secret_markers=$(git grep -n -I -E \
  'BEGIN (RSA |EC |DSA |OPENSSH )?PRIVATE KEY|github_pat_[A-Za-z0-9_]{20,}|gh[pousr]_[A-Za-z0-9_]{20,}' \
  -- . ':(exclude)deploy/check-repository.sh' || true)
report_matches "tracked files contain a private-key or access-token marker:" "$secret_markers"

if [ "$fail" -ne 0 ]; then
  exit 1
fi

echo "repository boundary check passed"
