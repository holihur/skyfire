<#
.SYNOPSIS
  Windows full-tunnel smoke test for skyfire-client.

.DESCRIPTION
  Verifies that a full tunnel (AllowedIPs 0.0.0.0/0) comes up correctly and,
  critically, that the endpoint-exception route keeps the tunnel's own
  transport on the physical path (so surfacing the internet through the
  tunnel does not cut the machine off).

  The script reports:
    * the full-tunnel /1 routes (must appear only while connected, via the
      Wintun "Skyfire" adapter),
    * the endpoint /32 exception route (must point at the physical default
      gateway),
    * whether the internet is reachable through the tunnel and what the
      egress IP is,
    * that the /1 routes are gone after the client stops.

  Before touching anything it schedules a one-shot task that force-kills the
  client after -RecoverySeconds, so a regression that captures the tunnel's
  own transport (a lockout) cannot strand the machine: killing the client
  closes the Wintun adapter and removes the full-tunnel routes.

  Requires an elevated PowerShell. wintun.dll must sit next to the client exe.

.PARAMETER ConnectUrl
  The peer config URL, e.g. http://host:51821/api/p/<token>/wg.conf

.EXAMPLE
  .\windows-fulltunnel-test.ps1 -ConnectUrl 'http://vpn.example.com:51821/api/p/<token>/wg.conf'
#>
param(
  [string]$ClientExe = (Join-Path $PWD 'skyfire-client-windows-amd64.exe'),
  [Parameter(Mandatory = $true)][string]$ConnectUrl,
  [int]$RunSeconds = 25,
  [int]$RecoverySeconds = 150,
  [string]$WorkDir = $PWD
)

$ErrorActionPreference = 'Stop'

$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
  Write-Error 'run this from an elevated PowerShell (Administrator)'
  exit 2
}
if (-not (Test-Path $ClientExe)) { Write-Error "client not found: $ClientExe"; exit 2 }
if (-not (Test-Path (Join-Path (Split-Path $ClientExe -Parent) 'wintun.dll'))) {
  Write-Warning 'wintun.dll not found next to the client exe; the tunnel will fail to start'
}

$log = Join-Path $WorkDir 'skyfire-wintest.log'
$errLog = Join-Path $WorkDir 'skyfire-wintest.client.err'
$outLog = Join-Path $WorkDir 'skyfire-wintest.client.out'
$recoverName = 'SkyfireClientTestRecover'

function Log($m) { $m | Tee-Object -FilePath $log -Append | Out-Null; Write-Host $m }
function OneOneRoutes { (Get-NetRoute -AddressFamily IPv4 |
    Where-Object { $_.DestinationPrefix -in @('0.0.0.0/1', '128.0.0.0/1') } |
    Format-Table DestinationPrefix, NextHop, InterfaceIndex, RouteMetric -AutoSize | Out-String) }

Log "==== skyfire windows full-tunnel test $(Get-Date -Format o) ===="
Log "client : $ClientExe"
Log "url    : $ConnectUrl"

# Safety net: kill the client after RecoverySeconds no matter what.
$recAction = New-ScheduledTaskAction -Execute 'powershell.exe' `
  -Argument '-NoProfile -Command "Stop-Process -Name skyfire-client* -Force -ErrorAction SilentlyContinue"'
$recTrigger = New-ScheduledTaskTrigger -Once -At (Get-Date).AddSeconds($RecoverySeconds)
Register-ScheduledTask -TaskName $recoverName -Action $recAction -Trigger $recTrigger `
  -User 'SYSTEM' -RunLevel Highest -Force | Out-Null
Log "recovery backstop scheduled (+${RecoverySeconds}s)"

$client = $null
try {
  Log ''
  Log "default gateway : $((Get-NetRoute -DestinationPrefix 0.0.0.0/0 | Sort-Object RouteMetric | Select-Object -First 1).NextHop)"
  Log '--- /1 routes (before) ---'
  Log (OneOneRoutes)

  Log '--- starting client ---'
  $client = Start-Process -FilePath $ClientExe -ArgumentList @('-cli', '-connect', $ConnectUrl) -PassThru `
    -RedirectStandardError $errLog -RedirectStandardOutput $outLog
  Log "client pid=$($client.Id)"
  Start-Sleep -Seconds $RunSeconds

  Log '--- client log ---'
  Log ((Get-Content $errLog -Raw -ErrorAction SilentlyContinue))
  Log '--- /1 routes (during, expect the Skyfire adapter) ---'
  Log (OneOneRoutes)
  Log '--- /32 routes (during, expect the endpoint via the physical gateway) ---'
  Log ((Get-NetRoute -AddressFamily IPv4 | Where-Object { $_.DestinationPrefix -like '*/32' } |
      Format-Table DestinationPrefix, NextHop, InterfaceIndex -AutoSize | Out-String))

  Log "google http = $(curl.exe -4 -sS --max-time 20 -o NUL -w '%{http_code}' https://www.google.com)"
  Log "egress ip   = $(curl.exe -4 -sS --max-time 15 https://api.ipify.org)"

  Log '--- stopping client ---'
  Stop-Process -Id $client.Id -Force -ErrorAction SilentlyContinue
  Start-Sleep -Seconds 4
  Log '--- /1 routes (after, expect none) ---'
  Log (OneOneRoutes)
  Log '==== DONE ===='
}
finally {
  Get-Process -Name 'skyfire-client*' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
  Unregister-ScheduledTask -TaskName $recoverName -Confirm:$false -ErrorAction SilentlyContinue
  Log 'recovery backstop removed'
}
