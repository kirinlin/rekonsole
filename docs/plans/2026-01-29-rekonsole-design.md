# rekonsole Design

## Purpose

A CLI tool that establishes a raw USB serial console connection, provides an interactive session, and records all raw I/O to a timestamped file.

## CLI Interface

```
rekonsole <device> [flags]

Arguments:
  <device>    Serial device path (e.g. /dev/ttyUSB0)

Flags:
  --baud, -b      Baud rate (default: 115200)
  --no-log, -l    Disable I/O logging
  --version, -v   Print version and exit
```

## Architecture

Single `main.go` file. Two goroutines handle bidirectional I/O:

- stdin -> serial port (fed to TX `lineLogger`)
- serial port -> stdout (fed to RX `lineLogger`)

Terminal is put into raw mode so keystrokes pass through immediately.

## Dependencies

- `go.bug.st/serial` — serial port access
- `golang.org/x/term` — raw terminal mode
- Standard library for everything else

## Error Handling

- Device not found / can't open -> error message, exit 1
- Serial connection drops -> print notice, exit cleanly
- Device disappears (powered off) -> detected via `os.Stat` check on read timeout, exit cleanly with notice
- Log file creation fails -> warning, continue without logging

## Exit Behavior

- Ctrl+C / SIGTERM -> graceful shutdown: explicitly close serial port (unblocks any pending read), flush log, restore terminal
- Device power-off -> 500ms read timeout detects device disappearance, exit cleanly
- No custom escape sequence needed

## Log File

- Filename: `YYYYMMDD-HHMMSS-session.log`
- Each line is timestamped with millisecond precision and tagged with direction:
  - `[TX]` — user input (stdin to serial)
  - `[RX]` — serial output (serial to stdout, includes device echo)
- Format: `2026-02-03 10:30:45.123 [TX] command here`
- Line-buffered via `lineLogger` struct; partial lines flushed on shutdown

## Version

The version is stored as a `var` in `main.go` (currently `0.1.0`) and can be overridden at build time:

```bash
go build -ldflags "-X main.version=0.2.0" -o rekonsole .
```

The version is displayed via `-v`/`--version` and in the startup banner.

## Project Structure

```
rekonsole/
├── main.go
├── go.mod
└── go.sum
```
