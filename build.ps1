$ErrorActionPreference = "Stop"
Push-Location $PSScriptRoot

Write-Host "==> 构建 admin SPA"
Push-Location admin
npm run build
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Pop-Location

Write-Host "==> 同步产物到 internal/server/dist"
if (Test-Path internal/server/dist) { Remove-Item -Recurse -Force internal/server/dist }
New-Item -ItemType Directory -Path internal/server/dist -Force | Out-Null
Copy-Item -Path admin/dist/* -Destination internal/server/dist/ -Recurse -Force
New-Item -ItemType File -Path internal/server/dist/.gitkeep -Force | Out-Null

Write-Host "==> 构建二进制"
go build -o dulizhan.exe ./cmd/dulizhan
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "==> 完成: ./dulizhan.exe (配置见 config.yaml)"
Pop-Location
