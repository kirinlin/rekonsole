# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
go build -o rekonsole .              # build binary
./rekonsole /dev/ttyUSB0             # run with default 115200 baud
./rekonsole /dev/ttyUSB0 -b 9600    # custom baud rate
./rekonsole -v                       # print version
```

Version can be overridden at build time: `go build -ldflags "-X main.version=0.2.0" -o rekonsole .`

No test suite, Makefile, or linter configured yet.

## Architecture

Single-file Go CLI (`main.go`) that opens a raw serial port connection and provides an interactive terminal session.

**Core flow:** Two goroutines handle bidirectional I/O — one pipes stdin to the serial port, the other pipes serial output to stdout. Each goroutine feeds data to a `lineLogger` that buffers bytes, splitting on both `\r` and `\n` (with `\r\n` pair dedup), and writes timestamped lines tagged with `[TX]` or `[RX]` to the session log file. The terminal is set to raw mode via `golang.org/x/term` so keystrokes pass through immediately. A 500ms read timeout on the serial port prevents Read from blocking forever, ensuring `port.Close()` on signal takes effect. Ctrl+C is detected as byte `0x03` in the stdin goroutine for cross-platform support (especially Windows Terminal where raw mode swallows SIGINT). Graceful shutdown on Ctrl+C or SIGINT/SIGTERM explicitly closes the port (unblocking any pending read), restores the terminal, and flushes both loggers.

**Key dependencies:** `go.bug.st/serial` for serial port access, `golang.org/x/term` for raw terminal mode.
