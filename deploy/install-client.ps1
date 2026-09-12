<#
.SYNOPSIS
  Skyfire desktop client one-click installer (Windows).

.DESCRIPTION
  Downloads the prebuilt skyfire-client for Windows together with its
  wintun.dll dependency, installs both under %LOCALAPPDATA%\Skyfire, verifies
  the client checksum, and creates a "Run as administrator" desktop shortcut.

  Running the client needs Administrator (it creates a Wintun adapter and
  manages routes), which is why the shortcut is flagged to elevate.

.EXAMPLE
  irm https://raw.githubusercontent.com/holihur/skyfire/main/deploy/install-client.ps1 | iex

.EXAMPLE
  # pin a version
  $env:SKYFIRE_VERSION = 'v0.7.0'
  irm https://raw.githubusercontent.com/holihur/skyfire/main/deploy/install-client.ps1 | iex
#>
[CmdletBinding()]
param(
  [string]$Version    = $env:SKYFIRE_VERSION,
  [string]$InstallDir = (Join-Path $env:LOCALAPPDATA 'Skyfire'),
  [string]$Repo       = 'holihur/skyfire',
  [string]$WintunVersion = '0.14.1',
  [switch]$NoShortcut
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

function Info($m) { Write-Host "==> $m" -ForegroundColor Cyan }
function Warn($m) { Write-Warning $m }
function Die($m)  { Write-Error "ERROR: $m"; exit 1 }

if (-not [Environment]::Is64BitOperatingSystem) { Die 'only 64-bit Windows is supported' }
if (-not $Version) {
  Info 'resolving latest release'
  try {
    $Version = (Invoke-RestMethod 'https://api.github.com/repos/holihur/skyfire/releases/latest').tag_name
  } catch {
    Die "could not resolve the latest release (set `$env:SKYFIRE_VERSION): $_"
  }
}
Info "installing skyfire-client $Version"

$assetBase = if ($Version -eq 'latest') {
  "https://github.com/$Repo/releases/latest/download"
} else {
  "https://github.com/$Repo/releases/download/$Version"
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$exe = Join-Path $InstallDir 'skyfire-client.exe'
$dll = Join-Path $InstallDir 'wintun.dll'

# --- dependency: wintun.dll (the client will not create a tunnel without it) ---
Info "installing wintun.dll $WintunVersion (dependency)"
$zip = Join-Path $env:TEMP "wintun-$WintunVersion.zip"
$tmp = Join-Path $env:TEMP "wintun-$WintunVersion"
Invoke-WebRequest -UseBasicParsing "https://www.wintun.net/builds/wintun-$WintunVersion.zip" -OutFile $zip
Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
Expand-Archive -Path $zip -DestinationPath $tmp -Force
Copy-Item -Force (Join-Path $tmp 'wintun/bin/amd64/wintun.dll') $dll
Info "wintun.dll -> $dll"

# --- client binary ---
Info 'downloading skyfire-client-windows-amd64.exe'
Invoke-WebRequest -UseBasicParsing "$assetBase/skyfire-client-windows-amd64.exe" -OutFile "$exe.tmp"
try {
  $sums = (Invoke-WebRequest -UseBasicParsing "$assetBase/checksums.txt").Content
  if ($sums -is [byte[]]) { $sums = [Text.Encoding]::UTF8.GetString($sums) }
  $line = ($sums -split "`n" | Where-Object { $_ -match 'skyfire-client-windows-amd64\.exe\s*$' } | Select-Object -First 1)
  if ($line) {
    $want = ($line -split '\s+')[0].ToLower()
    $got  = (Get-FileHash "$exe.tmp" -Algorithm SHA256).Hash.ToLower()
    if ($want -ne $got) { Remove-Item "$exe.tmp" -Force; Die 'checksum mismatch for skyfire-client' }
    Info 'checksum ok'
  } else {
    Warn 'client not found in checksums.txt; skipping verification'
  }
} catch { Warn "checksum verification skipped: $_" }
Move-Item -Force "$exe.tmp" $exe
Info "client -> $exe"

# --- desktop shortcut that runs as administrator ---
if (-not $NoShortcut) {
  $lnk = Join-Path ([Environment]::GetFolderPath('Desktop')) 'Skyfire Client.lnk'
  $ws = New-Object -ComObject WScript.Shell
  $sc = $ws.CreateShortcut($lnk)
  $sc.TargetPath = $exe
  $sc.WorkingDirectory = $InstallDir
  $sc.Description = 'Skyfire WireGuard client'
  $sc.Save()
  # set the RUNASADMIN bit (byte 0x15, 0x20) so the shortcut self-elevates
  $bytes = [IO.File]::ReadAllBytes($lnk)
  $bytes[0x15] = $bytes[0x15] -bor 0x20
  [IO.File]::WriteAllBytes($lnk, $bytes)
  Info "desktop shortcut: $lnk (run as administrator)"
}

Write-Host ''
Write-Host "skyfire-client $Version installed to $InstallDir" -ForegroundColor Green
Write-Host 'Get the connect URL from the Web UI (Peer details -> Desktop client), then:'
Write-Host "  & '$exe' -connect '<connect-url>'"
Write-Host 'or double-click the "Skyfire Client" desktop shortcut.'
