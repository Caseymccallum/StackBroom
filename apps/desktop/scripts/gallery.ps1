# Captures the five real StackBroom screens in both themes at the target client
# sizes. Read-only: it only resizes, focuses and reads pixels.
param(
    [ValidateSet("dark", "light")]
    [string]$Theme = "dark",
    [string]$OutDir = "docs/desktop/screenshots"
)
$ErrorActionPreference = "Stop"
$screens = @("overview", "processes", "storage", "developer", "settings")
foreach ($screen in $screens) {
    & powershell -NoProfile -ExecutionPolicy Bypass -File "$PSScriptRoot/capture-screen.ps1" -Theme $Theme -Screen $screen -OutDir $OutDir
    if ($LASTEXITCODE -ne 0) { throw "capture failed for $screen/$Theme" }
}
Write-Host "Gallery complete: $OutDir"
