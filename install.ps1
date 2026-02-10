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
    try {
        $response = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -ErrorAction Stop
        return $response.tag_name
    } catch {
        $statusCode = $_.Exception.Response.StatusCode.value__
        if ($statusCode -eq 404) {
            Write-Error "No releases found for $Repo"
            Write-Info ""
            Write-Info "The repository exists but has no published releases."
            Write-Info "You can either:"
            Write-Info "  1. Build from source: git clone https://github.com/$Repo && cd just-stream && go build"
            Write-Info "  2. Wait for the maintainer to publish a release"
            Write-Info ""
            Write-Info "For manual installation, visit: https://github.com/$Repo"
        } else {
            Write-Error "Failed to fetch latest release: $_"
        }
        return $null
    }
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

function Find-MPV {
    # Check if MPV_PATH environment variable is set
    if ($env:MPV_PATH -and (Test-Path $env:MPV_PATH -PathType Leaf)) {
        return $env:MPV_PATH
    }

    # Check if mpv is in PATH
    $mpvCommand = Get-Command "mpv" -ErrorAction SilentlyContinue
    if ($mpvCommand) {
        return $mpvCommand.Source
    }

    # Check common installation paths
    $commonPaths = @(
        "$env:LOCALAPPDATA\Programs\mpv.net\mpvnet.exe",
        "$env:LOCALAPPDATA\Programs\mpv.net\mpv.exe",
        "$env:LOCALAPPDATA\mpv\mpv.exe",
        "$env:PROGRAMFILES\mpv\mpv.exe",
        "$env:PROGRAMFILES(x86)\mpv\mpv.exe",
        "$env:PROGRAMFILES\mpv.net\mpvnet.exe",
        "$env:PROGRAMFILES(x86)\mpv.net\mpvnet.exe"
    )

    foreach ($path in $commonPaths) {
        if (Test-Path $path -PathType Leaf) {
            return $path
        }
    }

    return $null
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
    if (-not (Find-MPV)) {
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
    if (-not (Find-MPV)) {
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
    if (-not (Find-MPV)) {
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
            $defaultPath = "$env:LOCALAPPDATA\Programs\mpv.net\mpvnet.exe"
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

    # Check mpv - use Find-MPV which searches PATH and common locations
    $mpvPath = Find-MPV
    if ($mpvPath) {
        $env:MPV_PATH = $mpvPath
        Set-Item -Path "Env:MPV_PATH" -Value $mpvPath
        Write-Success "Found mpv at: $mpvPath"
    } else {
        $missingDeps += "mpv"
    }

    # Check ffmpeg
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

function Get-GPU-Tier {
    Write-Info ""
    Write-Info "Select your GPU tier for optimal Anime4K settings:"
    Write-Info ""
    Write-Info "1. Low-End (GTX 980/1060, RX 570, GTX 1650, Intel UHD/ARC, M1/M2)"
    Write-Info "   - Optimized shaders with smaller network sizes (S/M)"
    Write-Info "   - Balanced quality and performance"
    Write-Info ""
    Write-Info "2. High-End (GTX 1080+, RTX 2070/3060+, RX 6000+, RTX 4060+)"
    Write-Info "   - Full shader set with larger networks (L/VL/UL)"
    Write-Info "   - Maximum quality enhancement"
    Write-Info ""
    
    $choice = Read-Host "Enter your choice [1-2 or n to skip]"
    
    switch ($choice) {
        "1" { return "low" }
        "2" { return "high" }
        "n" { return $null }
        "N" { return $null }
        default { 
            Write-Warning "Invalid choice. Skipping Anime4K setup."
            return $null
        }
    }
}

function Setup-Anime4K {
    Write-Info ""
    Write-Info "Setting up Anime4K shaders..."
    Write-Info "============================"
    
    $gpuTier = Get-GPU-Tier
    if (-not $gpuTier) {
        Write-Info "Skipping Anime4K setup."
        return
    }
    
    $shaderDir = "$env:APPDATA\mpv\shaders"
    New-Item -ItemType Directory -Force -Path $shaderDir | Out-Null
    
    $tempDir = "$env:TEMP\Anime4K"
        $tempFile = "$env:TEMP\Anime4K_v4.0.zip"

    try {
        Write-Info "Downloading Anime4K shaders v4.0.1..."
        $downloadUrl = "https://github.com/bloc97/Anime4K/releases/download/v4.0.1/Anime4K_v4.0.zip"
        Invoke-WebRequest -Uri $downloadUrl -OutFile $tempFile -ErrorAction Stop
        
        Write-Info "Extracting shaders..."
        if (Test-Path $tempDir) {
            Remove-Item -Path $tempDir -Recurse -Force
        }
        New-Item -ItemType Directory -Force -Path $tempDir | Out-Null
        Expand-Archive -Path $tempFile -DestinationPath $tempDir -Force
        Remove-Item -Path $tempFile -Force
        
        # Copy shaders based on GPU tier
        if ($gpuTier -eq "low") {
            Write-Info "Installing Low-End GPU shaders (S/M variants)..."
            Get-ChildItem -Path $tempDir -Filter "*.glsl" | Where-Object { 
                $_.Name -match "_(S|M)\\.glsl$" -or $_.Name -match "Upscale" -or $_.Name -match "Denoise" -or $_.Name -match "AutoDownscale"
            } | ForEach-Object {
                Copy-Item -Path $_.FullName -Destination $shaderDir -Force
            }
            Write-Success "Low-End GPU shaders installed!"
            Write-Info ""
            Write-Info "Recommended config for mpv.conf:"
            Write-Info "glsl-shaders=~~/shaders/Anime4K_Upscale_CNN_x2_S.glsl;~~/shaders/Anime4K_Auto_Downscale_Pre_x2.glsl"
        } else {
            Write-Info "Installing High-End GPU shaders (Full set)..."
            Get-ChildItem -Path $tempDir -Filter "*.glsl" | ForEach-Object {
                Copy-Item -Path $_.FullName -Destination $shaderDir -Force
            }
            Write-Success "High-End GPU shaders installed!"
            Write-Info ""
            Write-Info "Recommended config for mpv.conf (Mode A+A HQ):"
            Write-Info "glsl-shaders=~~/shaders/Anime4K_Clamp_Highlights.glsl;~~/shaders/Anime4K_Restore_CNN_VL.glsl;~~/shaders/Anime4K_Upscale_CNN_x2_VL.glsl;~~/shaders/Anime4K_Restore_CNN_M.glsl;~~/shaders/Anime4K_Auto_Downscale_Pre_x2.glsl;~~/shaders/Anime4K_Upscale_CNN_x2_M.glsl"
        }
        
        Remove-Item -Path $tempDir -Recurse -Force
        
        Write-Info ""
        Write-Info "Shaders installed to: $shaderDir"
        Write-Info ""
        Write-Info "To customize shaders, edit: %APPDATA%\mpv\mpv.conf"
        Write-Info "More info: https://github.com/bloc97/Anime4K/blob/master/md/GLSL_Instructions_Windows_MPV.md"
    } catch {
        Write-Error "Failed to install Anime4K shaders: $_"
        Write-Info ""
        Write-Info "You can manually download them from:"
        Write-Info "https://github.com/bloc97/Anime4K/releases"
        Write-Info ""
        Write-Info "Instructions:"
        Write-Info "1. Download Anime4K_v4.0.zip"
        Write-Info "2. Extract .glsl files to: $shaderDir"
        Write-Info "3. Configure mpv.conf with appropriate shaders for your GPU"
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
    Write-Info ""
    Write-Info "Adding $InstallDir to your PATH..."
    [Environment]::SetEnvironmentVariable('Path', "$env:Path;$InstallDir", 'User')
    # Also update current session
    $env:Path = "$env:Path;$InstallDir"
    Write-Success "Added to PATH! You may need to restart your terminal."
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
