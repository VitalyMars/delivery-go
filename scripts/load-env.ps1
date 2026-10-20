$ErrorActionPreference = 'Stop'
$envFile = Join-Path (Split-Path -Parent $PSScriptRoot) '.env'
if (-not (Test-Path -LiteralPath $envFile)) { throw 'Create .env with scripts/setup-env.ps1 first.' }
foreach ($line in [IO.File]::ReadAllLines($envFile)) {
    if ($line -match '^(AUTH_SECRET|INTERNAL_API_KEY|DEMO_PAYMENTS)=(.*)$') {
        [Environment]::SetEnvironmentVariable($Matches[1], $Matches[2], 'Process')
    }
}
Write-Host 'Loaded local application settings into this terminal.'
