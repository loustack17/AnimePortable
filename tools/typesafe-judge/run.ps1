# SPDX-License-Identifier: MPL-2.0
# Runs typesafe-judge with only the TYPESAFE_API_KEY secret injected from
# Bitwarden Secrets Manager into this process. The key is never written to disk
# or printed. If this fails, report ENVIRONMENT_BLOCKED for the judgment step.
param(
    [Parameter(Mandatory)] [string] $In,
    [Parameter(Mandatory)] [string] $Out,
    [string] $Questions = '',
    # Bitwarden Secrets Manager ID of TYPESAFE_API_KEY. An ID is not a secret;
    # it is useless without the owner's BWS_ACCESS_TOKEN.
    [string] $SecretId = $(if ($env:TYPESAFE_BWS_SECRET_ID) { $env:TYPESAFE_BWS_SECRET_ID } else { 'a422228e-c585-43d9-a7e6-b4db0165831f' })
)
$ErrorActionPreference = 'Stop'
$repo = Resolve-Path (Join-Path $PSScriptRoot '..\..')
# Resolve paths before changing directory so relative arguments keep working.
$In = (Resolve-Path $In).Path
$Out = [System.IO.Path]::GetFullPath($Out)
$goArgs = @('run', './tools/typesafe-judge', '-in', $In, '-out', $Out)
if ($Questions) { $goArgs += @('-questions', (Resolve-Path $Questions).Path) }

# Fetch only this one secret; never list the store.
$secret = bws secret get $SecretId -o json | ConvertFrom-Json
if ($LASTEXITCODE -ne 0 -or $secret.key -ne 'TYPESAFE_API_KEY' -or -not $secret.value) {
    throw "Secret $SecretId is not TYPESAFE_API_KEY or could not be read (check BWS_ACCESS_TOKEN). Report ENVIRONMENT_BLOCKED."
}
$env:TYPESAFE_API_KEY = $secret.value
$secret = $null
Push-Location $repo
try {
    & go @goArgs
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} finally {
    Pop-Location
    Remove-Item env:TYPESAFE_API_KEY -ErrorAction SilentlyContinue
}
