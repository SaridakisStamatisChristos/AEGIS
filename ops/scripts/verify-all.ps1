$ErrorActionPreference = 'Stop'

$root = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$timestamp = (Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssZ")
$outDir = Join-Path $root "artifacts\verification\$timestamp"
$summaryFile = Join-Path $outDir "summary.txt"

New-Item -ItemType Directory -Force -Path $outDir | Out-Null

function Run-Step {
    param(
        [string]$Name,
        [string]$Command
    )

    $logFile = Join-Path $outDir "$Name.log"
    Add-Content -Path $summaryFile -Value "[START] $Name"
    Add-Content -Path $summaryFile -Value "[CMD] $Command"

    $wrapped = "Set-Location '$root'; $Command"
    $stdoutFile = Join-Path $outDir "$Name.stdout.log"
    $stderrFile = Join-Path $outDir "$Name.stderr.log"
    $proc = Start-Process -FilePath "powershell.exe" -ArgumentList "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", $wrapped -NoNewWindow -Wait -PassThru -RedirectStandardOutput $stdoutFile -RedirectStandardError $stderrFile

    if (Test-Path $stdoutFile) { Get-Content $stdoutFile | Out-File -FilePath $logFile -Encoding utf8 }
    if (Test-Path $stderrFile) { Get-Content $stderrFile | Add-Content -Path $logFile }

    if ($proc.ExitCode -ne 0) {
        Add-Content -Path $summaryFile -Value "[FAIL] $Name"
        Add-Content -Path $summaryFile -Value "Log: $logFile"
        throw "Step failed: $Name"
    }

    Add-Content -Path $summaryFile -Value "[PASS] $Name"
    Add-Content -Path $summaryFile -Value "Log: $logFile"
    Add-Content -Path $summaryFile -Value ""
}

@(
    "AegisRun verification evidence",
    "Timestamp (UTC): $timestamp",
    "Repository: $root",
    ""
) | Out-File -FilePath $summaryFile -Encoding utf8

Run-Step -Name "api-tests" -Command "Set-Location api; go test ./..."
Run-Step -Name "verifier-tests" -Command "Set-Location verifier; go test ./..."
Run-Step -Name "ui-tests" -Command "Set-Location ui; npm run test -- --run"
Run-Step -Name "typescript-sdk-tests" -Command "Set-Location sdk/typescript; npm test -- --run"

Add-Content -Path $summaryFile -Value "All verification checks passed."
Add-Content -Path $summaryFile -Value "Evidence directory: $outDir"

Write-Output "All verification checks passed."
Write-Output "Evidence directory: $outDir"
