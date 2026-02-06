#!/bin/bash
set -e

REPO="kokoro/just-stream"
INSTALL_DIR_LINUX="${HOME}/.local/bin"
INSTALL_DIR_WIN="${LOCALAPPDATA:-$HOME/AppData/Local}/bin"

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

install_linux() {
    local version="$1"
    local arch="$2"
    local install_dir="${INSTALL_DIR_LINUX}"
    
    echo "Installing just-stream ${version} for Linux ${arch}..."
    
    # Create install directory
    mkdir -p "${install_dir}"
    
    # Download binary
    local download_url="https://github.com/${REPO}/releases/download/${version}/just-stream-linux-${arch}"
    if ! curl -L -o "${install_dir}/just-stream" "${download_url}"; then
        echo "Failed to download. Trying generic binary name..."
        curl -L -o "${install_dir}/just-stream" "https://github.com/${REPO}/releases/download/${version}/just-stream"
    fi
    
    # Make executable
    chmod +x "${install_dir}/just-stream"
    
    # Check if directory is in PATH
    if [[ ":$PATH:" != *":${install_dir}:"* ]]; then
        echo ""
        echo "⚠️  ${install_dir} is not in your PATH"
        echo "Add the following to your ~/.bashrc or ~/.zshrc:"
        echo "    export PATH=\"${install_dir}:\$PATH\""
    fi
    
    echo ""
    echo "✓ just-stream installed to ${install_dir}/just-stream"
    echo ""
    echo "Usage: just-stream [magnet-link]"
}

install_darwin() {
    local version="$1"
    local arch="$2"
    local install_dir="${INSTALL_DIR_LINUX}"
    
    echo "Installing just-stream ${version} for macOS ${arch}..."
    
    # Create install directory
    mkdir -p "${install_dir}"
    
    # Download binary
    local download_url="https://github.com/${REPO}/releases/download/${version}/just-stream-darwin-${arch}"
    if ! curl -L -o "${install_dir}/just-stream" "${download_url}"; then
        echo "Failed to download. Trying generic binary name..."
        curl -L -o "${install_dir}/just-stream" "https://github.com/${REPO}/releases/download/${version}/just-stream"
    fi
    
    # Make executable
    chmod +x "${install_dir}/just-stream"
    
    # Check if directory is in PATH
    if [[ ":$PATH:" != *":${install_dir}:"* ]]; then
        echo ""
        echo "⚠️  ${install_dir} is not in your PATH"
        echo "Add the following to your ~/.bashrc or ~/.zshrc:"
        echo "    export PATH=\"${install_dir}:\$PATH\""
    fi
    
    echo ""
    echo "✓ just-stream installed to ${install_dir}/just-stream"
    echo ""
    echo "Usage: just-stream [magnet-link]"
}

install_windows() {
    local version="$1"
    local arch="$2"
    local install_dir="${INSTALL_DIR_WIN}"
    
    echo "Installing just-stream ${version} for Windows ${arch}..."
    
    # Create install directory
    mkdir -p "${install_dir}"
    
    # Download binary
    local download_url="https://github.com/${REPO}/releases/download/${version}/just-stream-windows-${arch}.exe"
    if ! curl -L -o "${install_dir}/just-stream.exe" "${download_url}"; then
        echo "Failed to download. Trying generic binary name..."
        curl -L -o "${install_dir}/just-stream.exe" "https://github.com/${REPO}/releases/download/${version}/just-stream.exe"
    fi
    
    # Check if directory is in PATH
    if [[ ":$PATH:" != *":${install_dir}:"* ]]; then
        echo ""
        echo "⚠️  ${install_dir} is not in your PATH"
        echo "Add it to your PATH environment variable:"
        echo "    setx PATH \"%PATH%;${install_dir}\""
    fi
    
    echo ""
    echo "✓ just-stream installed to ${install_dir}/just-stream.exe"
    echo ""
    echo "Usage: just-stream.exe [magnet-link]"
}

main() {
    echo "just-stream installer"
    echo "====================="
    echo ""
    
    # Detect OS
    OS=$(detect_os)
    ARCH=$(detect_arch)
    
    echo "Detected: ${OS} ${ARCH}"
    echo ""
    
    # Get latest version
    echo "Fetching latest release..."
    VERSION=$(get_latest_release)
    
    if [ -z "${VERSION}" ]; then
        echo "Error: Could not determine latest version"
        exit 1
    fi
    
    echo "Latest version: ${VERSION}"
    echo ""
    
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
            echo "Error: Unsupported operating system: ${OS}"
            echo "Supported: Linux, macOS, Windows"
            exit 1
            ;;
    esac
}

main "$@"
