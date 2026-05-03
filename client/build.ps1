<#
.SYNOPSIS
    Full build: UI -> App -> Daemon -> MSI installer

.PARAMETER Version
    Version string (default: read from wails.json)

.PARAMETER SkipUI
    Skip npm install + frontend build (reuse existing frontend/dist)

.EXAMPLE
    .\build.ps1
    .\build.ps1 -Version 0.2.0
    .\build.ps1 -SkipUI

.NOTES
    Prerequisites:
      go    1.22+   https://go.dev/dl
      node  18+     https://nodejs.org
      wails v2      go install github.com/wailsapp/wails/v2/cmd/wails@latest
      wix   v4      dotnet tool install --global wix --version "4.*"
                    wix extension add WixToolset.UI.wixext --global
#>
param(
    [string]$Version,
    [switch]$SkipUI,
    [switch]$SkipServerPackage
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$Root   = $PSScriptRoot
$BinDir = "$Root\build\bin"

# ---------------------------------------------------------------------------
# Version
# ---------------------------------------------------------------------------
if (-not $Version) {
    $Version = (Get-Content "$Root\wails.json" -Raw | ConvertFrom-Json).info.productVersion
}

Write-Host ""
Write-Host "  ProIdentity Access  v$Version" -ForegroundColor Cyan
Write-Host ""

# ---------------------------------------------------------------------------
# Prerequisites
# ---------------------------------------------------------------------------
$missing = $false
foreach ($t in @(
    @{ name="go";    hint="https://go.dev/dl" },
    @{ name="node";  hint="https://nodejs.org" },
    @{ name="npm";   hint="https://nodejs.org" },
    @{ name="wails"; hint="go install github.com/wailsapp/wails/v2/cmd/wails@latest" },
    @{ name="wix";   hint='dotnet tool install --global wix --version "4.*"' }
)) {
    if (-not (Get-Command $t.name -ErrorAction SilentlyContinue)) {
        Write-Host "  [MISSING] $($t.name)  ->  $($t.hint)" -ForegroundColor Red
        $missing = $true
    }
}
if ($missing) { exit 1 }

$extOut = wix extension list --global 2>&1
if ($LASTEXITCODE -ne 0 -or ($extOut -notmatch "WixToolset\.UI")) {
    Write-Host "  Installing WixToolset.UI.wixext..." -ForegroundColor Yellow
    wix extension add "WixToolset.UI.wixext/4.0.6" --global
    if ($LASTEXITCODE -ne 0) { exit 1 }
}
if ($extOut -notmatch "WixToolset\.Util") {
    Write-Host "  Installing WixToolset.Util.wixext..." -ForegroundColor Yellow
    wix extension add "WixToolset.Util.wixext/4.0.6" --global
    if ($LASTEXITCODE -ne 0) { exit 1 }
}

# ---------------------------------------------------------------------------
# Step 0 -- Wintun DLL (embedded into daemon at compile time)
# ---------------------------------------------------------------------------
$wintunDll = "$Root\internal\daemon\wintun_amd64.dll"
if (-not (Test-Path $wintunDll)) {
    Write-Host "[0/4] Wintun -- downloading wintun-0.14.1.zip" -ForegroundColor Yellow
    $wintunZip = "$env:TEMP\wintun-0.14.1.zip"
    Invoke-WebRequest -Uri "https://www.wintun.net/builds/wintun-0.14.1.zip" -OutFile $wintunZip
    if ($LASTEXITCODE -ne 0) { exit 1 }
    Expand-Archive -Path $wintunZip -DestinationPath "$env:TEMP\wintun-extract" -Force
    Copy-Item "$env:TEMP\wintun-extract\wintun\bin\amd64\wintun.dll" $wintunDll
    Remove-Item $wintunZip, "$env:TEMP\wintun-extract" -Recurse -Force
    Write-Host "[0/4] Wintun -- saved to $wintunDll" -ForegroundColor Green
} else {
    Write-Host "[0/4] Wintun -- already present, skipping" -ForegroundColor DarkGray
}

# ---------------------------------------------------------------------------
# Step 1 -- Frontend (React + TypeScript)
# ---------------------------------------------------------------------------
if ($SkipUI) {
    Write-Host "[1/4] Frontend -- skipped (-SkipUI)" -ForegroundColor DarkGray
} else {
    Write-Host "[1/4] Frontend -- npm install" -ForegroundColor Green
    Push-Location "$Root\frontend"
    npm install --prefer-offline --no-audit --no-fund
    if ($LASTEXITCODE -ne 0) { Pop-Location; exit 1 }

    Write-Host "[1/4] Frontend -- npm run build" -ForegroundColor Green
    npm run build
    if ($LASTEXITCODE -ne 0) { Pop-Location; exit 1 }
    Pop-Location
}

# ---------------------------------------------------------------------------
# Step 2 -- GUI app (Wails embeds the built frontend into the binary)
# ---------------------------------------------------------------------------
Write-Host "[2/4] App -- wails build (windows/amd64)" -ForegroundColor Green
Push-Location $Root
# -s skips the frontend build inside wails; step 1 already handled it
Remove-Item "$BinDir\ProIdentity.exe" -Force -ErrorAction SilentlyContinue
Remove-Item "$BinDir\ProIdentity Access.exe" -Force -ErrorAction SilentlyContinue
wails build -platform windows/amd64 -s -ldflags "-X main.appVersion=$Version"
if ($LASTEXITCODE -ne 0) { Pop-Location; exit 1 }
Pop-Location

if (-not (Test-Path "$BinDir\ProIdentity Access.exe")) {
    Write-Host "  ERROR: ProIdentity Access.exe not found in $BinDir" -ForegroundColor Red
    exit 1
}

# ---------------------------------------------------------------------------
# Step 3 -- Daemon
# ---------------------------------------------------------------------------
Write-Host "[3/4] Daemon -- go build (windows/amd64)" -ForegroundColor Green
Push-Location $Root
$env:GOOS   = "windows"
$env:GOARCH = "amd64"
go build -ldflags="-s -w" -o "build\bin\ProIdentity Daemon.exe" .\cmd\daemon
$rc = $LASTEXITCODE
Remove-Item Env:\GOOS, Env:\GOARCH -ErrorAction SilentlyContinue
Pop-Location
if ($rc -ne 0) { exit 1 }

if (-not (Test-Path "$BinDir\ProIdentity Daemon.exe")) {
    Write-Host "  ERROR: daemon exe not found in $BinDir" -ForegroundColor Red
    exit 1
}

# ---------------------------------------------------------------------------
# Step 4 -- MSI installer
# ---------------------------------------------------------------------------
Write-Host "[4/4] Installer -- wix build (x64)" -ForegroundColor Green
$OutMsi = "$Root\build\ProIdentity-Access-$Version.msi"

Push-Location "$Root\installer"
wix build "Product.wxs" `
    -ext WixToolset.UI.wixext `
    -ext WixToolset.Util.wixext `
    -d Version=$Version `
    -arch x64 `
    -o $OutMsi
$rc = $LASTEXITCODE
Pop-Location
if ($rc -ne 0) { exit 1 }

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------
$msiPath = Resolve-Path $OutMsi
$msiSize = "{0:N1} MB" -f ((Get-Item $msiPath).Length / 1MB)

if (-not $SkipServerPackage) {
    $serverRoot = Resolve-Path "$Root\..\server" -ErrorAction SilentlyContinue
    if ($serverRoot) {
        $updatesDir = Join-Path $serverRoot "internal\api\client_updates\windows"
        New-Item -ItemType Directory -Force -Path $updatesDir | Out-Null

        $fileName = Split-Path $msiPath -Leaf
        $serverMsi = Join-Path $updatesDir $fileName
        Copy-Item -LiteralPath $msiPath -Destination $serverMsi -Force

        $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $msiPath).Hash.ToLowerInvariant()
        $manifest = [ordered]@{
            version      = $Version
            latest_version = $Version
            platform     = "windows-amd64"
            filename     = $fileName
            url          = "/api/v1/client-updates/windows/$fileName"
            sha256       = $hash
            size         = (Get-Item -LiteralPath $msiPath).Length
            published_at = (Get-Date).ToUniversalTime().ToString("o")
            mandatory    = $false
        }
        $manifestJson = $manifest | ConvertTo-Json
        [System.IO.File]::WriteAllText(
            (Join-Path $updatesDir "latest.json"),
            $manifestJson,
            [System.Text.UTF8Encoding]::new($false)
        )
        Write-Host "  Published client update package to server embed directory" -ForegroundColor Green
    }
}

Write-Host ""
Write-Host "  Build complete" -ForegroundColor Cyan
Write-Host "  $msiPath  ($msiSize)"
Write-Host ""
