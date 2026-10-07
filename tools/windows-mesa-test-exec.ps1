$ErrorActionPreference = 'Stop'
if ($env:GITHUB_ACTIONS -ne 'true' -or $args.Count -lt 1) { throw 'Isolated GitHub test execution required' }
$testExecutable = (Resolve-Path -LiteralPath $args[0]).Path
if (-not $testExecutable.EndsWith('.test.exe', [StringComparison]::OrdinalIgnoreCase)) { throw 'Go test executable required' }
$allowedRoots = @($env:RUNNER_TEMP, $env:TEMP) | Where-Object { $_ } | ForEach-Object { [IO.Path]::GetFullPath($_).TrimEnd('\') + '\' }
if (-not ($allowedRoots | Where-Object { $testExecutable.StartsWith($_, [StringComparison]::OrdinalIgnoreCase) })) { throw 'Go test executable must be inside runner temporary storage' }
$mesaDirectory = (Resolve-Path -LiteralPath $env:ANIMEPORTABLE_TEST_MESA_BIN).Path
$testDirectory = Split-Path -Parent $testExecutable
foreach ($name in @('opengl32.dll', 'libgallium_wgl.dll')) {
    $source = Join-Path $mesaDirectory $name
    if (-not (Test-Path -LiteralPath $source -PathType Leaf)) { throw "Mesa test dependency missing: $name" }
    Copy-Item -LiteralPath $source -Destination (Join-Path $testDirectory $name)
}
$env:PATH = $mesaDirectory + [IO.Path]::PathSeparator + $env:PATH
$env:GALLIUM_DRIVER = 'llvmpipe'
$testArguments = @()
if ($args.Count -gt 1) { $testArguments = $args[1..($args.Count - 1)] }
& $testExecutable @testArguments
exit $LASTEXITCODE
