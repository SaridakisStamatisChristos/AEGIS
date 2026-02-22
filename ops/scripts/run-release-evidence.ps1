param(
    [Parameter(Mandatory = $true)]
    [string]$Version,

    [Parameter(Mandatory = $true)]
    [string]$ReleaseBranch,

    [string]$EvidenceDir,

    [ValidateSet('smoke', 'baseline', 'stress', 'soak')]
    [string]$LoadScenario = 'baseline',

    [switch]$SkipE2E,

    [switch]$SkipTagPush
)

$ErrorActionPreference = 'Stop'

$script:GhExe = $null

function Require-Command([string]$Name) {
    $cmd = Get-Command $Name -ErrorAction SilentlyContinue
    if ($cmd) {
        return $cmd.Source
    }

    if ($Name -eq 'gh') {
        $fallback = 'C:\Program Files\GitHub CLI\gh.exe'
        if (Test-Path $fallback) {
            return $fallback
        }
    }

    if ($Name -eq 'git') {
        $gitFallbacks = @(
            'C:\Program Files\Git\cmd\git.exe',
            'C:\Program Files\Git\bin\git.exe'
        )
        foreach ($fallback in $gitFallbacks) {
            if (Test-Path $fallback) {
                return $fallback
            }
        }
    }

    throw "Required command not found: $Name"
}

function Invoke-Gh([string[]]$Args) {
    & $script:GhExe @Args
}

function Run-GhJson([string]$Args) {
    $raw = Invoke-Gh ($Args -split ' ')
    return $raw | ConvertFrom-Json
}

function Get-RepoRoot {
    return (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
}

function Resolve-EvidenceDir([string]$RepoRoot, [string]$InputDir) {
    if ($InputDir -and $InputDir.Trim() -ne '') {
        return (Resolve-Path $InputDir).Path
    }

    $releasesRoot = Join-Path $RepoRoot 'artifacts/releases'
    if (-not (Test-Path $releasesRoot)) {
        throw "Evidence directory not found and -EvidenceDir not provided: $releasesRoot"
    }

    $latest = Get-ChildItem -Path $releasesRoot -Directory |
        Sort-Object LastWriteTimeUtc -Descending |
        Select-Object -First 1

    if (-not $latest) {
        throw "No release evidence directories found under: $releasesRoot"
    }

    return $latest.FullName
}

function Get-WorkflowRun([string]$Workflow, [string]$Branch) {
    $runs = Run-GhJson "run list --workflow $Workflow --branch $Branch --limit 1 --json databaseId,url,status,conclusion,createdAt,updatedAt"
    if (-not $runs -or $runs.Count -eq 0) {
        throw "No run found for workflow '$Workflow' on branch '$Branch'"
    }
    return $runs[0]
}

function Update-EvidenceSection {
    param(
        [string]$EvidencePath,
        [string]$Section,
        [pscustomobject]$Run
    )

    $content = Get-Content -Path $EvidencePath -Raw

    if ($Section -eq 'gate') {
        $content = $content -replace '(?m)^- Run URL:.*$', "- Run URL: $($Run.url)"
        $content = $content -replace '(?m)^- Run ID:.*$', "- Run ID: $($Run.databaseId)"
        $content = $content -replace '(?m)^- Completed At \(UTC\):.*$', "- Completed At (UTC): $($Run.updatedAt)"
        $content = $content -replace '(?m)^- Final Status:.*$', "- Final Status: $($Run.conclusion)"
    }

    if ($Section -eq 'release') {
        $marker = '## 2) Tag Release Workflow (`release.yml`)'
        $idx = $content.IndexOf($marker)
        if ($idx -lt 0) {
            throw "Tag Release section marker not found in evidence file"
        }

        $prefix = $content.Substring(0, $idx)
        $suffix = $content.Substring($idx)

        $suffix = $suffix -replace '(?m)^- Run URL:.*$', "- Run URL: $($Run.url)"
        $suffix = $suffix -replace '(?m)^- Run ID:.*$', "- Run ID: $($Run.databaseId)"
        $suffix = $suffix -replace '(?m)^- Completed At \(UTC\):.*$', "- Completed At (UTC): $($Run.updatedAt)"
        $suffix = $suffix -replace '(?m)^- Final Status:.*$', "- Final Status: $($Run.conclusion)"

        $content = $prefix + $suffix
    }

    Set-Content -Path $EvidencePath -Value $content -Encoding utf8
}

$repoRoot = Get-RepoRoot
Set-Location $repoRoot
$env:GH_PROMPT_DISABLED = '1'

$script:GhExe = Require-Command 'gh'
$null = Require-Command 'git'

$authStatus = Invoke-Gh @('auth', 'status') 2>&1
if ($LASTEXITCODE -ne 0) {
    throw "GitHub CLI is not authenticated. Run: gh auth login"
}

$evidenceDirResolved = Resolve-EvidenceDir -RepoRoot $repoRoot -InputDir $EvidenceDir
$evidencePath = Join-Path $evidenceDirResolved 'release-evidence.md'
if (-not (Test-Path $evidencePath)) {
    throw "Evidence file not found: $evidencePath"
}

Write-Output "Using evidence directory: $evidenceDirResolved"

# Trigger release-gate workflow via workflow_dispatch
$skipE2EVal = if ($SkipE2E) { 'true' } else { 'false' }
Write-Output "Triggering release-gate.yml on $ReleaseBranch (load=$LoadScenario skip_e2e=$skipE2EVal)..."
Invoke-Gh @('workflow', 'run', 'release-gate.yml', '--ref', $ReleaseBranch, '-f', "run_load_tests=$LoadScenario", '-f', "skip_e2e=$skipE2EVal")
if ($LASTEXITCODE -ne 0) {
    throw 'Failed to trigger release-gate.yml'
}

Start-Sleep -Seconds 5
$gateRun = Get-WorkflowRun -Workflow 'release-gate.yml' -Branch $ReleaseBranch
Write-Output "Watching release-gate run: $($gateRun.databaseId)"
Invoke-Gh @('run', 'watch', "$($gateRun.databaseId)", '--exit-status')
if ($LASTEXITCODE -ne 0) {
    throw "release-gate run failed: $($gateRun.url)"
}

$gateRunFinal = Get-WorkflowRun -Workflow 'release-gate.yml' -Branch $ReleaseBranch
Update-EvidenceSection -EvidencePath $evidencePath -Section 'gate' -Run $gateRunFinal

if (-not $SkipTagPush) {
    # Trigger release workflow by pushing tag
    Write-Output "Creating and pushing tag: $Version"
    git tag $Version
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to create tag '$Version' (it may already exist)."
    }

    git push origin $Version
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to push tag '$Version'"
    }

    Start-Sleep -Seconds 8
    $releaseRun = Run-GhJson "run list --workflow release.yml --limit 1 --json databaseId,url,status,conclusion,createdAt,updatedAt"
    if (-not $releaseRun -or $releaseRun.Count -eq 0) {
        throw 'No release.yml run found after pushing tag.'
    }

    $releaseRun = $releaseRun[0]
    Write-Output "Watching release run: $($releaseRun.databaseId)"
    Invoke-Gh @('run', 'watch', "$($releaseRun.databaseId)", '--exit-status')
    if ($LASTEXITCODE -ne 0) {
        throw "release.yml run failed: $($releaseRun.url)"
    }

    $releaseRunFinal = (Run-GhJson "run list --workflow release.yml --limit 1 --json databaseId,url,status,conclusion,createdAt,updatedAt")[0]
    Update-EvidenceSection -EvidencePath $evidencePath -Section 'release' -Run $releaseRunFinal
}

Write-Output "Release evidence execution complete."
Write-Output "Evidence file updated: $evidencePath"
