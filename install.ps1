#!/usr/bin/env pwsh
#Requires -Version 5.1

$ErrorActionPreference = "Stop"

$Repo = "enrell/just-stream"
$InstallDir = "$env:LOCALAPPDATA\bin"
$BinaryName = "just-stream.exe"

# Colors
function Write-Info($Message) {
    Write-Host $Message -ForegroundColor Cyan
}

function Write-Success($Message) {
    Write-Host $Message -ForegroundColor Green
}

function Write-Warning($Message) {
    Write-Host $Message -ForegroundColor Yellow
}

function Write-Error($Message) {
    Write-Host $Message -ForegroundColor Red
}

function Get-LatestRelease {
    $response = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    return $response.tag_name
}

function Get-Architecture {
    if ($env:PROCESSOR_ARCHITECTURE -eq "AMD64") {
        return "amd64"
    } elseif ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
        return "arm64"
    } else {
        return "amd64"
    }
}

function Test-Command($Command) {
    $null = Get-Command $Command -ErrorAction SilentlyContinue
    return $?
}

function Ask-User($Prompt) {
    $response = Read-Host "$Prompt [Y/n]"
    $response = if ($response) { $response } else { "Y" }
    return $response -match "^[Yy]"
}

function Install-Scoop {
    Write-Info "Scoop not found. Installing Scoop..."
    Write-Info ""
    
    try {
        Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser -Force
        Invoke-RestMethod -Uri https://get.scoop.sh | Invoke-Expression
        Write-Success "Scoop installed successfully!"
        return $true
    } catch {
        Write-Error "Failed to install Scoop: $_"
        return $false
    }
}

function Install-Dependencies-With-Scoop {
    Write-Info ""
    Write-Info "Installing dependencies with Scoop..."
    Write-Info "===================================="
    
    # Check for Scoop
    if (-not (Test-Command "scoop")) {
        if (Ask-User "Scoop package manager not found. Install Scoop?") {
            if (-not (Install-Scoop)) {
                Write-Error "Failed to install Scoop. Please install mpv and ffmpeg manually."
                return $false
            }
        } else {
            Write-Error "Scoop is required to install dependencies automatically."
            Write-Info "Please install mpv and ffmpeg manually from:"
            Write-Info "  https://mpv.io/installation/"
            Write-Info "  https://ffmpeg.org/download.html"
            return $false
        }
    }
    
    # Install mpv
    if (-not (Test-Command "mpv")) {
        Write-Info "Installing mpv..."
        try {
            scoop install mpv
            Write-Success "mpv installed successfully!"
        } catch {
            Write-Error "Failed to install mpv: $_"
        }
    } else {
        Write-Success "mpv is already installed"
    }
    
    # Install ffmpeg
    if (-not (Test-Command "ffmpeg")) {
        Write-Info "Installing ffmpeg..."
        try {
            scoop install ffmpeg
            Write-Success "ffmpeg installed successfully!"
        } catch {
            Write-Error "Failed to install ffmpeg: $_"
        }
    } else {
        Write-Success "ffmpeg is already installed"
    }
    
    return $true
}

function Install-Dependencies-With-Chocolatey {
    Write-Info ""
    Write-Info "Installing dependencies with Chocolatey..."
    Write-Info "=========================================="
    
    # Check for Chocolatey
    if (-not (Test-Command "choco")) {
        Write-Warning "Chocolatey not found. Skipping..."
        return $false
    }
    
    # Install mpv
    if (-not (Test-Command "mpv")) {
        Write-Info "Installing mpv via Chocolatey..."
        try {
            choco install mpv -y
            Write-Success "mpv installed successfully!"
        } catch {
            Write-Error "Failed to install mpv: $_"
        }
    } else {
        Write-Success "mpv is already installed"
    }
    
    # Install ffmpeg
    if (-not (Test-Command "ffmpeg")) {
        Write-Info "Installing ffmpeg via Chocolatey..."
        try {
            choco install ffmpeg -y
            Write-Success "ffmpeg installed successfully!"
        } catch {
            Write-Error "Failed to install ffmpeg: $_"
        }
    } else {
        Write-Success "ffmpeg is already installed"
    }
    
    return $true
}

function Install-Dependencies-With-Winget {
    Write-Info ""
    Write-Info "Installing dependencies with Winget..."
    Write-Info "======================================"
    
    # Check for Winget
    if (-not (Test-Command "winget")) {
        Write-Warning "Winget not found. Skipping..."
        return $false
    }
    
    # Install mpv
    if (-not (Test-Command "mpv")) {
        Write-Info "Installing mpv via Winget..."
        try {
            winget install --id=mpv.io -e
            Write-Success "mpv installed successfully!"
        } catch {
            Write-Error "Failed to install mpv: $_"
        }
    } else {
        Write-Success "mpv is already installed"
    }
    
    # Install ffmpeg
    if (-not (Test-Command "ffmpeg")) {
        Write-Info "Installing ffmpeg via Winget..."
        try {
            winget install --id=Gyan.FFmpeg -e
            Write-Success "ffmpeg installed successfully!"
        } catch {
            Write-Error "Failed to install ffmpeg: $_"
        }
    } else {
        Write-Success "ffmpeg is already installed"
    }
    
    return $true
}

# Set custom path for a dependency
function Set-Custom-Path {
    param($Dep)
    
    $envVar = ""
    $defaultPath = ""
    
    switch ($Dep) {
        "mpv" {
            $envVar = "MPV_PATH"
            $defaultPath = "C:\Program Files\mpv\mpv.exe"
        }
        "ffmpeg" {
            $envVar = "FFMPEG_PATH"
            $defaultPath = "C:\Program Files\ffmpeg\bin\ffmpeg.exe"
        }
        default { return $false }
    }
    
    Write-Info ""
    Write-Info "Setting custom path for $Dep"
    $customPath = Read-Host "Enter the full path to $Dep binary [$defaultPath]"
    $customPath = if ($customPath) { $customPath } else { $defaultPath }
    
    if (Test-Path $customPath -PathType Leaf) {
        # Set user environment variable permanently
        [Environment]::SetEnvironmentVariable($envVar, $customPath, "User")
        # Set for current session
        Set-Item -Path "Env:$envVar" -Value $customPath
        Write-Success "Set $envVar=$customPath"
        return $true
    } else {
        Write-Error "Binary not found at: $customPath"
        return $false
    }
}

function Check-And-Install-Dependencies {
    Write-Info ""
    Write-Info "Checking dependencies..."
    Write-Info "======================="
    
    $missingDeps = @()
    
    # Check if commands exist OR environment variables are set
    if (-not (Test-Command "mpv") -and -not $env:MPV_PATH) {
        $missingDeps += "mpv"
    }
    
    if (-not (Test-Command "ffmpeg") -and -not $env:FFMPEG_PATH) {
        $missingDeps += "ffmpeg"
    }
    
    if ($missingDeps.Count -eq 0) {
        Write-Success "All dependencies are installed!"
        return $true
    }
    
    Write-Warning "Missing dependencies: $($missingDeps -join ', ')"
    Write-Info ""
    
    # Handle each missing dependency individually
    foreach ($dep in $missingDeps) {
        Write-Info ""
        Write-Info "Dependency: $dep"
        
        # Ask to install
        if (Ask-User "Install $dep using package manager?") {
            # Try different package managers
            $installers = @(
                @{ Name = "Scoop"; Function = ${function:Install-Dependencies-With-Scoop} }
                @{ Name = "Chocolatey"; Function = ${function:Install-Dependencies-With-Chocolatey} }
                @{ Name = "Winget"; Function = ${function:Install-Dependencies-With-Winget} }
            )
            
            $installed = $false
            foreach ($installer in $installers) {
                if (Test-Command $installer.Name.ToLower()) {
                    Write-Info "Attempting to install $dep with $($installer.Name)..."
                    if (& $installer.Function) {
                        $installed = $true
                        break
                    }
                }
            }
            
            if (-not $installed) {
                Write-Error "Failed to install $dep with any package manager."
            }
        } else {
            # Offer to set custom path
            if (Ask-User "Do you have $dep installed at a custom location?") {
                Set-Custom-Path $dep
            } else {
                Write-Warning "WARNING: $dep is required for just-stream to work properly."
                Write-Info "You can set the path later by running:"
                if ($dep -eq "mpv") {
                    Write-Info "  [Environment]::SetEnvironmentVariable('MPV_PATH', 'C:\Path\To\mpv.exe', 'User')"
                } elseif ($dep -eq "ffmpeg") {
                    Write-Info "  [Environment]::SetEnvironmentVariable('FFMPEG_PATH', 'C:\Path\To\ffmpeg.exe', 'User')"
                }
            }
        }
    }
    
    return $true
}

function Setup-Anime4K {
    Write-Info ""
    Write-Info "Setting up Anime4K shaders..."
    Write-Info "============================"
    
    $shaderDir = "$env:APPDATA\mpv\shaders"
    New-Item -ItemType Directory -Force -Path $shaderDir | Out-Null
    
    $tempFile = "$env:TEMP\Anime4K.zip"
    
    try {
        Write-Info "Downloading Anime4K shaders..."
        Invoke-WebRequest -Uri "https://github.com/bloc97/Anime4K/releases/download/v4.0.1/Anime4K_v4.0.1.zip" -OutFile $tempFile
        
        Write-Info "Extracting shaders..."
        Expand-Archive -Path $tempFile -DestinationPath $shaderDir -Force
        Remove-Item -Path $tempFile -Force
        
        Write-Success "Anime4K shaders installed to: $shaderDir"
    } catch {
        Write-Error "Failed to install Anime4K shaders: $_"
        Write-Info "You can manually download them from:"
        Write-Info "https://github.com/bloc97/Anime4K/releases"
    }
}

function Install-JustStream {
    param($Version, $Arch)
    
    Write-Info ""
    Write-Info "Installing just-stream $Version for Windows $Arch..."
    Write-Info "===================================================="
    
    # Create install directory
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    
    $downloadUrl = "https://github.com/$Repo/releases/download/$Version/just-stream-windows-$Arch.exe"
    $outputPath = "$InstallDir\$BinaryName"
    
    Write-Info "Downloading from: $downloadUrl"
    
    try {
        Invoke-WebRequest -Uri $downloadUrl -OutFile $outputPath
    } catch {
        Write-Warning "Failed to download with arch suffix. Trying generic binary name..."
        $downloadUrl = "https://github.com/$Repo/releases/download/$Version/just-stream.exe"
        Invoke-WebRequest -Uri $downloadUrl -OutFile $outputPath
    }
    
    Write-Success ""
    Write-Success "✓ just-stream installed to: $outputPath"
    
    # Check if in PATH
    $pathDirs = $env:PATH -split ';'
    $inPath = $false
    foreach ($dir in $pathDirs) {
        if ($dir -eq $InstallDir) {
            $inPath = $true
            break
        }
    }
    
    if (-not $inPath) {
        Write-Warning ""
        Write-Warning "$InstallDir is not in your PATH"
        Write-Info "Add it to your PATH with:"
        Write-Info "  [Environment]::SetEnvironmentVariable('Path', '`$env:Path;$InstallDir', 'User')"
    }
    
    Write-Info ""
    Write-Info "Usage: just-stream [magnet-link]"
    Write-Info ""
    Write-Info "Keyboard shortcuts:"
    Write-Info "  - Input screen: Paste magnet link"
    Write-Info "  - File list: j/k to navigate, enter to play, 'a' to stream all"
    Write-Info "  - Playback: q to quit, Shift+>/< in mpv for next/prev"
    
    # Offer to setup Anime4K
    if (Ask-User "Would you like to install Anime4K shaders for enhanced quality?") {
        Setup-Anime4K
    }
}

# Main execution
Write-Info ""
Write-Info "just-stream installer"
Write-Info "====================="
Write-Info ""

# Check if running as Administrator (not recommended)
$currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if ($currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Warning "WARNING: Running as Administrator. This is not recommended for security reasons."
    if (-not (Ask-User "Continue anyway?")) {
        exit 1
    }
}

$arch = Get-Architecture
Write-Info "Detected: Windows $arch"
Write-Info ""

# Check and install dependencies
if (-not (Check-And-Install-Dependencies)) {
    exit 1
}

# Get latest version
Write-Info "Fetching latest release..."
$version = Get-LatestRelease

if (-not $version) {
    Write-Error "Error: Could not determine latest version"
    Write-Error "Please check your internet connection"
    exit 1
}

Write-Info "Latest version: $version"

# Install just-stream
Install-JustStream -Version $version -Arch $arch

Write-Info ""
Write-Success "Installation complete!"
Write-Info ""
Write-Info "To get started, run: just-stream"
