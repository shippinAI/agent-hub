# Install devagotchi.exe for this user (Windows). No admin, no service, no telemetry.
#   $env:DEVAGOTCHI_BASE_URL="$BASE"; irm "$BASE/install.ps1" | iex
#   .\install.ps1 -From .\dist\release          (offline)
param([string]$From = $env:DEVAGOTCHI_FROM, [string]$Base = $env:DEVAGOTCHI_BASE_URL,
      [string]$BinDir = $(if ($env:DEVAGOTCHI_BIN_DIR) { $env:DEVAGOTCHI_BIN_DIR } else { "$env:LOCALAPPDATA\Programs\Devagotchi" }))
$ErrorActionPreference = 'Stop'
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
$name = "devagotchi-windows-$arch.exe"
$tmp = Join-Path ([IO.Path]::GetTempPath()) ("dg-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
  foreach ($f in @($name, 'SHA256SUMS')) {
    if ($From) { Copy-Item (Join-Path $From $f) (Join-Path $tmp $f) }
    elseif ($Base) { Invoke-WebRequest -UseBasicParsing -Uri "$Base/$f" -OutFile (Join-Path $tmp $f) }
    else { throw 'set DEVAGOTCHI_BASE_URL or pass -From DIR' }
  }
  $want = (Get-Content (Join-Path $tmp 'SHA256SUMS') | Where-Object { $_ -match " $([regex]::Escape($name))$" }) -split ' ' | Select-Object -First 1
  $got = (Get-FileHash -Algorithm SHA256 (Join-Path $tmp $name)).Hash.ToLower()
  if (-not $want -or $want -ne $got) { throw "checksum mismatch for $name" }
  New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
  Copy-Item -Force (Join-Path $tmp $name) (Join-Path $BinDir 'devagotchi.exe')
  $p = [Environment]::GetEnvironmentVariable('Path', 'User')
  if (($p -split ';') -notcontains $BinDir) { [Environment]::SetEnvironmentVariable('Path', "$p;$BinDir", 'User'); Write-Host "added $BinDir to your PATH (new terminals)" }
  & (Join-Path $BinDir 'devagotchi.exe') version
  Write-Host 'next: devagotchi doctor; devagotchi config init --root $HOME\code'
} finally { Remove-Item -Recurse -Force $tmp }
