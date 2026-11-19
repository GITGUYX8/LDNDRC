# joining-cluster.ps1 — LDNDRC enrollment agent for Windows.
#
# Downloaded from the master laptop's onboarding portal
# (http://<master-ip>:8082/portal) while on the onboarding Wi-Fi.
# Mirrors joining-cluster.sh: profile the laptop, register with the
# control panel, wait for the instructor's decision, then switch to the
# cluster Wi-Fi. Re-running is safe — the server deduplicates
# registrations by hardware fingerprint.
#
# Run it from PowerShell in your Downloads folder:
#   powershell -ExecutionPolicy Bypass -File joining-cluster.ps1
#
# Note: k3s agents are Linux-only. Windows laptops enroll as Guests and
# use the platform from the browser; the script handles approval and
# the network switch, and stops before any cluster agent install.
#
# Environment:
#   PANEL_URL       e.g. http://192.168.4.1:8082 (skips the prompt)
#   JOIN_KEY        shared enrollment key, if the panel requires one
#   CLUSTER_SSID    cluster Wi-Fi network to switch to after approval
#   ENROLL_TIMEOUT  seconds to wait for approval (default 600)
# Testing: TEST_CPU TEST_RAM TEST_GPU ("nvidia"|"none")
#Requires -Version 5.1
[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$EnrollTimeout = if ($env:ENROLL_TIMEOUT) { [int]$env:ENROLL_TIMEOUT } else { 600 }

function Log([string]$Message) { Write-Host "[joining-cluster] $Message" }
function Die([string]$Message) { Log "ERROR: $Message"; exit 1 }

# --- Hardware detection (TEST_* overrides mirror the bash script) -------
$Cpu = 0; $RamGB = 0; $Gpu = "none"; $IsHost = $false
$Os = "windows"; $Arch = $env:PROCESSOR_ARCHITECTURE.ToLower()

if ($env:TEST_CPU) {
    $Cpu = [int]$env:TEST_CPU
} else {
    try {
        $Cpu = (Get-CimInstance Win32_Processor |
            Measure-Object -Property NumberOfCores -Sum).Sum
    } catch { $Cpu = 0 }
}

if ($env:TEST_RAM) {
    $RamGB = [int]$env:TEST_RAM
} else {
    try {
        $RamGB = [math]::Round(
            (Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory / 1GB, 0)
    } catch { $RamGB = 0 }
}

if ($env:TEST_GPU) {
    $Gpu = if ($env:TEST_GPU -eq "none") { "none" } else { "nvidia" }
} else {
    try {
        $controllers = Get-CimInstance Win32_VideoController
        if ($controllers | Where-Object { $_.Name -match "NVIDIA" }) { $Gpu = "nvidia" }
    } catch { }
}

if ($Cpu -ge 8 -and $RamGB -ge 16 -and $Gpu -eq "nvidia") { $IsHost = $true }

Log ("Detected: CPU={0} cores, RAM={1}GB, GPU={2}" -f $Cpu, $RamGB, $(if ($Gpu -eq "nvidia") { "yes" } else { "no" }))
if ($IsHost) {
    Log "Profile: HOST hardware — but K3s agents are Linux-only, so this laptop enrolls as a browser Guest."
} else {
    Log "Profile: GUEST (browser-only thin client)."
}

# --- Find the control panel ---------------------------------------------
$PanelUrl = if ($env:PANEL_URL) { $env:PANEL_URL.TrimEnd("/") } else { "" }
if (-not $PanelUrl) {
    $PanelUrl = Read-Host "[joining-cluster] Master panel URL (ask the instructor, e.g. http://192.168.4.1:8082)"
    if (-not $PanelUrl) { Die "no panel URL given" }
    $PanelUrl = $PanelUrl.TrimEnd("/")
}

$Hostname = $env:COMPUTERNAME
if (-not $Hostname) { $Hostname = "laptop" }

$Payload = @{
    hostname = $Hostname
    os       = $Os
    arch     = $Arch
    cpu      = $Cpu
    ram_gb   = $RamGB
    gpu      = $Gpu
    join_key = $(if ($env:JOIN_KEY) { $env:JOIN_KEY } else { "" })
} | ConvertTo-Json -Compress

# --- Register (retries; server-side idempotency makes re-runs safe) -----
$NodeId = $null
foreach ($attempt in 1..3) {
    try {
        $resp = Invoke-RestMethod -Method Post -Uri "$PanelUrl/api/nodes/register" `
            -ContentType "application/json" -Body $Payload -TimeoutSec 10
        $NodeId = $resp.id
        break
    } catch {
        $code = try { $_.Exception.Response.StatusCode.value__ } catch { "network" }
        if ($code -eq 403) { Die "join key rejected — ask the instructor for the JOIN_KEY value" }
        Log ("register attempt {0} failed: {1}" -f $attempt, $_.Exception.Message)
        Start-Sleep -Seconds ($attempt * 2)
    }
}
if (-not $NodeId) { Die "could not register after 3 attempts — check PANEL_URL ($PanelUrl)" }
Log "Registered as node $NodeId (status: pending)."

# --- Wait for the instructor's decision ---------------------------------
function Wait-Approval {
    $deadline = (Get-Date).AddSeconds($EnrollTimeout)
    while ((Get-Date) -lt $deadline) {
        try {
            $node = Invoke-RestMethod -Uri "$PanelUrl/api/nodes/$NodeId" -TimeoutSec 10
        } catch {
            Start-Sleep -Seconds 5
            continue
        }
        switch ($node.status) {
            "pending" {
                Log "Waiting for instructor approval… (Ctrl-C keeps the request pending)"
                Start-Sleep -Seconds 5
            }
            "approved" { return $node }
            "joined" { return $node }
            "denied" {
                Log ("DENIED by the instructor: {0}" -f $(if ($node.reason) { $node.reason } else { "no reason given" }))
                return $null
            }
            "expired" {
                Log "Request expired — run this script again to re-request access."
                return $null
            }
            default { Start-Sleep -Seconds 5 }
        }
    }
    Log "Timed out after ${EnrollTimeout}s — your request stays pending; re-run this script to check again."
    return $null
}

$node = Wait-Approval
if (-not $node) { exit 1 }
Log "APPROVED! "

# --- Switch to the cluster network --------------------------------------
$Ssid = $env:CLUSTER_SSID
if (-not $Ssid) { $Ssid = Read-Host "[joining-cluster] Cluster Wi-Fi name (Enter to skip and switch manually)" }
if ($Ssid) {
    $connected = $false
    try {
        $iface = netsh wlan show interfaces | Select-String "^\s*Name\s*:\s*(.+)$"
        if ($iface) {
            $name = $iface.Matches[0].Groups[1].Value.Trim()
            netsh wlan connect "name=$name" "ssid=$Ssid" | Out-Null
            Start-Sleep -Seconds 4
            $current = (netsh wlan show interfaces | Select-String "^\s*SSID\s*:\s*(.+)$")
            if ($current -and $current.Matches[0].Groups[1].Value.Trim() -eq $Ssid) { $connected = $true }
        }
    } catch { }
    if (-not $connected) {
        Log "Could not switch automatically — connect to '$Ssid' manually (Wi-Fi menu)."
    } else {
        Log "Connected to '$Ssid'."
    }
} else {
    Log "Manual step: switch Wi-Fi to the cluster network now (ask the instructor for its name/password)."
}

Log "Done. Once on the cluster network, open http://control.ros-platform.local"
