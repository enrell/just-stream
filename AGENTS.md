# just-stream - Agent Instructions

## Build Commands

```bash
# Build the binary
make build

# Install to ~/.local/bin (or $BINDIR)
make install

# Windows cross-compile
make windows

# Clean build artifacts
make clean
```

## Testing

This project uses Go's built-in testing. No specific commands found - check for tests with:
```bash
go test ./...
```

Run a specific test:
```bash
go test -run <TestName> ./<package>
```

## Code Style Guidelines

### Imports
- Group imports: stdlib packages, third-party, internal (separated by blank lines)
- Use fully qualified internal imports: `github.com/enrell/just-stream/<module>`
- Keep imports organized alphabetically within groups

### Formatting & Types
- Use `gofmt` or Go's standard formatting
- Explicit types for exported fields and constants
- Prefer explicit returns for clarity
- Use struct tags for JSON (`json:"field_name,omitempty"`)

### Naming Conventions
- CamelCase for exported, camelCase for unexported
- Acronyms preserve case: `IPC`, `MPV`, `TUI`
- Interface names: simple noun (e.g., `Player`)
- Error handling: wrap with `fmt.Errorf("context: %w", err)`

### Error Handling
- Always handle errors; never ignore with `_`
- Wrap errors with context about the operation
- Use `os.Exit(1)` only in main() for fatal errors
- Log warnings to `os.Stderr` for non-fatal issues

### Patterns
- Bubble Tea uses value copies for models; use pointers for shared state
- Use mutex (`sync.Mutex`) for concurrent access to shared fields
- Cleanup resources in defer where possible
- Platform-specific code: use build tags (`//go:build windows`)

### Configuration
- Config stored in JSON at platform-specific location
- Config field names: snake_case with `omitempty` tag
- Load returns empty Config (no error) if file doesn't exist

### TUI / Bubble Tea
- Models embed `sharedStruct` for common state
- Send messages via `p.Send()` from background code
- Use `lipgloss` for styling, define styles at package level
- Keyboard: `j/k` navigate, `enter` select, `q` quit, `ctrl+s` settings

### MPV Integration
- Use JSON IPC for mpv communication
- Windows paths: check common mpv locations with `os.Stat()`
- Include mpv.net path: `C:\Users\<USERNAME>\AppData\Local\Programs\mpv.net\mpvnet.exe`
- Always cleanup IPC connections with `conn.Close()`

### Critical Path Considerations
- Stream server and mpv IPC are critical for functionality
- Implement retries for network operations
- User input validation at TUI boundaries

### When in Doubt
- Follow existing patterns in player/, tui/, config/
- Keep functions focused and single-responsibility
- Comment the "Why" for non-obvious architectural decisions
