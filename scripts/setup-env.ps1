$ErrorActionPreference = 'Stop'
$taskRoot = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $taskRoot '.env'
if (Test-Path -LiteralPath $envFile) { throw '.env already exists; existing secrets were preserved.' }
function New-Secret {
    $bytes = New-Object byte[] 32
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($bytes) } finally { $rng.Dispose() }
    return [Convert]::ToBase64String($bytes)
}
$content = @("AUTH_SECRET=$(New-Secret)", "INTERNAL_API_KEY=$(New-Secret)", 'DEMO_PAYMENTS=true')
[IO.File]::WriteAllLines($envFile, $content, (New-Object Text.UTF8Encoding $false))
Write-Host 'Created .env with unique local secrets. Docker Compose reads this file automatically.'
