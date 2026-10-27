Add-Type -AssemblyName System.Drawing
Add-Type @"
using System;
using System.Runtime.InteropServices;
public static class GalleryCap {
    [StructLayout(LayoutKind.Sequential)] public struct RECT { public int Left, Top, Right, Bottom; }
    [StructLayout(LayoutKind.Sequential)] public struct POINT { public int X, Y; }
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT rect);
    [DllImport("user32.dll")] public static extern bool GetClientRect(IntPtr hWnd, out RECT rect);
    [DllImport("user32.dll")] public static extern bool ClientToScreen(IntPtr hWnd, ref POINT point);
    [DllImport("user32.dll")] public static extern bool SetWindowPos(IntPtr hWnd, IntPtr after, int x, int y, int cx, int cy, uint flags);
    [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int command);
    [DllImport("user32.dll")] public static extern bool BringWindowToTop(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(IntPtr hWnd, out uint processId);
    [DllImport("user32.dll")] public static extern bool AttachThreadInput(uint attach, uint attachTo, bool attachFlag);
    [DllImport("user32.dll")] public static extern IntPtr SetFocus(IntPtr hWnd);
    [DllImport("kernel32.dll")] public static extern uint GetCurrentThreadId();
    [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr hWnd, IntPtr hdc, uint flags);
    [DllImport("user32.dll")] public static extern bool SetProcessDPIAware();
}
"@
[GalleryCap]::SetProcessDPIAware() | Out-Null

$script:sbProc = Get-Process StackBroom -ErrorAction Stop | Where-Object { $_.MainWindowHandle -ne 0 } | Select-Object -First 1
if (-not $script:sbProc) { throw "StackBroom window not found" }
$script:sbHandle = $script:sbProc.MainWindowHandle
$script:sbShell = New-Object -ComObject WScript.Shell

function Focus-App {
    [GalleryCap]::ShowWindow($script:sbHandle, 9) | Out-Null
    [GalleryCap]::BringWindowToTop($script:sbHandle) | Out-Null
    $fg = [GalleryCap]::GetForegroundWindow()
    $fgPid = [uint32]0
    $fgThread = [GalleryCap]::GetWindowThreadProcessId($fg, [ref]$fgPid)
    $current = [GalleryCap]::GetCurrentThreadId()
    [GalleryCap]::AttachThreadInput($current, $fgThread, $true) | Out-Null
    [GalleryCap]::SetForegroundWindow($script:sbHandle) | Out-Null
    [GalleryCap]::BringWindowToTop($script:sbHandle) | Out-Null
    [GalleryCap]::SetFocus($script:sbHandle) | Out-Null
    [GalleryCap]::AttachThreadInput($current, $fgThread, $false) | Out-Null
    Start-Sleep -Milliseconds 700
    $script:sbShell.AppActivate($script:sbProc.Id) | Out-Null
    Start-Sleep -Milliseconds 300
}

function Set-ClientSize([int]$width, [int]$height) {
    $outer = New-Object GalleryCap+RECT
    $client = New-Object GalleryCap+RECT
    [GalleryCap]::GetWindowRect($script:sbHandle, [ref]$outer) | Out-Null
    [GalleryCap]::GetClientRect($script:sbHandle, [ref]$client) | Out-Null
    $frameW = ($outer.Right - $outer.Left) - ($client.Right - $client.Left)
    $frameH = ($outer.Bottom - $outer.Top) - ($client.Bottom - $client.Top)
    [GalleryCap]::SetWindowPos($script:sbHandle, [IntPtr](-1), 10, 10, $width + $frameW, $height + $frameH, 0x0040) | Out-Null
    Start-Sleep -Milliseconds 700
}

function Capture-Client([string]$path) {
    $client = New-Object GalleryCap+RECT
    [GalleryCap]::GetClientRect($script:sbHandle, [ref]$client) | Out-Null
    $width = $client.Right - $client.Left
    $height = $client.Bottom - $client.Top
    $origin = New-Object GalleryCap+POINT
    [GalleryCap]::ClientToScreen($script:sbHandle, [ref]$origin) | Out-Null
    $bitmap = New-Object System.Drawing.Bitmap($width, $height)
    $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
    $graphics.CopyFromScreen($origin.X, $origin.Y, 0, 0, (New-Object System.Drawing.Size($width, $height)))
    $bitmap.Save($path, [System.Drawing.Imaging.ImageFormat]::Png)
    $graphics.Dispose()
    $bitmap.Dispose()
}

function Capture-Offscreen([string]$path) {
    $outer = New-Object GalleryCap+RECT
    $client = New-Object GalleryCap+RECT
    [GalleryCap]::GetWindowRect($script:sbHandle, [ref]$outer) | Out-Null
    [GalleryCap]::GetClientRect($script:sbHandle, [ref]$client) | Out-Null
    $origin = New-Object GalleryCap+POINT
    [GalleryCap]::ClientToScreen($script:sbHandle, [ref]$origin) | Out-Null
    $frameX = $origin.X - $outer.Left
    $frameY = $origin.Y - $outer.Top
    $outerW = $outer.Right - $outer.Left
    $outerH = $outer.Bottom - $outer.Top
    $clientW = $client.Right - $client.Left
    $clientH = $client.Bottom - $client.Top
    $full = New-Object System.Drawing.Bitmap($outerW, $outerH)
    $fullGraphics = [System.Drawing.Graphics]::FromImage($full)
    $hdc = $fullGraphics.GetHdc()
    $ok = [GalleryCap]::PrintWindow($script:sbHandle, $hdc, 2)
    $fullGraphics.ReleaseHdc($hdc)
    $fullGraphics.Dispose()
    if (-not $ok) { throw "PrintWindow failed" }
    $cropped = New-Object System.Drawing.Bitmap($clientW, $clientH)
    $cropGraphics = [System.Drawing.Graphics]::FromImage($cropped)
    $cropGraphics.DrawImage($full, (New-Object System.Drawing.Rectangle(0, 0, $clientW, $clientH)), (New-Object System.Drawing.Rectangle($frameX, $frameY, $clientW, $clientH)), [System.Drawing.GraphicsUnit]::Pixel)
    $cropped.Save($path, [System.Drawing.Imaging.ImageFormat]::Png)
    $cropGraphics.Dispose()
    $cropped.Dispose()
    $full.Dispose()
}
