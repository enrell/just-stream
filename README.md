# just-stream

A TUI-based torrent streaming application that plays torrents directly in mpv with Anime4K support.

## Features

- **TUI Interface**: Beautiful terminal interface powered by [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- **Stream to mpv**: Plays torrents directly without downloading to disk
- **Playlist Support**: Stream all episodes with native mpv playlist navigation (Shift+>/<)
- **RAM-Only Storage**: All torrent data stored in memory, nothing written to disk
- **Anime4K**: Automatic upscaling for anime content
- **Cross-Platform**: Works on Linux and Windows
- **Proxy Support**: SOCKS5 and HTTP proxy support for torrent connections
- **Persistent Config**: Save mpv path preferences

## Installation

### Quick Install (Linux/macOS)

```bash
curl -sSL https://raw.githubusercontent.com/kokoro/just-stream/main/install.sh | bash
```

### Quick Install (Windows PowerShell)

```powershell
irm https://raw.githubusercontent.com/kokoro/just-stream/main/install.ps1 | iex
```

### Manual Installation

#### Linux

```bash
# Download latest release
curl -L -o just-stream https://github.com/kokoro/just-stream/releases/latest/download/just-stream
chmod +x just-stream
sudo mv just-stream /usr/local/bin/
```

#### Windows

Download `just-stream.exe` from the [releases page](https://github.com/kokoro/just-stream/releases) and add it to your PATH.

### Build from Source

```bash
git clone https://github.com/kokoro/just-stream.git
cd just-stream
make install
```

## Requirements

- [mpv](https://mpv.io/) with Anime4K shaders installed
- Go 1.21+ (for building from source)

### Anime4K Setup

```bash
# Linux/macOS
mkdir -p ~/.config/mpv/shaders
cd ~/.config/mpv/shaders
curl -L -o Anime4K.zip https://github.com/bloc97/Anime4K/releases/download/v4.0.1/Anime4K_v4.0.1.zip
unzip Anime4K.zip
```

## Usage

```bash
# Interactive mode
just-stream

# With magnet link
just-stream "magnet:?xt=urn:btih:..."

# With proxy
just-stream --proxy socks5://127.0.0.1:1080 "magnet:?xt=urn:btih:..."
```

### Keyboard Shortcuts

- **Input Screen**: Paste magnet link
- **File List**: `j/k` navigate, `enter` play, `a` stream all
- **Playback**: `q` quit, `ctrl+s` open settings
- **mpv**: `Shift+>` next episode, `Shift+<` previous episode

### Configuration

Press `ctrl+s` in the TUI to configure:
- **mpv path**: Set custom mpv binary location

Config is saved to:
- Linux/macOS: `~/.config/just-stream/config.json`
- Windows: `%APPDATA%\just-stream\config.json`

## Building

```bash
# Linux
make build

# Windows (cross-compile)
make windows

# Install to ~/.local/bin
make install
```

## License

MIT License - see [LICENSE](LICENSE)
