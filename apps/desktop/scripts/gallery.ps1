# Captures the five real StackBroom screens in both themes at the target client
# sizes. Read-only: it only resizes, focuses and reads pixels.
param(
    [ValidateSet("dark", "light")]
    [string]$Theme = "dark",
    [string]$OutDir = "docs/desktop/screenshots",
    [switch]$SkipLarge
)
$ErrorActionPreference = "Stop"
$app = Join-Path (Split-Path -Parent $PSScriptRoot) "bin/StackBroom.exe"
Get-Process StackBroom -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Milliseconds 500
$previousPreview = $env:STACKBROOM_PREVIEW
$previousTheme = $env:STACKBROOM_PREVIEW_THEME
$env:STACKBROOM_PREVIEW = "1"
$env:STACKBROOM_PREVIEW_THEME = $Theme
try {
    $process = Start-Process -FilePath $app -PassThru
    Start-Sleep -Seconds 14
    $process.Refresh()
    if ($process.MainWindowHandle -eq 0) { throw "StackBroom window did not appear" }
    . "$PSScriptRoot/capture-lib.ps1"

    $screens = @(
        @{ name = "overview"; keys = "^1" },
        @{ name = "processes"; keys = "^2" },
        @{ name = "storage"; keys = "^3" },
        @{ name = "developer"; keys = "^4" },
        @{ name = "settings"; keys = "^5" }
    )
    New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
    $resolvedOut = (Resolve-Path -LiteralPath $OutDir).Path

    Set-ClientSize 1366 768
    Focus-App
    foreach ($screen in $screens) {
        $script:sbShell.SendKeys($screen.keys)
        Start-Sleep -Milliseconds 750
        $path = Join-Path $resolvedOut ("{0}-{1}-1366x768.png" -f $screen.name, $Theme)
        Capture-Client $path
        Write-Host "Captured $path"
    }

    if (-not $SkipLarge) {
        Set-ClientSize 1920 1080
        Focus-App
        foreach ($screen in $screens) {
            $script:sbShell.SendKeys($screen.keys)
            Start-Sleep -Milliseconds 850
            $path = Join-Path $resolvedOut ("{0}-{1}-1920x1080.png" -f $screen.name, $Theme)
            Capture-Offscreen $path
            Write-Host "Captured $path"
        }
    }
}
finally {
    Get-Process StackBroom -ErrorAction SilentlyContinue | Stop-Process -Force
    $env:STACKBROOM_PREVIEW = $previousPreview
    $env:STACKBROOM_PREVIEW_THEME = $previousTheme
}
Write-Host "Gallery complete: $OutDir"
