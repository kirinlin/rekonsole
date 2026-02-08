# Timestamped TX/RX Logging

## Problem

Serial devices typically echo user input back. With raw byte logging, every keystroke appeared twice in the log — once when sent (stdin → serial) and again when echoed back (serial → stdout). The log had no timestamps or directionality, making it difficult to review sessions.

## Solution

Replace raw byte logging with a line-buffered `lineLogger` that tags each completed line with:

- A millisecond-precision timestamp
- A direction prefix: `[TX]` for user input, `[RX]` for serial output

### Log format

```
2026-02-03 10:30:45.123 [TX] ls -la
2026-02-03 10:30:45.234 [RX] ls -la
2026-02-03 10:30:45.345 [RX] total 42
2026-02-03 10:30:45.346 [RX] drwxr-xr-x  2 root root 4096 ...
```

The echoed input is still present in the `[RX]` stream but is now clearly distinguishable from the original `[TX]` entry.

## Design

### `lineLogger` struct

```go
type lineLogger struct {
    mu           sync.Mutex
    prefix       string       // "TX" or "RX"
    out          *os.File     // log file handle
    buf          []byte       // incomplete line buffer
    passwordMode *atomic.Bool // shared flag: suppresses logging when true
}
```

- **`Write(p []byte)`** — appends data to the buffer, scans for the earliest `\r` or `\n` delimiter. On each line break, flushes the buffered line with timestamp and prefix (unless `passwordMode` is true). Handles `\r\n` and `\n\r` pairs by skipping the second byte to avoid duplicate empty lines. This ensures TX lines are logged immediately when the user presses Enter in raw mode (which sends `\r`, not `\n`).
- **`Flush()`** — writes any remaining partial line on shutdown, stripping trailing `\r\n`. Respects `passwordMode`.
- Thread-safe via `sync.Mutex` since TX and RX goroutines share the same log file.

### Password redaction

When the RX stream contains a sensitive prompt pattern (`password:`, `passphrase:`, `secret:` — case-insensitive), a shared `atomic.Bool` flag is set to true. Both TX and RX loggers check this flag and suppress output while it is set. The flag is cleared when a newline is received after the prompt, indicating password entry is complete. This prevents passwords from appearing in session logs.

### Integration

Two `lineLogger` instances are created (one TX, one RX) sharing the same log file. Each goroutine writes to its respective logger after performing its primary I/O (sending to serial or writing to stdout). Both loggers are flushed after the main select loop exits, before the log file is closed.

### File naming

Changed from `*-raw.txt` to `*-session.log` to reflect the structured format.
