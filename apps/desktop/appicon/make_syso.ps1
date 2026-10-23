# SPDX-License-Identifier: MPL-2.0
# Rebuilds ../rsrc_windows_amd64.syso, the executable's icon resource, from icon.ico. Go links a
# .syso in the main package into the Windows build. Run after changing the icon; needs MinGW
# windres on PATH.
$ErrorActionPreference = 'Stop'
Push-Location -LiteralPath $PSScriptRoot
try {
    & windres --target=pe-x86-64 -i icon.rc -O coff -o ../rsrc_windows_amd64.syso
    if ($LASTEXITCODE -ne 0) { throw "windres failed with exit code $LASTEXITCODE" }
} finally {
    Pop-Location
}
