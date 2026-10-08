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
    [switch]$SkipServerPackage,
    # Authenticode-sign our own binaries + installer with Azure Artifact Signing.
    # On by default; -SkipSign produces an unsigned local build.
    [switch]$SkipSign,
    # Azure signing metadata JSON (endpoint/account/profile). Defaults to the
    # per-user guide file; override with $env:ARTIFACT_SIGNING_METADATA.
    [string]$SigningMetadata = $(if ($env:ARTIFACT_SIGNING_METADATA) { $env:ARTIFACT_SIGNING_METADATA } else { "$HOME\signing\proit.json" })
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$Root   = $PSScriptRoot
$BinDir = "$Root\build\bin"

# ---------------------------------------------------------------------------
# Code signing (Azure Artifact Signing — PRO IT SERVICES s.r.o.)
# Signs only our own files (app exe, daemon exe, MSI). Third-party binaries
# (OpenVPN, OpenSSL, wintun, driver MSMs) keep their vendor signatures and are
# never re-signed. Guide: C:\Users\Marko\signing\README.md
# ---------------------------------------------------------------------------
function Resolve-SignTool {
    $c = Get-Command signtool.exe -ErrorAction SilentlyContinue
    if ($c) { return $c.Source }
    $kitBin = "${env:ProgramFiles(x86)}\Windows Kits\10\bin"
    if (Test-Path $kitBin) {
        $found = Get-ChildItem -Path $kitBin -Recurse -Filter signtool.exe -ErrorAction SilentlyContinue |
                 Where-Object { $_.FullName -match '\\x64\\signtool\.exe$' } |
                 Sort-Object FullName -Descending | Select-Object -First 1
        if ($found) { return $found.FullName }
    }
    return $null
}

function Invoke-Sign {
    param([string[]]$Paths, [string]$Description = "ProIdentity Access")

    if ($SkipSign) {
        Write-Host "  [sign] skipped (-SkipSign): $([IO.Path]::GetFileName($Paths[0]))$(if ($Paths.Count -gt 1) { " +$($Paths.Count - 1)" })" -ForegroundColor DarkGray
        return
    }

    $signtool = Resolve-SignTool
    $dlib     = "$env:LOCALAPPDATA\Microsoft\MicrosoftArtifactSigningClientTools\Azure.CodeSigning.Dlib.dll"
    if (-not $signtool) {
        Write-Host "  ERROR: signtool.exe not found. Install the Windows SDK, or pass -SkipSign." -ForegroundColor Red; exit 1
    }
    if (-not (Test-Path $dlib)) {
        Write-Host "  ERROR: Azure signing dlib not found: $dlib" -ForegroundColor Red
        Write-Host "         winget install Microsoft.Azure.ArtifactSigningClientTools, or pass -SkipSign." -ForegroundColor Red; exit 1
    }
    if (-not (Test-Path $SigningMetadata)) {
        Write-Host "  ERROR: signing metadata not found: $SigningMetadata" -ForegroundColor Red
        Write-Host "         Set `$env:ARTIFACT_SIGNING_METADATA, or pass -SkipSign." -ForegroundColor Red; exit 1
    }
    foreach ($p in $Paths) {
        if (-not (Test-Path $p)) { Write-Host "  ERROR: cannot sign missing file: $p" -ForegroundColor Red; exit 1 }
    }

    Write-Host "  [sign] $([IO.Path]::GetFileName($Paths[0]))$(if ($Paths.Count -gt 1) { " +$($Paths.Count - 1)" })" -ForegroundColor Green
    & $signtool sign /v /fd SHA256 /tr http://timestamp.acs.microsoft.com /td SHA256 `
        /d $Description /dlib $dlib /dmdf $SigningMetadata @Paths
    if ($LASTEXITCODE -ne 0) {
        Write-Host "  ERROR: signing failed (exit $LASTEXITCODE). Run 'az login' as the signer identity, or pass -SkipSign." -ForegroundColor Red; exit 1
    }
    foreach ($p in $Paths) {
        $sig = Get-AuthenticodeSignature -LiteralPath $p
        if ($sig.Status -ne 'Valid') {
            Write-Host "  ERROR: signature not Valid on $([IO.Path]::GetFileName($p)): $($sig.Status)" -ForegroundColor Red; exit 1
        }
    }
}

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
wails build -platform windows/amd64 -s -ldflags "-X wg-client/internal/update.Version=$Version"
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
go build -ldflags="-s -w -X wg-client/internal/update.Version=$Version" -o "build\bin\ProIdentity Daemon.exe" .\cmd\daemon
$rc = $LASTEXITCODE
Remove-Item Env:\GOOS, Env:\GOARCH -ErrorAction SilentlyContinue
Pop-Location
if ($rc -ne 0) { exit 1 }

if (-not (Test-Path "$BinDir\ProIdentity Daemon.exe")) {
    Write-Host "  ERROR: daemon exe not found in $BinDir" -ForegroundColor Red
    exit 1
}

# ---------------------------------------------------------------------------
# Step 3b -- Sign our binaries (before packaging, so the MSI embeds signed exes)
# The daemon carries the embedded wintun DLL; signing the daemon covers it.
# ---------------------------------------------------------------------------
Write-Host "[3/4] Sign -- app + daemon" -ForegroundColor Green
Invoke-Sign -Paths @("$BinDir\ProIdentity Access.exe", "$BinDir\ProIdentity Daemon.exe")

# ---------------------------------------------------------------------------
# Step 4 -- MSI installer
# ---------------------------------------------------------------------------
Write-Host "[4/4] Installer -- wix build (x64)" -ForegroundColor Green
$OutMsi = "$Root\build\ProIdentity-Access-$Version.msi"

# Bundled OpenVPN runtime + drivers (official, pinned, signature-checked).
& "$Root\tools\fetch-openvpn.ps1"
$OpenVPNDir = "$Root\build\third_party\openvpn-windows"

Push-Location "$Root\installer"
wix build "Product.wxs" `
    -ext WixToolset.UI.wixext `
    -ext WixToolset.Util.wixext `
    -d Version=$Version `
    -d OpenVPNDir=$OpenVPNDir `
    -arch x64 `
    -o $OutMsi
$rc = $LASTEXITCODE
Pop-Location
if ($rc -ne 0) { exit 1 }

# Sign the installer. Must happen before the update feed is generated so the
# Ed25519 feed signature covers the Authenticode-signed bytes.
Write-Host "[4/4] Sign -- installer" -ForegroundColor Green
Invoke-Sign -Paths @($OutMsi)

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------
$msiPath = Resolve-Path $OutMsi
$msiSize = "{0:N1} MB" -f ((Get-Item $msiPath).Length / 1MB)

if (-not $SkipServerPackage) {
    # Signed update feed for the clients' services (see publish-update.ps1).
    # Fails the build if the release signing key is missing.
    & "$Root\publish-update.ps1" -File $msiPath -Platform "windows-amd64" -Version $Version
}

Write-Host ""
Write-Host "  Build complete" -ForegroundColor Cyan
Write-Host "  $msiPath  ($msiSize)"
Write-Host ""
