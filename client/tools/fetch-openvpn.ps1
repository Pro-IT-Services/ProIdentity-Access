<#
.SYNOPSIS
    Fetch the official OpenVPN runtime that ships inside the Windows installer.

.DESCRIPTION
    Downloads official, pinned artifacts and stages the files the MSI bundles
    into build\third_party\openvpn-windows\:

      bin\openvpn.exe + DLLs, tapctl.exe      from the OpenVPN 2.7.7 MSI (OpenVPN Inc.)
      ssl\modules\legacy.dll                  (OpenSSL legacy provider, old ciphers)
      msm\ovpn-dco-amd64.msm                  ovpn-dco-win 2.8.7 driver (DCO, TUN)
      msm\tap-windows-9.27.0-I0-amd64.msm     tap-windows6 9.27.0 driver (TAP, fallback)
      license.txt                             OpenVPN license (GPLv2 + bundled libraries)

    Every download is checked against a pinned SHA-256; the OpenVPN MSI must
    also carry a valid Authenticode signature from OpenVPN Inc. Nothing is
    installed on this machine. Bumping a version means updating the URL and
    hash below together.

    OpenVPN is GPLv2 and runs as a separate program next to ProIdentity
    Access; see installer\THIRD-PARTY-NOTICES.txt for the source offer.
#>
param([switch]$Force)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
$Out = Join-Path $Root "build\third_party\openvpn-windows"
$Cache = Join-Path $Root "build\third_party\downloads"

$Artifacts = @(
    @{ Name = "OpenVPN-2.7.7-I001-amd64.msi"
       Url = "https://swupdate.openvpn.org/community/releases/OpenVPN-2.7.7-I001-amd64.msi"
       Sha256 = "9069a48397c4fd4135fb9c9baea4863c74a465651f67e55a423a433e503100bb" },
    @{ Name = "ovpn-dco-amd64.msm"
       Url = "https://github.com/OpenVPN/ovpn-dco-win/releases/download/2.8.7/ovpn-dco-amd64.msm"
       Sha256 = "03ec775ea1be356b7ce0a3001b3eff477c21247640db9d0f637e32c3ec96ac3f" },
    @{ Name = "tap-windows-9.27.0-I0-amd64.msm"
       Url = "https://github.com/OpenVPN/tap-windows6/releases/download/9.27.0/tap-windows-9.27.0-I0-amd64.msm"
       Sha256 = "0f52d8e2b22a0b827b0d5f1e23077dc53f2b41b67839737c39503dc20c435063" }
)

$BinFiles = @("openvpn.exe", "libcrypto-3-x64.dll", "libssl-3-x64.dll",
              "libpkcs11-helper-1.dll", "vcruntime140.dll", "tapctl.exe")

$stamp = Join-Path $Out ".complete"
if ((Test-Path $stamp) -and -not $Force) {
    Write-Host "  OpenVPN runtime already staged ($Out)" -ForegroundColor DarkGray
    return
}

New-Item -ItemType Directory -Force -Path $Cache | Out-Null
foreach ($a in $Artifacts) {
    $path = Join-Path $Cache $a.Name
    if (-not (Test-Path $path) -or (Get-FileHash $path -Algorithm SHA256).Hash.ToLowerInvariant() -ne $a.Sha256) {
        Write-Host "  Downloading $($a.Name)"
        Invoke-WebRequest -Uri $a.Url -OutFile $path -UseBasicParsing
    }
    $hash = (Get-FileHash $path -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($hash -ne $a.Sha256) {
        Remove-Item $path -Force
        throw "$($a.Name): SHA-256 mismatch (got $hash). Refusing to bundle it."
    }
}

$msi = Join-Path $Cache $Artifacts[0].Name
$sig = Get-AuthenticodeSignature $msi
if ($sig.Status -ne "Valid" -or $sig.SignerCertificate.Subject -notmatch "O=OpenVPN Inc\.") {
    throw "OpenVPN MSI signature is not a valid OpenVPN Inc. signature ($($sig.Status))."
}

# Administrative extraction: unpacks files without installing anything.
$extract = Join-Path $Cache "openvpn-extract"
Remove-Item $extract -Recurse -Force -ErrorAction SilentlyContinue
$p = Start-Process msiexec.exe -ArgumentList "/a `"$msi`" /qn TARGETDIR=`"$extract`"" -Wait -PassThru
if ($p.ExitCode -ne 0) { throw "Extracting the OpenVPN MSI failed ($($p.ExitCode))." }
$src = Join-Path $extract "OpenVPN"

Remove-Item $Out -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path (Join-Path $Out "bin"), (Join-Path $Out "ssl\modules"), (Join-Path $Out "msm") | Out-Null
foreach ($f in $BinFiles) {
    Copy-Item (Join-Path $src "bin\$f") (Join-Path $Out "bin\$f")
}
Copy-Item (Join-Path $src "ssl\modules\legacy.dll") (Join-Path $Out "ssl\modules\legacy.dll")
Copy-Item (Join-Path $src "license.txt") (Join-Path $Out "license.txt")
Copy-Item (Join-Path $Cache $Artifacts[1].Name) (Join-Path $Out "msm\ovpn-dco-amd64.msm")
Copy-Item (Join-Path $Cache $Artifacts[2].Name) (Join-Path $Out "msm\tap-windows-amd64.msm")

foreach ($f in $BinFiles) {
    $s = Get-AuthenticodeSignature (Join-Path $Out "bin\$f")
    if ($s.Status -ne "Valid") { throw "bin\$f is not validly signed ($($s.Status))." }
}
Remove-Item $extract -Recurse -Force
Set-Content -Path $stamp -Value (Get-Date).ToString("o")
Write-Host "  OpenVPN runtime staged: $Out" -ForegroundColor Green
