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
go build -o rekonsole .
```

## Usage

```bash
rekonsole <device> [flags]
```

**Arguments:**

| Argument | Description |
|----------|-------------|
| `<device>` | Serial device path (e.g. `/dev/ttyUSB0`) |

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `-b`, `-baud` | `115200` | Baud rate |
| `-l`, `-no-log` | `false` | Disable I/O logging |

**Examples:**

```bash
rekonsole /dev/ttyUSB0                # connect at 115200 baud
rekonsole /dev/ttyUSB0 -b 9600       # connect at 9600 baud
rekonsole /dev/ttyACM0 -no-log       # connect without logging
```

All I/O is automatically recorded to a file named `YYYYMMDD-HHMMSS-raw.txt` in the current directory unless `-no-log` is specified.

Press `Ctrl+C` to disconnect.

## License

[MIT](LICENSE)
