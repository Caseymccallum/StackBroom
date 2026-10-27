# Build a separate local native validation binary. Shipping options remain unchanged.
# Run the normal build first so embedded frontend/dist matches current source.
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$sourcePath = Join-Path $root 'ux_validation_main.go'
if (Test-Path -LiteralPath $sourcePath) { throw 'Temporary validation source already exists; inspect it before retrying.' }
$original = Get-Content (Join-Path $root 'main.go') -Raw
$needle = 'Name:        "StackBroom",'
if (-not $original.Contains($needle)) { throw 'main.go options changed; update this validation helper.' }
$options = 'Windows: application.WindowsOptions{WebviewUserDataPath: "bin/ux-profile", AdditionalBrowserArgs: []string{"--remote-debugging-port=9333", "--remote-debugging-address=127.0.0.1"}},'
try {
 Set-Content -LiteralPath $sourcePath -Value $original.Replace($needle, $needle + "`n        " + $options)
 Push-Location $root
 try {
  $files = Get-ChildItem *.go | Where-Object { $_.Name -ne 'main.go' -and $_.Name -notlike '*_test.go' } | ForEach-Object { $_.Name }
  go build -tags production -ldflags '-H windowsgui' -o bin/StackBroom-UXValidation.exe $files
  if ($LASTEXITCODE -ne 0) { throw 'Validation build failed' }
 } finally { Pop-Location }
} finally {
 if (Test-Path -LiteralPath $sourcePath) {
  Move-Item -LiteralPath $sourcePath -Destination (Join-Path $root 'bin/ux_validation_main.go.txt') -Force
 }
}
Write-Host 'Launch bin/StackBroom-UXValidation.exe explicitly; close it normally after validation. Never publish this diagnostic binary.'
