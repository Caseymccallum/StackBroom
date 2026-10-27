# Validates and builds StackBroom on Windows.
#
#   pwsh -File scripts/build.ps1 -Version 0.1.0
#
# The script fails on formatting drift, vet findings or test failures, and does
# not touch anything outside this repository.

param(
    [string]$Version = "0.2.0-dev",
    [switch]$SkipLiveTests
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot

try {
    Write-Host "==> gofmt check"
    $unformatted = gofmt -l .
    if ($unformatted) {
        Write-Host "These files need gofmt:" -ForegroundColor Red
        $unformatted | ForEach-Object { Write-Host "  $_" }
        exit 1
    }

    Write-Host "==> go vet"
    go vet ./...
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    Write-Host "==> go test"
    go test ./...
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    if (-not $SkipLiveTests) {
        Write-Host "==> live read-only inspection tests"
        $previous = $env:STACKBROOM_LIVE
        $previousNetstat = $env:STACKBROOM_LIVE_NETSTAT
        $env:STACKBROOM_LIVE = "1"
        $env:STACKBROOM_LIVE_NETSTAT = "1"
        try {
            go test ./internal/winproc -run TestLive
            if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
            go test ./internal/netstat -run TestLive
            if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        }
        finally {
            $env:STACKBROOM_LIVE = $previous
            $env:STACKBROOM_LIVE_NETSTAT = $previousNetstat
        }
    }

    Write-Host "==> build bin/stackbroom.exe ($Version)"
    New-Item -ItemType Directory -Force -Path (Join-Path $repoRoot "bin") | Out-Null
    $ldflags = "-s -w -X github.com/Caseymccallum/StackBroom/internal/version.Version=$Version"
    go build -trimpath -ldflags $ldflags -o (Join-Path $repoRoot "bin/stackbroom.exe") ./cmd/stackbroom
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    Write-Host "==> done"
    & (Join-Path $repoRoot "bin/stackbroom.exe") version
}
finally {
    Pop-Location
}
