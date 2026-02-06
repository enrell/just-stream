#!/bin/bash
set -e

REPO="kokoro/just-stream"
INSTALL_DIR_LINUX="${HOME}/.local/bin"
INSTALL_DIR_WIN="${LOCALAPPDATA:-$HOME/AppData/Local}/bin"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_info() {
    echo -e "${BLUE}$1${NC}"
}

print_success() {
    echo -e "${GREEN}$1${NC}"
}

print_warning() {
    echo -e "${YELLOW}$1${NC}"
}

print_error() {
    echo -e "${RED}$1${NC}"
}

get_latest_release() {
    curl --silent "https://api.github.com/repos/${REPO}/releases/latest" | 
    grep '"tag_name":' |
    sed -E 's/.*"([^"]+)".*/\1/'
}

detect_os() {
    case "$(uname -s)" in
        Linux*)     echo "linux";;
        Darwin*)    echo "darwin";;
        CYGWIN*)    echo "windows";;
        MINGW*)     echo "windows";;
        MSYS*)      echo "windows";;
        *)          echo "unknown";;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64*)    echo "amd64";;
        amd64*)     echo "amd64";;
        arm64*)     echo "arm64";;
        aarch64*)   echo "arm64";;
        *)          echo "amd64";;
    esac
}

detect_distro() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        echo "$ID"
    elif [ -f /etc/redhat-release ]; then
        echo "rhel"
    elif [ -f /etc/arch-release ]; then
        echo "arch"
    else
        echo "unknown"
    fi
}

# Check if a command exists
check_command() {
    command -v "$1" >/dev/null 2>&1
}

# Ask user for confirmation (returns 0 for yes, 1 for no)
ask_user() {
    local prompt="$1"
    local response
    
    while true; do
        read -r -p "${prompt} [Y/n]: " response
        response=${response:-Y}
        case "$response" in
            [Yy]*) return 0 ;;
            [Nn]*) return 1 ;;
            *) echo "Please answer y or n." ;;
        esac
    done
}

# Set environment variable for custom binary path
set_custom_path() {
    local dep="$1"
    local env_var=""
    local default_path=""
    
    case "$dep" in
        mpv)
            env_var="MPV_PATH"
            default_path="/usr/bin/mpv"
            ;;
        ffmpeg)
            env_var="FFMPEG_PATH"
            default_path="/usr/bin/ffmpeg"
            ;;
        *)
            return 1
            ;;
    esac
    
    print_info ""
    print_info "Setting custom path for $dep"
    read -r -p "Enter the full path to $dep binary [${default_path}]: " custom_path
    custom_path=${custom_path:-$default_path}
    
    # SECURITY: Validate path - only allow safe characters (alphanumeric, /, _, -, ., space)
    if [[ ! "$custom_path" =~ ^[a-zA-Z0-9_/.\ -]+$ ]]; then
        print_error "Invalid path. Only alphanumeric characters, /, _, -, ., and spaces are allowed."
        return 1
    fi
    
    # SECURITY: Prevent path traversal
    if [[ "$custom_path" == *".."* ]]; then
        print_error "Path cannot contain '..' (path traversal detected)."
        return 1
    fi
    
    if [ -x "$custom_path" ]; then
        # Determine shell config file
        local shell_config=""
        if [ -f "$HOME/.bashrc" ]; then
            shell_config="$HOME/.bashrc"
        elif [ -f "$HOME/.zshrc" ]; then
            shell_config="$HOME/.zshrc"
        elif [ -f "$HOME/.bash_profile" ]; then
            shell_config="$HOME/.bash_profile"
        fi
        
        if [ -n "$shell_config" ]; then
            # Add header comment for easy identification
            echo "" >> "$shell_config"
            echo "# just-stream: Custom path for ${dep}" >> "$shell_config"
            echo "export ${env_var}=\"${custom_path}\"" >> "$shell_config"
            print_success "Added ${env_var}=${custom_path} to ${shell_config}"
            print_info "Run 'source ${shell_config}' to apply the changes"
        else
            print_warning "Could not find shell config file. Please manually add:"
            print_warning "  export ${env_var}=\"${custom_path}\""
        fi
        
        # Export for current session
        export "${env_var}=${custom_path}"
        return 0
    else
        print_error "Binary not found or not executable at: $custom_path"
        return 1
    fi
}

# Check and install dependencies
check_and_install_dependencies() {
    local os="$1"
    
    print_info ""
    print_info "Checking dependencies..."
    
    local missing_deps=()
    
    if ! check_command mpv && [ -z "$MPV_PATH" ]; then
        missing_deps+=("mpv")
    fi
    
    if ! check_command ffmpeg && [ -z "$FFMPEG_PATH" ]; then
        missing_deps+=("ffmpeg")
    fi
    
    if ! check_command curl; then
        missing_deps+=("curl")
    fi
    
    if [ ${#missing_deps[@]} -eq 0 ]; then
        print_success "✓ All dependencies are installed"
        return 0
    fi
    
    print_warning "Missing dependencies: ${missing_deps[*]}"
    
    # Handle each missing dependency
    for dep in "${missing_deps[@]}"; do
        print_info ""
        print_info "Dependency: $dep"
        
        if [ "$dep" = "curl" ]; then
            # curl is required for the installer itself
            print_error "curl is required to download just-stream. Please install curl first."
            exit 1
        fi
        
        # Ask to install
        if ask_user "Install $dep using package manager?"; then
            if [ "$os" = "linux" ]; then
                install_single_dep_linux "$dep"
            elif [ "$os" = "darwin" ]; then
                install_single_dep_brew "$dep"
            elif [ "$os" = "windows" ]; then
                install_single_dep_windows "$dep"
            fi
        else
            # Offer to set custom path
            if ask_user "Do you have $dep installed at a custom location?"; then
                set_custom_path "$dep"
            else
                print_warning "WARNING: $dep is required for just-stream to work properly."
                print_warning "You can set the path later by adding to your shell config:"
                if [ "$dep" = "mpv" ]; then
                    print_warning "  export MPV_PATH=/path/to/mpv"
                elif [ "$dep" = "ffmpeg" ]; then
                    print_warning "  export FFMPEG_PATH=/path/to/ffmpeg"
                fi
            fi
        fi
    done
}

# Install single dependency on Linux
install_single_dep_linux() {
    local dep="$1"
    local distro=$(detect_distro)
    
    print_info "Installing $dep..."
    
    if check_command apt; then
        sudo apt update && sudo apt install -y "$dep" || {
            print_warning "apt installation failed, trying alternative..."
            install_with_homebrew_single "$dep"
        }
    elif check_command dnf; then
        sudo dnf install -y "$dep" || {
            print_warning "dnf installation failed, trying alternative..."
            install_with_homebrew_single "$dep"
        }
    elif check_command yum; then
        sudo yum install -y "$dep" || {
            print_warning "yum installation failed, trying alternative..."
            install_with_homebrew_single "$dep"
        }
    elif check_command pacman; then
        sudo pacman -S --noconfirm "$dep" || {
            print_warning "pacman installation failed, trying alternative..."
            install_with_homebrew_single "$dep"
        }
    elif check_command zypper; then
        sudo zypper install -y "$dep" || {
            print_warning "zypper installation failed, trying alternative..."
            install_with_homebrew_single "$dep"
        }
    elif check_command emerge; then
        sudo emerge "$dep" || {
            print_warning "emerge installation failed, trying alternative..."
            install_with_homebrew_single "$dep"
        }
    elif check_command apk; then
        sudo apk add "$dep" || {
            print_warning "apk installation failed, trying alternative..."
            install_with_homebrew_single "$dep"
        }
    elif check_command xbps-install; then
        sudo xbps-install -y "$dep" || {
            print_warning "xbps installation failed, trying alternative..."
            install_with_homebrew_single "$dep"
        }
    elif check_command snap; then
        sudo snap install "$dep" || {
            print_warning "snap installation failed, trying alternative..."
            install_with_homebrew_single "$dep"
        }
    elif check_command flatpak; then
        if [ "$dep" = "mpv" ]; then
            flatpak install -y flathub io.mpv.Mpv
        else
            print_warning "flatpak doesn't support $dep directly"
            install_with_homebrew_single "$dep"
        fi
    else
        print_warning "No supported package manager found, trying Homebrew..."
        install_with_homebrew_single "$dep"
    fi
}

# Install single dependency with Homebrew
install_single_dep_brew() {
    local dep="$1"
    print_info "Installing $dep via Homebrew..."
    brew install "$dep"
}

# Install single dependency on Windows (Git Bash/MSYS2)
install_single_dep_windows() {
    local dep="$1"
    print_info "Installing $dep..."
    
    if check_command pacman; then
        # MSYS2 environment
        local pkg="$dep"
        if [ "$dep" = "mpv" ]; then
            pkg="mingw-w64-x86_64-mpv"
        elif [ "$dep" = "ffmpeg" ]; then
            pkg="mingw-w64-x86_64-ffmpeg"
        fi
        pacman -S --noconfirm "$pkg" || {
            print_warning "pacman installation failed."
            print_error "Please install $dep manually (e.g. via Scoop: scoop install $dep)"
        }
    elif check_command scoop; then
        scoop install "$dep" || {
            print_warning "scoop installation failed."
            print_error "Please install $dep manually."
        }
    elif check_command choco; then
        choco install "$dep" -y || {
            print_warning "choco installation failed."
            print_error "Please install $dep manually."
        }
    else
        print_error "No supported package manager found (pacman, scoop, choco)."
        print_error "Please install $dep manually."
    fi
}

# Fallback to homebrew for single dependency
install_with_homebrew_single() {
    local dep="$1"
    print_info "Attempting to install $dep via Homebrew..."
    
    if check_command brew; then
        brew install "$dep"
    else
        print_warning ""
        print_warning "SECURITY NOTICE: About to download and execute Homebrew installer"
        print_warning "from https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh"
        print_warning "This is the official Homebrew installer, but always verify the source."
        print_warning ""
        
        if ask_user "Continue with Homebrew installation?"; then
            print_info "Installing Homebrew first..."
            /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
            if check_command brew; then
                brew install "$dep"
            else
                print_error "Failed to install Homebrew. Please install $dep manually."
            fi
        else
            print_error "Homebrew installation cancelled. Please install $dep manually."
        fi
    fi
}

install_linux() {
    local version="$1"
    local arch="$2"
    local install_dir="${INSTALL_DIR_LINUX}"
    
    # Check and install dependencies first
    check_and_install_dependencies "linux"
    
    print_info ""
    print_info "Installing just-stream ${version} for Linux ${arch}..."
    
    # Create install directory
    mkdir -p "${install_dir}"
    
    # Download binary
    local download_url="https://github.com/${REPO}/releases/download/${version}/just-stream-linux-${arch}"
    print_info "Downloading from: ${download_url}"
    
    if ! curl -L -o "${install_dir}/just-stream" "${download_url}"; then
        print_warning "Failed to download with arch suffix. Trying generic binary name..."
        curl -L -o "${install_dir}/just-stream" "https://github.com/${REPO}/releases/download/${version}/just-stream"
    fi
    
    # Make executable
    chmod +x "${install_dir}/just-stream"
    
    # Check if directory is in PATH
    if [[ ":$PATH:" != *":${install_dir}:"* ]]; then
        print_warning ""
        print_warning "${install_dir} is not in your PATH"
        print_warning "Add the following to your ~/.bashrc or ~/.zshrc:"
        print_warning "    export PATH=\"${install_dir}:\$PATH\""
    fi
    
    print_success ""
    print_success "✓ just-stream installed to ${install_dir}/just-stream"
    print_info ""
    print_info "Usage: just-stream [magnet-link]"
    
    # Offer to setup Anime4K
    if ask_user "Would you like to install Anime4K shaders for enhanced quality?"; then
        setup_anime4k
    fi
}

install_darwin() {
    local version="$1"
    local arch="$2"
    local install_dir="${INSTALL_DIR_LINUX}"
    
    # Check and install dependencies first
    check_and_install_dependencies "darwin"
    
    print_info ""
    print_info "Installing just-stream ${version} for macOS ${arch}..."
    
    # Create install directory
    mkdir -p "${install_dir}"
    
    # Download binary
    local download_url="https://github.com/${REPO}/releases/download/${version}/just-stream-darwin-${arch}"
    print_info "Downloading from: ${download_url}"
    
    if ! curl -L -o "${install_dir}/just-stream" "${download_url}"; then
        print_warning "Failed to download with arch suffix. Trying generic binary name..."
        curl -L -o "${install_dir}/just-stream" "https://github.com/${REPO}/releases/download/${version}/just-stream"
    fi
    
    # Make executable
    chmod +x "${install_dir}/just-stream"
    
    # Check if directory is in PATH
    if [[ ":$PATH:" != *":${install_dir}:"* ]]; then
        print_warning ""
        print_warning "${install_dir} is not in your PATH"
        print_warning "Add the following to your ~/.bashrc or ~/.zshrc:"
        print_warning "    export PATH=\"${install_dir}:\$PATH\""
    fi
    
    print_success ""
    print_success "✓ just-stream installed to ${install_dir}/just-stream"
    print_info ""
    print_info "Usage: just-stream [magnet-link]"
    
    # Offer to setup Anime4K
    if ask_user "Would you like to install Anime4K shaders for enhanced quality?"; then
        setup_anime4k
    fi
}

setup_anime4k() {
    print_info ""
    print_info "Setting up Anime4K shaders..."
    
    local shader_dir="${HOME}/.config/mpv/shaders"
    
    mkdir -p "${shader_dir}"
    
    print_info "Downloading Anime4K shaders to ${shader_dir}..."
    
    if curl -L -o /tmp/Anime4K.zip "https://github.com/bloc97/Anime4K/releases/download/v4.0.1/Anime4K_v4.0.1.zip"; then
        unzip -o /tmp/Anime4K.zip -d "${shader_dir}"
        rm -f /tmp/Anime4K.zip
        print_success "✓ Anime4K shaders installed"
    else
        print_error "Failed to download Anime4K shaders. You can manually download them from:"
        print_error "https://github.com/bloc97/Anime4K/releases"
    fi
}

install_windows() {
    local version="$1"
    local arch="$2"
    
    # Check if running in Git Bash/MSYS — otherwise redirect to PowerShell installer
    if [ -z "$MSYSTEM" ] && [ -z "$MINGW_PREFIX" ]; then
        print_warning ""
        print_warning "Windows installation is recommended via PowerShell."
        print_info "Run the following in PowerShell:"
        print_info ""
        print_info '  Invoke-WebRequest -Uri "https://raw.githubusercontent.com/kokoro/just-stream/main/install.ps1" -OutFile install.ps1; .\install.ps1'
        print_info ""
        print_warning "If you are in Git Bash/MSYS, re-run this script from that shell."
        return
    fi
    
    print_info ""
    print_info "Detected Git Bash/MSYS environment."
    
    # Check and install dependencies
    check_and_install_dependencies "windows"
    
    print_info ""
    print_info "Installing just-stream ${version} for Windows ${arch}..."
    
    local install_dir="${INSTALL_DIR_WIN}"
    mkdir -p "${install_dir}"
    
    # Download binary
    local download_url="https://github.com/${REPO}/releases/download/${version}/just-stream-windows-${arch}.exe"
    print_info "Downloading from: ${download_url}"
    
    if ! curl -L -o "${install_dir}/just-stream.exe" "${download_url}"; then
        print_warning "Failed to download with arch suffix. Trying generic binary name..."
        curl -L -o "${install_dir}/just-stream.exe" "https://github.com/${REPO}/releases/download/${version}/just-stream.exe"
    fi
    
    print_success ""
    print_success "✓ just-stream installed to ${install_dir}/just-stream.exe"
    print_warning ""
    print_warning "Remember to add ${install_dir} to your Windows PATH"
}

main() {
    echo ""
    print_info "just-stream installer"
    print_info "====================="
    print_info ""
    
    # Detect OS
    OS=$(detect_os)
    ARCH=$(detect_arch)
    
    print_info "Detected: ${OS} ${ARCH}"
    print_info ""
    
    # Get latest version
    print_info "Fetching latest release..."
    VERSION=$(get_latest_release)
    
    if [ -z "${VERSION}" ]; then
        print_error "Error: Could not determine latest version"
        print_error "Please check your internet connection"
        exit 1
    fi
    
    print_info "Latest version: ${VERSION}"
    
    # Install based on OS
    case "${OS}" in
        linux)
            install_linux "${VERSION}" "${ARCH}"
            ;;
        darwin)
            install_darwin "${VERSION}" "${ARCH}"
            ;;
        windows)
            install_windows "${VERSION}" "${ARCH}"
            ;;
        *)
            print_error "Error: Unsupported operating system: ${OS}"
            print_error "Supported: Linux, macOS, Windows"
            exit 1
            ;;
    esac
}

main "$@"
