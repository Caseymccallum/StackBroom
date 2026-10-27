# Builds the StackBroom Windows desktop executable.
# Requires: Go 1.26+, Node/npm, Wails v3 CLI, WebView2 runtime.
param(
    [string]$Version = "0.2.0",
    [switch]$SkipTests
)
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$frontend = Join-Path $root "frontend"
$wails = (Get-Command wails3 -ErrorAction SilentlyContinue).Source
if (-not $wails) {
    $candidate = Join-Path $env:USERPROFILE "go/bin/wails3.exe"
    if (Test-Path -LiteralPath $candidate) { $wails = $candidate }
}
if (-not $wails) {
    throw "wails3 was not found. Install Wails v3 CLI explicitly (go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0) before building."
}

Push-Location $root
try {
    if (-not (Test-Path -LiteralPath "build/windows/icon.ico")) {
        powershell -NoProfile -ExecutionPolicy Bypass -File scripts/make-icon.ps1
        & $wails generate icons -input build/appicon.png -windowsfilename build/windows/icon.ico
    }

    & $wails generate bindings -ts -i -clean -d frontend/bindings .
    if ($LASTEXITCODE -ne 0) { throw "binding generation failed" }

    Push-Location $frontend
    try {
        npm ci
        if ($LASTEXITCODE -ne 0) { throw "npm ci failed" }
        npm test
        if ($LASTEXITCODE -ne 0) { throw "frontend tests failed" }
        npm run build
        if ($LASTEXITCODE -ne 0) { throw "frontend build failed" }
    }
    finally {
        Pop-Location
    }

    if (-not $SkipTests) {
        go test ./... -count=1
        if ($LASTEXITCODE -ne 0) { throw "desktop Go tests failed" }
        go vet ./...
        if ($LASTEXITCODE -ne 0) { throw "desktop go vet failed" }
    }

    & $wails generate syso -arch amd64 `
        -icon build/windows/icon.ico `
        -manifest build/windows/wails.exe.manifest `
        -info build/windows/info.json `
        -out ./wails_windows_amd64.syso
    if ($LASTEXITCODE -ne 0) { throw "Windows resource generation failed" }

    New-Item -ItemType Directory -Force -Path bin | Out-Null
    go build -tags production -trimpath -buildvcs=false `
        -ldflags "-w -s -H windowsgui -X github.com/Caseymccallum/StackBroom/internal/version.Version=$Version" `
        -o bin/StackBroom.exe .
    if ($LASTEXITCODE -ne 0) { throw "desktop build failed" }

    $binary = Get-Item bin/StackBroom.exe
    $hash = Get-FileHash -Algorithm SHA256 bin/StackBroom.exe
    Write-Host ("Built {0} ({1:N0} bytes) SHA256 {2}" -f $binary.FullName, $binary.Length, $hash.Hash)
}
finally {
    Remove-Item -LiteralPath (Join-Path $root "wails_windows_amd64.syso") -Force -ErrorAction SilentlyContinue
    Pop-Location
}
exit 0
