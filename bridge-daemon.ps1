<#
.SYNOPSIS
    Runs the WhatsApp bridge in the background and restarts it if it exits.

.DESCRIPTION
    Meant to be launched by a scheduled task at logon. No window appears: the
    child process is created without a console of its own.

    Build the bridge first:
        cd whatsapp-bridge
        go build -o whatsapp-bridge.exe .
    On Windows go-sqlite3 needs cgo, so a C compiler (e.g. MinGW-w64) must be on
    PATH and CGO_ENABLED must be 1. A binary built with CGO_ENABLED=0 still
    compiles but fails at runtime with a stub SQLite driver.

    Example scheduled task (adjust the path to this script):
        schtasks /create /tn WhatsAppBridge /sc onlogon /rl limited /f ^
          /tr "powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File C:\path\to\bridge-daemon.ps1"

.PARAMETER Port
    Port the bridge's REST API listens on. Used only to detect an instance that
    is already running.
#>
[CmdletBinding()]
param(
    [int]$Port = 8080
)

$ErrorActionPreference = 'Stop'

$BridgeDir = Join-Path $PSScriptRoot 'whatsapp-bridge'
$Exe       = Join-Path $BridgeDir 'whatsapp-bridge.exe'
$Log       = Join-Path $BridgeDir 'store\bridge.log'

New-Item -ItemType Directory -Force -Path (Split-Path $Log) | Out-Null

function Write-Log([string]$msg) {
    # A running bridge holds the log open through cmd's redirect, so appending can
    # fail. Logging must never take the daemon down with it.
    try {
        "[{0}] {1}" -f (Get-Date -Format 's'), $msg | Out-File -FilePath $Log -Append -Encoding utf8 -ErrorAction Stop
    } catch {
        Write-Verbose "Could not write to $($Log): $_"
    }
}

# Already listening? Another instance is running, so leave it alone.
if (Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue) {
    Write-Log "Port $Port already in use: exiting (bridge started elsewhere)."
    exit 0
}

if (-not (Test-Path $Exe)) {
    Write-Log "Executable not found: $Exe - build it with 'go build -o whatsapp-bridge.exe .'"
    exit 1
}

# Rotate the log at 5 MB.
if ((Test-Path $Log) -and ((Get-Item $Log).Length -gt 5MB)) {
    Move-Item $Log "$Log.1" -Force
}

$fastFailures = 0

while ($true) {
    Write-Log 'Starting whatsapp-bridge.exe'
    $started = Get-Date

    # cmd handles appending to the log. No window appears: the scheduled task
    # runs everything under "conhost.exe --headless", so the whole process tree
    # shares a console with no window.
    Set-Location $BridgeDir
    & cmd.exe /c "`"$Exe`" >> `"$Log`" 2>&1"
    $code = $LASTEXITCODE

    $uptime = (Get-Date) - $started
    Write-Log ("Process exited ({0}) after {1:N0}s" -f $code, $uptime.TotalSeconds)

    if ($uptime.TotalSeconds -lt 60) { $fastFailures++ } else { $fastFailures = 0 }

    if ($fastFailures -ge 5) {
        Write-Log 'Five rapid exits in a row: giving up. The WhatsApp session has probably expired - run start-bridge.cmd by hand and scan the QR code again.'
        exit 1
    }

    Start-Sleep -Seconds 30
}
