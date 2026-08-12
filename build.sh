#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

echo "==> 构建 admin SPA"
(cd admin && npm run build)

echo "==> 同步产物到 internal/server/dist"
rm -rf internal/server/dist
mkdir -p internal/server/dist
cp -r admin/dist/* internal/server/dist/
touch internal/server/dist/.gitkeep

echo "==> 构建二进制"
go build -o dulizhan ./cmd/dulizhan

echo "==> 完成: ./dulizhan (配置见 config.yaml)"
