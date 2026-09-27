param(
    [Parameter(Mandatory=$true)][string]$Arch,
    [Parameter(Mandatory=$true)][string]$Binary,
    [Parameter(Mandatory=$true)][string]$Output
)

$ErrorActionPreference = 'Stop'
$dll = $env:ANIMEPORTABLE_LIBMPV_DLL
$digest = $env:ANIMEPORTABLE_LIBMPV_SHA256
$license = $env:ANIMEPORTABLE_LIBMPV_LICENSE
if ([string]::IsNullOrWhiteSpace($dll) -or [string]::IsNullOrWhiteSpace($digest) -or [string]::IsNullOrWhiteSpace($license)) {
    throw 'Set ANIMEPORTABLE_LIBMPV_DLL, ANIMEPORTABLE_LIBMPV_SHA256 and ANIMEPORTABLE_LIBMPV_LICENSE before Windows packaging'
}
$arguments = @(
    'run', '../../tools/portable-package',
    '--os', 'windows', '--arch', $Arch,
    '--binary', $Binary, '--output', $Output,
    '--license', '../../LICENSE', '--notices', '../../THIRD_PARTY_NOTICES.md',
    '--libmpv', $dll, '--libmpv-sha256', $digest, '--libmpv-license', $license
)
& go @arguments
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
