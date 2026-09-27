<#
.SYNOPSIS
    Publish a client installer as a signed update in the server's update feed.

.DESCRIPTION
    Copies the installer into server\internal\api\client_updates\<platform>\,
    removes older installers there, and writes latest.json signed with the
    release key (tools\updatesign). Clients' services verify that signature
    before installing, so only packages published here can be installed.

    The server embeds the feed, so rebuild and deploy the server afterwards.

.EXAMPLE
    .\publish-update.ps1 -File build\ProIdentity-Access-0.7.3.msi
    .\publish-update.ps1 -File build\darwin\ProIdentity-Access-0.7.3.pkg -Platform darwin-arm64
    .\publish-update.ps1 -File build\ProIdentity-Access-0.7.3.msi -Mandatory -Notes "Security fix"
#>
param(
    [Parameter(Mandatory = $true)][string]$File,
    [string]$Platform,
    [string]$Version,
    [switch]$Mandatory,
    [string]$Notes = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$Root = $PSScriptRoot

$File = (Resolve-Path $File).Path
$fileName = Split-Path $File -Leaf

if (-not $Platform) {
    switch ([IO.Path]::GetExtension($fileName).ToLowerInvariant()) {
        ".msi" { $Platform = "windows-amd64" }
        ".pkg" { throw "Pass -Platform darwin-arm64 or darwin-amd64 for macOS packages." }
        default { throw "Unsupported installer type: $fileName" }
    }
}
if (-not $Version) {
    if ($fileName -match '(\d+\.\d+\.\d+)') { $Version = $Matches[1] }
    else { throw "Pass -Version; it can't be read from the file name." }
}

$serverRoot = Resolve-Path "$Root\..\server" -ErrorAction SilentlyContinue
if (-not $serverRoot) { throw "Server repository not found next to the client." }
$feedDir = Join-Path $serverRoot "internal\api\client_updates\$Platform"
New-Item -ItemType Directory -Force -Path $feedDir | Out-Null

# Keep only this installer: the server binary embeds everything in the feed.
Get-ChildItem $feedDir -File | Where-Object { $_.Extension -in ".msi", ".pkg" -and $_.Name -ne $fileName } |
    Remove-Item -Force
Copy-Item -LiteralPath $File -Destination (Join-Path $feedDir $fileName) -Force

$signArgs = @(
    "run", "./tools/updatesign",
    "-platform", $Platform,
    "-version", $Version,
    "-file", $File,
    "-url", "/api/v1/client-updates/$Platform/$fileName",
    "-out", (Join-Path $feedDir "latest.json")
)
if ($Mandatory) { $signArgs += "-mandatory" }
if ($Notes) { $signArgs += @("-notes", $Notes) }

Push-Location $Root
try {
    & go @signArgs
    if ($LASTEXITCODE -ne 0) { throw "Signing the update manifest failed." }
} finally {
    Pop-Location
}

Write-Host "Published $Platform $Version to $feedDir" -ForegroundColor Green
Write-Host "Rebuild and deploy the server to make it available to clients." -ForegroundColor DarkGray
