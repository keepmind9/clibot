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

# Build archive name
$versionNum = $latestVersion.TrimStart("v")
$archiveName = "$Binary-$versionNum-windows-amd64.zip"

# Find matching asset
try {
    $asset = $releaseInfo.assets | Where-Object { $_.name -eq $archiveName } | Select-Object -First 1

    if (-not $asset) {
        Write-Host "No matching release found for $archiveName." -ForegroundColor Red
        Write-Host "Available assets:"
        $releaseInfo.assets | ForEach-Object { Write-Host "  $($_.name)" }
        exit 1
    }

    Write-Host "Downloading clibot $latestVersion for Windows..."

    $tmpDir = [System.IO.Path]::GetTempPath() + "clibot-install"
    New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

    $downloadPath = Join-Path $tmpDir $archiveName
    Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $downloadPath -TimeoutSec 120

    # Extract archive
    Write-Host "Extracting..."
    Expand-Archive -Path $downloadPath -DestinationPath $tmpDir -Force

    # Find the binary in the extracted directory
    $binaryPath = Get-ChildItem -Path $tmpDir -Recurse -Filter "$Binary.exe" | Select-Object -First 1
    if (-not $binaryPath) {
        Write-Host "Failed to find clibot.exe in archive." -ForegroundColor Red
        exit 1
    }

    # Validate binary is within tmpDir (prevent path traversal)
    try {
        $realBinPath = (Resolve-Path $binaryPath.FullName).Path
        $realTmpDir = (Resolve-Path $tmpDir).Path
        if (-not $realBinPath.StartsWith($realTmpDir + '\')) {
            Write-Host "Binary path is outside expected directory. Aborting." -ForegroundColor Red
            exit 1
        }
    } catch {
        Write-Host "Failed to validate binary path. Aborting." -ForegroundColor Red
        exit 1
    }

    # Install
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }

    Copy-Item $binaryPath.FullName "$InstallDir\$Binary.exe" -Force

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
    Write-Host "clibot $latestVersion installed successfully!"
    Write-Host "  Location: $InstallDir\$Binary.exe"
    Write-Host ""
    Write-Host "Verify:"
    Write-Host "  clibot version"

} catch {
    Write-Host "Installation failed: $_" -ForegroundColor Red
    Write-Host "Install manually: https://github.com/$Repo/releases"
    exit 1
}