# clibot Windows Installation Script
# Downloads latest release from GitHub and installs to ~/.local/bin

$ErrorActionPreference = "Stop"

$Repo = "keepmind9/clibot"
$Binary = "clibot"
$InstallDir = "$env:USERPROFILE\.local\bin"

Write-Host "Checking clibot installation..."

# Get latest release info
Write-Host "Fetching latest release..."
$releaseInfo = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -TimeoutSec 30

if (-not $releaseInfo.tag_name) {
    Write-Host "No releases found. Install manually:" -ForegroundColor Red
    Write-Host "  https://github.com/$Repo/releases"
    exit 1
}

$latestVersion = $releaseInfo.tag_name

if (Get-Command $Binary -ErrorAction SilentlyContinue) {
    try {
        $currentOutput = & $Binary version 2>$null
        $currentVersion = ($currentOutput | Select-String "Version:\s+(\S+)").Matches.Groups[1].Value
        if ($currentVersion -eq $latestVersion.TrimStart("v")) {
            Write-Host "clibot is already up to date ($latestVersion)."
            exit 0
        }
        if ($currentVersion) {
            Write-Host "clibot $currentVersion installed, upgrading to $latestVersion..."
        } else {
            Write-Host "clibot installed, upgrading to $latestVersion..."
        }
    } catch {
        Write-Host "clibot installed but broken, reinstalling $latestVersion..."
    }
} else {
    Write-Host "clibot not found. Installing $latestVersion..."
}

# Find matching asset for windows-amd64
try {
    $version = $releaseInfo.tag_name
    $asset = $releaseInfo.assets | Where-Object { $_.name -like "*windows-amd64*" -and $_.name -notlike "*.sha256" } | Select-Object -First 1

    if (-not $asset) {
        Write-Host "No matching release found for windows-amd64." -ForegroundColor Red
        Write-Host "Available assets:"
        $releaseInfo.assets | ForEach-Object { Write-Host "  $($_.name)" }
        exit 1
    }

    Write-Host "Downloading clibot $version for Windows..."

    $tmpDir = [System.IO.Path]::GetTempPath() + "clibot-install"
    New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

    $downloadPath = Join-Path $tmpDir "$Binary.exe"
    Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $downloadPath -TimeoutSec 120

    # Install
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }

    Move-Item $downloadPath "$InstallDir\$Binary.exe" -Force

    # Clean up
    Remove-Item $tmpDir -Recurse -Force

    # Add to PATH if needed
    $pathEnv = [Environment]::GetEnvironmentVariable("PATH", "User")
    if ($pathEnv -notlike "*$InstallDir*") {
        Write-Host "Adding $InstallDir to user PATH..."
        $newPath = $pathEnv + ";$InstallDir"
        [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
        Write-Host "Added to PATH. Restart your shell for changes to take effect."
    }

    Write-Host ""
    Write-Host "clibot $version installed successfully!"
    Write-Host "  Location: $InstallDir\$Binary.exe"
    Write-Host ""
    Write-Host "Verify:"
    Write-Host "  clibot version"

} catch {
    Write-Host "Installation failed: $_" -ForegroundColor Red
    Write-Host "Install manually: https://github.com/$Repo/releases"
    exit 1
}
