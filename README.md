# rekonsole

A raw USB serial console tool with automatic I/O logging.

## Install

```bash
go install github.com/kirinlin/rekonsole@latest
```

Or build from source:

```bash
git clone https://github.com/kirinlin/rekonsole.git
cd rekonsole

# Linux ARM64
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o rekonsole .

# Windows
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o rekonsole.exe .

# macOS (Apple Silicon)
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o rekonsole .

# macOS (Intel)
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o rekonsole .
```

## Usage

```bash
rekonsole <device> [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `<device>` | Serial device path (e.g. `/dev/ttyUSB0`, `COM3`) |

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `-b`, `-baud` | `115200` | Baud rate |
| `-l`, `-no-log` | `false` | Disable I/O logging |
| `-v`, `-version` | | Print version and exit |

**Examples:**

```bash
# Linux / macOS
rekonsole /dev/ttyUSB0               # connect at 115200 baud
rekonsole /dev/ttyUSB0 -b 9600       # connect at 9600 baud
rekonsole /dev/ttyUSB0 -no-log       # connect without logging

# Windows
rekonsole COM3                       # connect at 115200 baud
rekonsole COM3 -b 9600               # connect at 9600 baud
```

All I/O is automatically recorded to a file named `YYYYMMDD-HHMMSS-session.log` in the current directory unless `-no-log` is specified. Each line in the log is timestamped and tagged with `[TX]` (user input) or `[RX]` (serial output):

```
2026-02-03 10:30:45.123 [TX] ls -la
2026-02-03 10:30:45.234 [RX] ls -la
2026-02-03 10:30:45.345 [RX] total 42
```

**Password redaction:** When a password prompt is detected (`password:`, `passphrase:`, or `secret:`), logging is automatically suppressed until the password entry is complete. The prompt and password are displayed normally on the terminal but do not appear in the session log.

Press `Ctrl+C` to disconnect. If the serial device is powered off or disconnected, rekonsole detects the disappearance and exits automatically.

## License

[MIT](LICENSE)
