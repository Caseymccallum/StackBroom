# Captures the process details and in-app action confirmation at both target
# client sizes. Preview-only: no process action is executed.
param(
    [ValidateSet("dark", "light")]
    [string]$Theme = "dark",
    [ValidateSet("detail", "confirm")]
    [string]$Mode = "detail",
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
    STACKBROOM_PREVIEW_DETAIL = $env:STACKBROOM_PREVIEW_DETAIL
    STACKBROOM_PREVIEW_CONFIRM = $env:STACKBROOM_PREVIEW_CONFIRM
}
$env:STACKBROOM_PREVIEW = "1"
$env:STACKBROOM_PREVIEW_THEME = $Theme
$env:STACKBROOM_PREVIEW_SCREEN = "processes"
$env:STACKBROOM_PREVIEW_DETAIL = "1"
if ($Mode -eq "confirm") { $env:STACKBROOM_PREVIEW_CONFIRM = "1" } else { $env:STACKBROOM_PREVIEW_CONFIRM = "0" }
try {
    $process = Start-Process -FilePath $app -PassThru
    Start-Sleep -Seconds 13
    $process.Refresh()
    if ($process.MainWindowHandle -eq 0) { throw "StackBroom window did not appear" }
    . "$PSScriptRoot/capture-lib.ps1"
    New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
    $resolvedOut = (Resolve-Path -LiteralPath $OutDir).Path
    $name = if ($Mode -eq "confirm") { "process-confirm" } else { "process-detail" }

    Set-ClientSize 1366 768
    Focus-App
    $small = Join-Path $resolvedOut ("{0}-{1}-1366x768.png" -f $name, $Theme)
    Capture-Client $small
    Write-Host "Captured $small"

    Set-ClientSize 1920 1080
    Focus-App
    $large = Join-Path $resolvedOut ("{0}-{1}-1920x1080.png" -f $name, $Theme)
    Capture-Offscreen $large
    Write-Host "Captured $large"
}
finally {
    Get-Process StackBroom -ErrorAction SilentlyContinue | Stop-Process -Force
    $env:STACKBROOM_PREVIEW = $previous.STACKBROOM_PREVIEW
    $env:STACKBROOM_PREVIEW_THEME = $previous.STACKBROOM_PREVIEW_THEME
    $env:STACKBROOM_PREVIEW_SCREEN = $previous.STACKBROOM_PREVIEW_SCREEN
    $env:STACKBROOM_PREVIEW_DETAIL = $previous.STACKBROOM_PREVIEW_DETAIL
    $env:STACKBROOM_PREVIEW_CONFIRM = $previous.STACKBROOM_PREVIEW_CONFIRM
}
