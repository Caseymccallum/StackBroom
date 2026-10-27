# Generates the StackBroom desktop icon. Uses only Windows' System.Drawing;
# no network access and no third-party image tooling.
$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.Drawing

$root = Split-Path -Parent $PSScriptRoot
$build = Join-Path $root "build"
New-Item -ItemType Directory -Force -Path $build | Out-Null
$path = Join-Path $build "appicon.png"

$size = 512
$bitmap = New-Object System.Drawing.Bitmap($size, $size)
$graphics = [System.Drawing.Graphics]::FromImage($bitmap)
$graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias

$rect = New-Object System.Drawing.Rectangle(0, 0, $size, $size)
$navy = [System.Drawing.Color]::FromArgb(255, 10, 17, 24)
$teal = [System.Drawing.Color]::FromArgb(255, 45, 212, 191)
$cyan = [System.Drawing.Color]::FromArgb(255, 34, 211, 238)
$brush = New-Object System.Drawing.Drawing2D.LinearGradientBrush($rect, $navy, [System.Drawing.Color]::FromArgb(255, 16, 35, 45), 45)
$graphics.FillRectangle($brush, $rect)

$clipPath = New-Object System.Drawing.Drawing2D.GraphicsPath
$r = 104
$clipPath.AddArc(0, 0, $r, $r, 180, 90)
$clipPath.AddArc($size - $r, 0, $r, $r, 270, 90)
$clipPath.AddArc($size - $r, $size - $r, $r, $r, 0, 90)
$clipPath.AddArc(0, $size - $r, $r, $r, 90, 90)
$clipPath.CloseFigure()
$graphics.SetClip($clipPath)
$glow = New-Object System.Drawing.Drawing2D.GraphicsPath
$glow.AddEllipse(110, 60, 330, 330)
$glowBrush = New-Object System.Drawing.Drawing2D.PathGradientBrush($glow)
$glowBrush.CenterColor = [System.Drawing.Color]::FromArgb(90, 34, 211, 238)
$glowBrush.SurroundColors = @([System.Drawing.Color]::FromArgb(0, 34, 211, 238))
$graphics.FillPath($glowBrush, $glow)
$graphics.ResetClip()

$pen = New-Object System.Drawing.Pen($teal, 24)
$pen.StartCap = [System.Drawing.Drawing2D.LineCap]::Round
$pen.EndCap = [System.Drawing.Drawing2D.LineCap]::Round
$graphics.DrawLine($pen, 130, 170, 370, 170)
$graphics.DrawLine($pen, 160, 236, 340, 236)
$graphics.DrawLine($pen, 196, 302, 306, 302)

$handle = New-Object System.Drawing.Pen($cyan, 22)
$handle.StartCap = [System.Drawing.Drawing2D.LineCap]::Round
$handle.EndCap = [System.Drawing.Drawing2D.LineCap]::Round
$graphics.DrawLine($handle, 306, 340, 382, 404)
$graphics.FillEllipse((New-Object System.Drawing.SolidBrush($cyan)), 350, 374, 72, 56)

$bitmap.Save($path, [System.Drawing.Imaging.ImageFormat]::Png)
$graphics.Dispose()
$bitmap.Dispose()
Write-Host "Wrote $path"
