<#
.SYNOPSIS
    Build the Windows client MSI, publish it into the server update package,
    rebuild the embedded server web UI, and compile the Linux server binary.

.EXAMPLE
    .\build-release.ps1
    .\build-release.ps1 -Version 0.5.1
#>
param(
    [string]$Version,
    [switch]$SkipClientUI,
    [switch]$SkipServerUI
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$Root = $PSScriptRoot

$clientArgs = @{}
if ($Version) { $clientArgs["Version"] = $Version }
if ($SkipClientUI) { $clientArgs["SkipUI"] = $true }

Write-Host ""
Write-Host "==> Building Windows client and publishing update package" -ForegroundColor Cyan
& "$Root\client\build.ps1" @clientArgs
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host ""
Write-Host "==> Building server Web UI" -ForegroundColor Cyan
if (-not $SkipServerUI) {
    Push-Location "$Root\server\webui"
    npm install --prefer-offline --no-audit --no-fund
    if ($LASTEXITCODE -ne 0) { Pop-Location; exit $LASTEXITCODE }
    npm run build
    if ($LASTEXITCODE -ne 0) { Pop-Location; exit $LASTEXITCODE }
    Pop-Location

    Remove-Item "$Root\server\internal\api\ui\dist" -Recurse -Force -ErrorAction SilentlyContinue
    Copy-Item "$Root\server\webui\dist" "$Root\server\internal\api\ui\dist" -Recurse -Force
} else {
    Write-Host "    skipped (-SkipServerUI)" -ForegroundColor DarkGray
}

Write-Host ""
Write-Host "==> Building Linux server binary" -ForegroundColor Cyan
Push-Location "$Root\server"
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -o "bin\proidentity" .\cmd\server
$rc = $LASTEXITCODE
Remove-Item Env:\GOOS, Env:\GOARCH -ErrorAction SilentlyContinue
Pop-Location
if ($rc -ne 0) { exit $rc }

Write-Host ""
Write-Host "Release build complete" -ForegroundColor Green
Write-Host "  Client MSI:     $Root\client\build"
Write-Host "  Server binary:  $Root\server\bin\proidentity"
Write-Host "  Update package: $Root\server\internal\api\client_updates\windows"
Write-Host ""
