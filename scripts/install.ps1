# Install-Dock.ps1 — Windows installer for Dock.
#
# Downloads the latest tagged release from GitHub, verifies the archive sha256
# against checksums.txt, and installs dock.exe under %LOCALAPPDATA%\Programs\Dock.
# Requires Windows 10 1803+ (ships curl.exe and tar). No Go toolchain needed.
#
# Recommended usage (no MOTW on the script or the binary):
#   powershell -ExecutionPolicy Bypass -Command "irm https://raw.githubusercontent.com/udit-001/dock/main/scripts/install.ps1 | iex"
#
# Or download the script and run:
#   Unblock-File .\install.ps1; .\install.ps1
#
# To uninstall: remove "$InstallDir" and delete it from your user PATH.

[CmdletBinding()]
param(
    [string]$InstallDir = "$env:LOCALAPPDATA\Programs\Dock"
)

$ErrorActionPreference = 'Stop'

$repo = 'udit-001/dock'
$releaseBase = "https://github.com/$repo/releases"
$downloadBase = "$releaseBase/download"

# Resolve the latest tag via the /releases/latest redirect (no JSON parsing).
$tag = & curl.exe -fsSL -o NUL -w '%{url_effective}' "$releaseBase/latest"
if ($LASTEXITCODE -ne 0) { throw 'Failed to resolve the latest release' }
$tag = ($tag -split '/')[-1]

if ($env:PROCESSOR_ARCHITECTURE -ne 'AMD64') {
    throw "No prebuilt binary for $($env:PROCESSOR_ARCHITECTURE) (only windows/amd64 is published). Fall back to: go install $repo@latest"
}

if (Get-Process dock -ErrorAction SilentlyContinue) {
    throw 'Dock is currently running - close it and re-run the installer.'
}

$asset = "dock_${tag}_windows_amd64.tar.gz"

$tmp = Join-Path $env:TEMP ("dock-install-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    $archive = Join-Path $tmp $asset
    Write-Host "Downloading $asset"
    & curl.exe -fsSL "$downloadBase/$tag/$asset" -o $archive
    if ($LASTEXITCODE -ne 0) { throw "Download failed: $asset" }

    $checksums = Join-Path $tmp 'checksums.txt'
    & curl.exe -fsSL "$downloadBase/$tag/checksums.txt" -o $checksums
    if ($LASTEXITCODE -ne 0) { throw 'Download failed: checksums.txt' }

    $expected = (Get-Content $checksums | Where-Object { $_ -match " $asset$" } | Select-Object -First 1) -split '\s+' | Select-Object -First 1
    if (-not $expected) { throw "No checksum entry for $asset" }

    $actual = (Get-FileHash -Algorithm SHA256 -Path $archive).Hash.ToLowerInvariant()
    if ($actual -ne $expected) {
        throw "sha256 mismatch`n  expected $expected`n  got      $actual"
    }

    Write-Host "Installing to $InstallDir"
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    & tar -xzf $archive -C $InstallDir dock.exe
    if ($LASTEXITCODE -ne 0) { throw "Failed to extract $asset (Windows 10 1803+ required for tar)" }
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

# Add to the user PATH once.
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if ((";$userPath;") -notlike "*;$InstallDir;*") {
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$InstallDir", 'User')
    Write-Host "Added $InstallDir to your user PATH (open a new terminal)."
}

Write-Host "Installed dock $tag -> $InstallDir\dock.exe"
Write-Host "Run: dock"
