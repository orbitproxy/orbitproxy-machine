#!/usr/bin/env bash
# 把 CLI 发布版本写入 VERSION。本地 make / publish.sh 都读这个文件。
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:-}"

if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-].*)?$ ]]; then
  echo "usage: $0 <semver>   e.g. $0 1.2.3" >&2
  exit 1
fi

printf '%s\n' "$VERSION" > "$ROOT/VERSION"
echo "orbitproxy-machine VERSION -> $VERSION"
