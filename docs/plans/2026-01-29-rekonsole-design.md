# rekonsole Design

## Purpose

A CLI tool that establishes a raw USB serial console connection, provides an interactive session, and records all raw I/O to a timestamped file.

## CLI Interface

```
rekonsole <device> [flags]

Arguments:
  <device>    Serial device path (e.g. /dev/ttyUSB0)

Flags:
  --baud, -b    Baud rate (default: 115200)
  --log, -l     Enable/disable logging (default: true)
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
- Log file creation fails -> warning, continue without logging

## Exit Behavior

- Ctrl+C / SIGTERM -> graceful shutdown: close serial, flush log, restore terminal
- No custom escape sequence needed

## Log File

- Filename: `YYYYMMDD-HHMMSS-session.log`
- Each line is timestamped with millisecond precision and tagged with direction:
  - `[TX]` — user input (stdin to serial)
  - `[RX]` — serial output (serial to stdout, includes device echo)
- Format: `2026-02-03 10:30:45.123 [TX] command here`
- Line-buffered via `lineLogger` struct; partial lines flushed on shutdown

## Project Structure

```
rekonsole/
├── main.go
├── go.mod
└── go.sum
```
