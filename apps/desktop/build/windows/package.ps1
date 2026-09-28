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
$fltkSource = $env:ANIMEPORTABLE_FLTK_SOURCE
$fltkPatch = $env:ANIMEPORTABLE_FLTK_PATCH
$fltkSourceHash = '7715e69ce081fa9ce6da48bb0dd3b07a4cf2cf937813814c04272f36fff593ea'
$fltkPatchHash = '44688325d5029586a406609c6f6afc0ca3e3bd6ae117e2fef886e88ef6f3a4f6'
if ([string]::IsNullOrWhiteSpace($dll) -or [string]::IsNullOrWhiteSpace($digest) -or [string]::IsNullOrWhiteSpace($license) -or [string]::IsNullOrWhiteSpace($manifest) -or [string]::IsNullOrWhiteSpace($sources) -or [string]::IsNullOrWhiteSpace($fltkSource) -or [string]::IsNullOrWhiteSpace($fltkPatch)) {
    throw 'Set ANIMEPORTABLE_LIBMPV_DLL, ANIMEPORTABLE_LIBMPV_SHA256, ANIMEPORTABLE_LIBMPV_LICENSE, ANIMEPORTABLE_LIBMPV_MANIFEST, ANIMEPORTABLE_LIBMPV_SOURCES, ANIMEPORTABLE_FLTK_SOURCE and ANIMEPORTABLE_FLTK_PATCH before Windows packaging'
}
if ((Get-FileHash -LiteralPath $fltkSource -Algorithm SHA256).Hash -ne $fltkSourceHash) { throw 'FLTK 1.4.5 source archive SHA-256 mismatch' }
if ((Get-FileHash -LiteralPath $fltkPatch -Algorithm SHA256).Hash -ne $fltkPatchHash) { throw 'go-fltk Windows patch SHA-256 mismatch' }
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
        '--libmpv-manifest', $manifest, '--libmpv-sources', $sources,
        '--fltk-source', $fltkSource, '--fltk-source-sha256', $fltkSourceHash,
        '--fltk-patch', $fltkPatch, '--fltk-patch-sha256', $fltkPatchHash
    )
    & go @arguments
    if ($LASTEXITCODE -ne 0) { throw "Windows packaging failed with exit code $LASTEXITCODE" }
} finally {
    Remove-Item -LiteralPath $notices -ErrorAction SilentlyContinue
}
