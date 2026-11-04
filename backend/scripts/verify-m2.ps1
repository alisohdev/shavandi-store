[CmdletBinding()]
param(
    [string] $BunExe,
    [string] $PlaywrightChannel
)

$ErrorActionPreference = 'Stop'
$backendRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$repoRoot = (Resolve-Path (Join-Path $backendRoot '..')).Path
$envFile = Join-Path $backendRoot '.env'

if (-not (Test-Path -LiteralPath $envFile -PathType Leaf)) {
    throw 'backend/.env is required for the PostgreSQL integration tests.'
}

foreach ($line in Get-Content -LiteralPath $envFile) {
    if ($line.TrimStart().StartsWith('#')) {
        continue
    }
    $match = [regex]::Match($line, '^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*?)\s*$')
    if (-not $match.Success) {
        continue
    }
    $name = $match.Groups[1].Value
    $value = $match.Groups[2].Value
    if ($value.Length -ge 2 -and (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'")))) {
        $value = $value.Substring(1, $value.Length - 2)
    }
    [Environment]::SetEnvironmentVariable($name, $value, 'Process')
}

foreach ($name in @('DATABASE_URL', 'TEST_DATABASE_URL')) {
    $value = [Environment]::GetEnvironmentVariable($name, 'Process')
    if ([string]::IsNullOrWhiteSpace($value)) {
        throw "backend/.env must define a non-empty $name."
    }
}

$env:GOSUMDB = 'sum.golang.org'
if ($PlaywrightChannel) {
    $env:PLAYWRIGHT_CHANNEL = $PlaywrightChannel
}
if ($BunExe) {
    $resolvedBunExe = (Resolve-Path -LiteralPath $BunExe).Path
    if (-not (Test-Path -LiteralPath $resolvedBunExe -PathType Leaf)) {
        throw 'BunExe must point to a Bun executable.'
    }
    $env:PATH = (Split-Path -Parent $resolvedBunExe) + [IO.Path]::PathSeparator + $env:PATH
}

function Invoke-Checked {
    param(
        [Parameter(Mandatory = $true)][string] $Executable,
        [Parameter(Mandatory = $true)][string[]] $Arguments
    )
    Write-Host ('Running: ' + $Executable + ' ' + ($Arguments -join ' '))
    & $Executable @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$Executable failed with exit code $LASTEXITCODE."
    }
}

$go = Get-Command go -ErrorAction Stop
$bun = Get-Command bun -ErrorAction Stop

Push-Location $backendRoot
try {
    Invoke-Checked $go.Source @('run', 'github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1', 'generate')
    Invoke-Checked $go.Source @('test', './...')
    Invoke-Checked $go.Source @('test', '-tags=integration', './...', '-count=1')
    Invoke-Checked $go.Source @('vet', './...')
    Invoke-Checked $go.Source @('build', './...')
}
finally {
    Pop-Location
}

Push-Location $repoRoot
try {
    & $bun.Source lint --help *> $null
    if ($LASTEXITCODE -ne 0) {
        throw 'Bun must provide the native lint command documented in README.md.'
    }
    & $bun.Source fmt --help *> $null
    if ($LASTEXITCODE -ne 0) {
        throw 'Bun must provide the native fmt command documented in README.md.'
    }
    Invoke-Checked $bun.Source @('run', 'validate')
    Invoke-Checked $bun.Source @('run', 'test:e2e')
}
finally {
    Pop-Location
}
