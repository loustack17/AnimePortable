# SPDX-License-Identifier: MPL-2.0
param(
    [Parameter(Mandatory = $true)]
    [string]$Output
)

$ErrorActionPreference = 'Stop'
if (Test-Path -LiteralPath $Output) {
    throw "Output already exists: $Output"
}

$records = @(go list -deps -tags production -f '{{if .Module}}{{.Module.Path}}|{{.Module.Version}}|{{.Module.Dir}}{{end}}' ./apps/desktop)
if ($LASTEXITCODE -ne 0) {
    throw 'Cannot determine Windows desktop Go dependencies'
}

$builder = [Text.StringBuilder]::new()
$base = [IO.File]::ReadAllText((Join-Path (Get-Location) 'THIRD_PARTY_NOTICES.md')).Replace("`r`n", "`n")
[void]$builder.AppendLine($base.TrimEnd())
[void]$builder.AppendLine()
[void]$builder.AppendLine('## Linked Go module license texts')

foreach ($record in @($records | Where-Object { $_ -and -not $_.StartsWith('animeportable|') } | Sort-Object -Unique)) {
    $parts = $record -split '\|', 3
    if ($parts.Count -ne 3 -or -not $parts[1] -or -not (Test-Path -LiteralPath $parts[2] -PathType Container)) {
        throw "Incomplete module record: $record"
    }
    $root = $parts[2]
    $rootPrefix = $root.TrimEnd('\', '/') + [IO.Path]::DirectorySeparatorChar
    $files = @(Get-ChildItem -LiteralPath $root -Recurse -File | Where-Object { $_.Name -match '^(LICENSE|LICENCE|COPYING|NOTICE)([-._].*)?$' } | Sort-Object FullName)
    if (-not @($files | Where-Object { $_.DirectoryName -eq $root }).Count) {
        throw "Module has no root license: $($parts[0])"
    }
    foreach ($file in $files) {
        if (($file.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw "Linked license file is a reparse point: $($file.FullName)"
        }
        if (-not $file.FullName.StartsWith($rootPrefix, [StringComparison]::OrdinalIgnoreCase)) {
            throw "License file is outside its module: $($file.FullName)"
        }
        $relative = $file.FullName.Substring($rootPrefix.Length).Replace('\', '/')
        [void]$builder.AppendLine()
        [void]$builder.AppendLine("### $($parts[0]) $($parts[1]) - $relative")
        [void]$builder.AppendLine()
        [void]$builder.AppendLine('```text')
        [void]$builder.AppendLine([IO.File]::ReadAllText($file.FullName).Replace("`r`n", "`n").TrimEnd())
        [void]$builder.AppendLine('```')
    }
}

$parent = [IO.Path]::GetDirectoryName([IO.Path]::GetFullPath($Output))
[IO.Directory]::CreateDirectory($parent) | Out-Null
$stream = [IO.File]::Open($Output, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)
try {
    $bytes = [Text.UTF8Encoding]::new($false).GetBytes($builder.ToString())
    $stream.Write($bytes, 0, $bytes.Length)
} finally {
    $stream.Dispose()
}
