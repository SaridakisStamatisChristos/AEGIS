param(
    [switch]$Json,
    [switch]$Ci
)

$ErrorActionPreference = 'Stop'

foreach ($arg in $args) {
    switch ($arg) {
        '--json' { $Json = $true }
        '--ci' { $Ci = $true }
    }
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Resolve-Path (Join-Path $scriptDir "../..")
Set-Location $repoRoot

$dimensions = @('Security', 'Reliability', 'Operability', 'Quality', 'Governance')
$scores = @{}
$maxScores = @{}
$details = @{}

foreach ($d in $dimensions) {
    $scores[$d] = 0
    $maxScores[$d] = 20
    $details[$d] = New-Object System.Collections.Generic.List[string]
}

function Award([string]$dim, [int]$points, [string]$reason) {
    $scores[$dim] = [int]$scores[$dim] + $points
    $details[$dim].Add("  +$points`: $reason")
}

function Deduct([string]$dim, [string]$reason) {
    $details[$dim].Add("  -0: $reason (gap)")
}

function FileExists([string]$path) {
    return Test-Path -Path $path -PathType Leaf
}

function DirExists([string]$path) {
    return Test-Path -Path $path -PathType Container
}

function HasText([string]$path, [string]$pattern) {
    if (-not (Test-Path -Path $path)) {
        return $false
    }
    return Select-String -Path $path -Pattern $pattern -Quiet
}

function HasAnyTestFiles([string]$pattern) {
    return (Get-ChildItem -Path $pattern -ErrorAction SilentlyContinue | Measure-Object).Count -gt 0
}

# Security
if (FileExists '.github/workflows/security-scan.yml') {
    Award 'Security' 4 'Security scan CI workflow present'
} else {
    Deduct 'Security' 'No security-scan.yml workflow'
}

if (HasText 'api/internal/auth/oidc.go' 'ErrMockProviderInProduction') {
    Award 'Security' 4 'Mock OIDC production guard implemented'
} else {
    Deduct 'Security' 'Mock OIDC production guard missing'
}

if ((HasText 'api/internal/auth/oidc.go' 'MaxTokenAge') -and (HasText 'api/internal/auth/oidc.go' 'Audience')) {
    Award 'Security' 4 'Token validation (issuer/audience/age) implemented'
} else {
    Deduct 'Security' 'Token validation incomplete'
}

$composeHasVars = HasText 'docker-compose.yml' '\$\{'
$composeHasHardcodedPassword = HasText 'docker-compose.yml' '(?i)password:\s*["''][a-zA-Z]'
if ($composeHasVars -and -not $composeHasHardcodedPassword) {
    Award 'Security' 4 'No hardcoded credentials in docker-compose.yml'
} else {
    Deduct 'Security' 'Hardcoded credentials found in docker-compose.yml'
}

if (FileExists 'ops/k8s/network-policy.yaml') {
    Award 'Security' 4 'Kubernetes NetworkPolicy defined'
} else {
    Deduct 'Security' 'No NetworkPolicy manifest'
}

# Reliability
if (FileExists 'docs/BACKUP_RESTORE_RUNBOOK.md') {
    Award 'Reliability' 4 'Backup/restore runbook present'
} else {
    Deduct 'Reliability' 'No backup/restore runbook'
}

if (FileExists 'ops/prometheus/rules/aegisrun-alerts.yml') {
    Award 'Reliability' 4 'Prometheus alert rules defined'
} else {
    Deduct 'Reliability' 'No Prometheus alert rules'
}

if (FileExists 'ops/grafana/dashboards/slo-reliability-dashboard.json') {
    Award 'Reliability' 4 'Grafana SLO dashboard present'
} else {
    Deduct 'Reliability' 'No SLO dashboard'
}

if (FileExists 'docs/ROLLBACK_PLAYBOOK.md') {
    Award 'Reliability' 4 'Deploy rollback playbook present'
} else {
    Deduct 'Reliability' 'No rollback playbook'
}

if (FileExists 'ops/scripts/backup-postgres.sh') {
    Award 'Reliability' 4 'Backup/restore script present'
} else {
    Deduct 'Reliability' 'No backup script'
}

# Operability
if (FileExists 'api/internal/telemetry/metrics.go') {
    Award 'Operability' 4 'Prometheus metrics instrumentation present'
} else {
    Deduct 'Operability' 'No metrics instrumentation'
}

if ((HasText 'ops/k8s/api-deployment.yaml' 'readinessProbe') -and (HasText 'ops/k8s/api-deployment.yaml' 'livenessProbe')) {
    Award 'Operability' 4 'Liveness and readiness probes configured'
} else {
    Deduct 'Operability' 'Probes missing in deployment'
}

if (FileExists 'ops/k8s/api-hpa.yaml') {
    Award 'Operability' 4 'HorizontalPodAutoscaler configured'
} else {
    Deduct 'Operability' 'No HPA manifest'
}

if (FileExists 'docs/GAMEDAY_DRILL_TEMPLATE.md') {
    Award 'Operability' 4 'Game-day drill template present'
} else {
    Deduct 'Operability' 'No game-day drill template'
}

if (FileExists '.github/workflows/deploy.yml') {
    Award 'Operability' 4 'Automated deploy workflow present'
} else {
    Deduct 'Operability' 'No deploy workflow'
}

# Quality
if ((DirExists 'api/internal/auth') -and (HasAnyTestFiles 'api/internal/auth/*_test.go')) {
    Award 'Quality' 4 'API auth tests present'
} else {
    Deduct 'Quality' 'No API auth tests'
}

if (FileExists 'tests/e2e/full_flow_test.go') {
    Award 'Quality' 4 'E2E test suite present'
} else {
    Deduct 'Quality' 'No E2E tests'
}

if (FileExists 'tests/load/locustfile.py') {
    Award 'Quality' 4 'Load test suite present'
} else {
    Deduct 'Quality' 'No load tests'
}

$sdkPts = 0
if (DirExists 'sdk/python/tests') { $sdkPts += 2 }
if (DirExists 'sdk/typescript/tests') { $sdkPts += 2 }
if ($sdkPts -gt 0) {
    Award 'Quality' $sdkPts "SDK tests present ($sdkPts/4)"
}

if (FileExists 'api/cmd/server/config_test.go') {
    Award 'Quality' 4 'Config validation tests present'
} else {
    Deduct 'Quality' 'No config tests'
}

# Governance
if (FileExists '.github/workflows/release-gate.yml') {
    Award 'Governance' 4 'Release gate CI workflow present'
} else {
    Deduct 'Governance' 'No release-gate workflow'
}

if (FileExists 'docs/RELEASE_CHECKLIST.md') {
    Award 'Governance' 4 'Signed release checklist template present'
} else {
    Deduct 'Governance' 'No release checklist'
}

if ((FileExists 'ops/k8s/api-canary-deployment.yaml') -and (FileExists 'ops/scripts/canary-deploy.sh')) {
    Award 'Governance' 4 'Canary deployment infrastructure present'
} else {
    Deduct 'Governance' 'No canary deployment setup'
}

if (FileExists 'ops/scripts/release-freeze.sh') {
    Award 'Governance' 4 'Release freeze script present'
} else {
    Deduct 'Governance' 'No release freeze script'
}

if ((FileExists 'PRODUCTION_ROADMAP.md') -and (HasText 'PRODUCTION_ROADMAP.md' 'COMPLETED')) {
    Award 'Governance' 4 'Production roadmap actively tracked'
} else {
    Deduct 'Governance' 'Production roadmap not tracked'
}

$total = 0
foreach ($d in $dimensions) {
    $total += [int]$scores[$d]
}
$maxTotal = 100
$passed = $total -ge 90

if ($Json) {
    $payload = [ordered]@{
        total_score = $total
        max_score = $maxTotal
        pass = $passed
        dimensions = [ordered]@{
            security    = [ordered]@{ score = [int]$scores['Security']; max = [int]$maxScores['Security'] }
            reliability = [ordered]@{ score = [int]$scores['Reliability']; max = [int]$maxScores['Reliability'] }
            operability = [ordered]@{ score = [int]$scores['Operability']; max = [int]$maxScores['Operability'] }
            quality     = [ordered]@{ score = [int]$scores['Quality']; max = [int]$maxScores['Quality'] }
            governance  = [ordered]@{ score = [int]$scores['Governance']; max = [int]$maxScores['Governance'] }
        }
        timestamp = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
    }
    $payload | ConvertTo-Json -Depth 6
} else {
    Write-Host ""
    Write-Host "============================================================" -ForegroundColor Blue
    Write-Host "        AegisRun Production Readiness Score                " -ForegroundColor Blue
    Write-Host "============================================================" -ForegroundColor Blue
    Write-Host ""

    foreach ($d in $dimensions) {
        $score = [int]$scores[$d]
        $max = [int]$maxScores[$d]
        $color = 'Green'
        if ($score -lt [math]::Floor($max * 0.8)) { $color = 'Yellow' }
        if ($score -lt [math]::Floor($max * 0.6)) { $color = 'Red' }

        Write-Host ("  {0,-20} {1,2} / {2,2}" -f $d, $score, $max) -ForegroundColor $color
        foreach ($line in $details[$d]) {
            Write-Host $line
        }
    }

    Write-Host "  --------------------------------"
    $totalColor = 'Green'
    if ($total -lt 90) { $totalColor = 'Yellow' }
    if ($total -lt 70) { $totalColor = 'Red' }
    Write-Host ("  {0,-20} {1,2} / {2,2}" -f 'TOTAL', $total, $maxTotal) -ForegroundColor $totalColor
    Write-Host ""

    if ($passed) {
        Write-Host "  PRODUCTION READY (score >= 90)" -ForegroundColor Green
    } else {
        Write-Host ("  NOT YET PRODUCTION READY (score < 90, need {0} more points)" -f ([int](90 - $total))) -ForegroundColor Yellow
    }
    Write-Host ""
}

if ($Ci -and -not $passed) {
    exit 1
}
