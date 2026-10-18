# One-command Devagotchi bridge install for a Windows host. In an elevated PowerShell:
#   powershell -ExecutionPolicy Bypass -File .\install.ps1
# 1. picks the exe for the CPU  2. asks for the enrollment code (hidden; never a flag)  3. enrolls with the hub in
# host.env  4. installs + starts the DevagotchiBridge service. Needs Jesse's approval per host before it is run.
$ErrorActionPreference = 'Stop'
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$cfg = @{}; Get-Content "$here\host.env" | Where-Object { $_ -match '^\s*[A-Z_]+=' } | ForEach-Object {
  $k, $v = $_ -split '=', 2; $cfg[$k.Trim()] = $v.Trim().Trim('"') }
if (-not $cfg.HUB_URL) { throw 'host.env: HUB_URL is empty (set it to the hub tailnet URL)' }
if ($cfg.HUB_URL -match '127\.0\.0\.1|localhost') { throw 'HUB_URL is loopback; a remote host needs the tailnet URL' }
$admin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole('Administrators')
if (-not $admin) { throw 'run this in an elevated PowerShell (the service and ProgramData config need it)' }
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
$exe = "devagotchi-bridge-windows-$arch.exe"
$want = (Get-Content "$here\bin\SHA256SUMS" | Where-Object { $_ -match [regex]::Escape($exe) }) -split '\s+' | Select-Object -First 1
$got = (Get-FileHash "$here\bin\$exe" -Algorithm SHA256).Hash.ToLower()
if ($want -ne $got) { throw "checksum mismatch for $exe" }
$dir = "$env:ProgramFiles\Devagotchi"; New-Item -ItemType Directory -Force $dir | Out-Null
$dst = "$dir\devagotchi-bridge.exe"; Copy-Item "$here\bin\$exe" $dst -Force
Write-Host "Machine id in the hub: $($cfg.MACHINE_ID) ($($cfg.MACHINE_LABEL)). Get its code: hub > Machines > Add machine."
$sec = Read-Host 'Enrollment code' -AsSecureString
$code = [Runtime.InteropServices.Marshal]::PtrToStringBSTR([Runtime.InteropServices.Marshal]::SecureStringToBSTR($sec))
$code | & $dst enroll --hub $cfg.HUB_URL; Remove-Variable code
if ($LASTEXITCODE -ne 0) { throw 'enroll failed' }
& $dst probe
& $dst install
& powershell -ExecutionPolicy Bypass -File "$env:ProgramData\Devagotchi\install-service.ps1"
sc.exe failure DevagotchiBridge reset= 86400 actions= restart/5000/restart/30000/restart/60000 | Out-Null
Get-Service DevagotchiBridge
