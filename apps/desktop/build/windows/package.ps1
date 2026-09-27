param(
    [Parameter(Mandatory=$true)][string]$Arch,
    [Parameter(Mandatory=$true)][string]$Binary,
    [Parameter(Mandatory=$true)][string]$Output
)

$ErrorActionPreference = 'Stop'
$dll = $env:ANIMEPORTABLE_LIBMPV_DLL
$digest = $env:ANIMEPORTABLE_LIBMPV_SHA256
$license = $env:ANIMEPORTABLE_LIBMPV_LICENSE
$manifest = $env:ANIMEPORTABLE_LIBMPV_MANIFEST
$sources = $env:ANIMEPORTABLE_LIBMPV_SOURCES
if ([string]::IsNullOrWhiteSpace($dll) -or [string]::IsNullOrWhiteSpace($digest) -or [string]::IsNullOrWhiteSpace($license) -or [string]::IsNullOrWhiteSpace($manifest) -or [string]::IsNullOrWhiteSpace($sources)) {
    throw 'Set ANIMEPORTABLE_LIBMPV_DLL, ANIMEPORTABLE_LIBMPV_SHA256, ANIMEPORTABLE_LIBMPV_LICENSE, ANIMEPORTABLE_LIBMPV_MANIFEST and ANIMEPORTABLE_LIBMPV_SOURCES before Windows packaging'
}
$notices = Join-Path ([IO.Path]::GetDirectoryName([IO.Path]::GetFullPath($Binary))) ("third-party-notices-$([guid]::NewGuid().ToString('N')).md")
try {
    Push-Location ../..
    try {
        & ./tools/build-go-license-notices.ps1 -Output $notices
    } finally {
        Pop-Location
    }
    $arguments = @(
        'run', '../../tools/portable-package',
        '--os', 'windows', '--arch', $Arch,
        '--binary', $Binary, '--output', $Output,
        '--license', '../../LICENSE', '--notices', $notices,
        '--libmpv', $dll, '--libmpv-sha256', $digest, '--libmpv-license', $license,
        '--libmpv-manifest', $manifest, '--libmpv-sources', $sources
    )
    & go @arguments
    if ($LASTEXITCODE -ne 0) { throw "Windows packaging failed with exit code $LASTEXITCODE" }
} finally {
    Remove-Item -LiteralPath $notices -ErrorAction SilentlyContinue
}
