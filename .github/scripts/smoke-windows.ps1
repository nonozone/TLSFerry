$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../..")).Path
$testRoot = Join-Path $env:RUNNER_TEMP ("tlsferry-windows-smoke-" + [Guid]::NewGuid().ToString("N"))
$binaryPath = Join-Path $testRoot "tlsferry.exe"
$originalAppData = $env:APPDATA
$serviceInstalled = $false

function Invoke-Checked {
    param(
        [Parameter(Mandatory = $true)][string]$Command,
        [Parameter(Mandatory = $true)][string[]]$Arguments
    )
    & $Command @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$Command exited with code $LASTEXITCODE"
    }
}

New-Item -ItemType Directory -Path $testRoot | Out-Null
$env:APPDATA = Join-Path $testRoot "AppData\Roaming"

try {
    Set-Location $repoRoot
    Write-Output ("Windows version: " + [System.Environment]::OSVersion.VersionString)
    Write-Output ("PowerShell version: " + $PSVersionTable.PSVersion.ToString())
    Invoke-Checked -Command "go" -Arguments @("build", "-trimpath", "-o", $binaryPath, "./cmd/tlsferry")
    Invoke-Checked -Command "go" -Arguments @("run", "./internal/releasetestfixture", "--root", $testRoot)

    $schedule = (Get-Date).AddMinutes(10)
    Invoke-Checked -Command $binaryPath -Arguments @(
        "service", "install",
        "--config", (Join-Path $testRoot "config.json"),
        "--state-dir", (Join-Path $testRoot "state"),
        "--output-dir", (Join-Path $testRoot "certificates"),
        "--hour", $schedule.Hour.ToString(),
        "--minute", $schedule.Minute.ToString(),
        "--accept-tos",
        "--execute"
    )
    $serviceInstalled = $true

    Invoke-Checked -Command $binaryPath -Arguments @("service", "status")
    $task = Get-ScheduledTask -TaskName "TLSFerry Renewal"
    $task | Select-Object TaskName, State, @{Name="LogonType";Expression={$_.Principal.LogonType}}, @{Name="RunLevel";Expression={$_.Principal.RunLevel}} | Format-List

    $startedAt = Get-Date
    Invoke-Checked -Command $binaryPath -Arguments @("service", "run-now")
    $deadline = $startedAt.AddSeconds(30)
    do {
        Start-Sleep -Seconds 1
        $task = Get-ScheduledTask -TaskName "TLSFerry Renewal"
        $taskInfo = Get-ScheduledTaskInfo -TaskName "TLSFerry Renewal"
        if ((Get-Date) -gt $deadline) {
            throw "Task Scheduler run did not finish within 30 seconds"
        }
    } while ($task.State -eq "Running" -or $taskInfo.LastRunTime -lt $startedAt.AddSeconds(-1))

    $taskInfo | Select-Object LastRunTime, LastTaskResult, NextRunTime | Format-List
    if ($taskInfo.LastTaskResult -ne 0) {
        throw "Task Scheduler last result was $($taskInfo.LastTaskResult), expected 0"
    }
    Invoke-Checked -Command $binaryPath -Arguments @("service", "logs")
    Invoke-Checked -Command "schtasks.exe" -Arguments @("/Query", "/TN", "TLSFerry Renewal", "/V", "/FO", "LIST")

    Invoke-Checked -Command $binaryPath -Arguments @("service", "uninstall")
    $serviceInstalled = $false
    Invoke-Checked -Command $binaryPath -Arguments @("service", "status")
    if (Get-ScheduledTask -TaskName "TLSFerry Renewal" -ErrorAction SilentlyContinue) {
        throw "Task Scheduler entry remains after uninstall"
    }
}
finally {
    if ($serviceInstalled -and (Test-Path $binaryPath)) {
        & $binaryPath service uninstall *> $null
    }
    $env:APPDATA = $originalAppData
    Set-Location $repoRoot
    Remove-Item -Recurse -Force $testRoot -ErrorAction SilentlyContinue
}
