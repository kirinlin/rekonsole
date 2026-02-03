# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
go build -o rekonsole .       # build binary
./rekonsole /dev/ttyUSB0      # run with default 115200 baud
./rekonsole /dev/ttyUSB0 -b 9600  # custom baud rate
```

No test suite, Makefile, or linter configured yet.

## Architecture

Single-file Go CLI (`main.go`) that opens a raw serial port connection and provides an interactive terminal session.

**Core flow:** Two goroutines handle bidirectional I/O — one pipes stdin to the serial port, the other pipes serial output to stdout. Each goroutine feeds data to a `lineLogger` that buffers bytes, and on each newline writes a timestamped line tagged with `[TX]` (user input) or `[RX]` (serial output) to the session log file. The terminal is set to raw mode via `golang.org/x/term` so keystrokes pass through immediately. Graceful shutdown on SIGINT/SIGTERM restores the terminal, flushes both loggers, and closes the port.

**Key dependencies:** `go.bug.st/serial` for serial port access, `golang.org/x/term` for raw terminal mode.
