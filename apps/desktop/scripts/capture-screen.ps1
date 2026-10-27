# Captures one deterministic screen at 1366x768 and 1920x1080 in preview mode.
param(
    [ValidateSet("dark", "light")]
    [string]$Theme = "dark",
    [ValidateSet("overview", "processes", "storage", "developer", "settings")]
    [string]$Screen = "overview",
    [string]$OutDir = "docs/desktop/screenshots"
)
$ErrorActionPreference = "Stop"
$app = Join-Path (Split-Path -Parent $PSScriptRoot) "bin/StackBroom.exe"
Get-Process StackBroom -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Milliseconds 400
$previous = @{
    STACKBROOM_PREVIEW = $env:STACKBROOM_PREVIEW
    STACKBROOM_PREVIEW_THEME = $env:STACKBROOM_PREVIEW_THEME
    STACKBROOM_PREVIEW_SCREEN = $env:STACKBROOM_PREVIEW_SCREEN
}
$env:STACKBROOM_PREVIEW = "1"
$env:STACKBROOM_PREVIEW_THEME = $Theme
$env:STACKBROOM_PREVIEW_SCREEN = $Screen
try {
    $process = Start-Process -FilePath $app -PassThru
    Start-Sleep -Seconds 13
    $process.Refresh()
    if ($process.MainWindowHandle -eq 0) { throw "StackBroom window did not appear" }
    . "$PSScriptRoot/capture-lib.ps1"
    New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
    $resolvedOut = (Resolve-Path -LiteralPath $OutDir).Path

    Set-ClientSize 1366 768
    Focus-App
    $small = Join-Path $resolvedOut ("{0}-{1}-1366x768.png" -f $Screen, $Theme)
    Capture-Client $small
    Write-Host "Captured $small"

    Set-ClientSize 1920 1080
    Focus-App
    $large = Join-Path $resolvedOut ("{0}-{1}-1920x1080.png" -f $Screen, $Theme)
    Capture-Offscreen $large
    Write-Host "Captured $large"
}
finally {
    Get-Process StackBroom -ErrorAction SilentlyContinue | Stop-Process -Force
    $env:STACKBROOM_PREVIEW = $previous.STACKBROOM_PREVIEW
    $env:STACKBROOM_PREVIEW_THEME = $previous.STACKBROOM_PREVIEW_THEME
    $env:STACKBROOM_PREVIEW_SCREEN = $previous.STACKBROOM_PREVIEW_SCREEN
}
