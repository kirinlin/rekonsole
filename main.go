package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.bug.st/serial"
	"golang.org/x/term"
)

// sensitivePrompts lists patterns that trigger log redaction.
// When an RX line contains any of these (case-insensitive), logging
// is suppressed until the password entry is complete.
var sensitivePrompts = []string{
	"password:",
	"passphrase:",
	"secret:",
}

// escapeSequences maps ANSI escape sequences to readable Unicode symbols.
var escapeSequences = map[string]string{
	// Arrow keys
	"\x1b[A": "↑", // Up
	"\x1b[B": "↓", // Down
	"\x1b[C": "→", // Right
	"\x1b[D": "←", // Left
	// Navigation keys
	"\x1b[H":  "⇱", // Home
	"\x1b[F":  "⇲", // End
	"\x1b[1~": "⇱", // Home (alternate)
	"\x1b[4~": "⇲", // End (alternate)
	"\x1b[5~": "⇞", // Page Up
	"\x1b[6~": "⇟", // Page Down
	"\x1b[2~": "⎀", // Insert
	"\x1b[3~": "⌦", // Delete
	// Function keys
	"\x1bOP":   "[F1]",
	"\x1bOQ":   "[F2]",
	"\x1bOR":   "[F3]",
	"\x1bOS":   "[F4]",
	"\x1b[15~": "[F5]",
	"\x1b[17~": "[F6]",
	"\x1b[18~": "[F7]",
	"\x1b[19~": "[F8]",
	"\x1b[20~": "[F9]",
	"\x1b[21~": "[F10]",
	"\x1b[23~": "[F11]",
	"\x1b[24~": "[F12]",
	// Control characters
	"\x1b": "␛", // Escape key alone
}

// convertEscapeSequences replaces ANSI escape sequences with readable Unicode symbols.
func convertEscapeSequences(data []byte) []byte {
	result := string(data)
	// Process longer sequences first to avoid partial matches
	// (e.g., "\x1b[15~" before "\x1b")
	orderedKeys := []string{
		"\x1b[15~", "\x1b[17~", "\x1b[18~", "\x1b[19~",
		"\x1b[20~", "\x1b[21~", "\x1b[23~", "\x1b[24~",
		"\x1bOP", "\x1bOQ", "\x1bOR", "\x1bOS",
		"\x1b[1~", "\x1b[2~", "\x1b[3~", "\x1b[4~", "\x1b[5~", "\x1b[6~",
		"\x1b[A", "\x1b[B", "\x1b[C", "\x1b[D",
		"\x1b[H", "\x1b[F",
		"\x1b",
	}
	for _, seq := range orderedKeys {
		if symbol, ok := escapeSequences[seq]; ok {
			result = strings.ReplaceAll(result, seq, symbol)
		}
	}
	return []byte(result)
}

// lineLogger buffers incoming bytes and writes timestamped, prefixed lines
// to the underlying file whenever a newline is encountered.
type lineLogger struct {
	mu           sync.Mutex
	prefix       string
	out          *os.File
	buf          []byte
	passwordMode *atomic.Bool // shared flag: suppresses logging when true
}

func (l *lineLogger) Write(p []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf = append(l.buf, p...)
	for {
		// Find the earliest line delimiter: either \n or \r.
		// Raw terminal sends \r on Enter; serial devices typically send \r\n.
		idxN := bytes.IndexByte(l.buf, '\n')
		idxR := bytes.IndexByte(l.buf, '\r')
		idx := idxN
		if idx < 0 || (idxR >= 0 && idxR < idx) {
			idx = idxR
		}
		if idx < 0 {
			break
		}
		line := l.buf[:idx]
		delim := l.buf[idx]

		if l.passwordMode != nil && !l.passwordMode.Load() {
			ts := time.Now().Format("2006-01-02 15:04:05.000")
			// Convert escape sequences to readable Unicode symbols for logging
			displayLine := convertEscapeSequences(line)
			fmt.Fprintf(l.out, "%s [%s] %s\n", ts, l.prefix, displayLine)
		}

		l.buf = l.buf[idx+1:]
		// Skip the paired byte of a \r\n or \n\r sequence to avoid a duplicate empty line.
		if len(l.buf) > 0 && (l.buf[0] == '\n' || l.buf[0] == '\r') && l.buf[0] != delim {
			l.buf = l.buf[1:]
		}
	}
}

func (l *lineLogger) Flush() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.buf) > 0 {
		line := bytes.TrimRight(l.buf, "\r\n")
		if l.passwordMode == nil || !l.passwordMode.Load() {
			ts := time.Now().Format("2006-01-02 15:04:05.000")
			// Convert escape sequences to readable Unicode symbols for logging
			displayLine := convertEscapeSequences(line)
			fmt.Fprintf(l.out, "%s [%s] %s\n", ts, l.prefix, displayLine)
		}
		l.buf = nil
	}
}

// matchesSensitivePrompt checks if data contains a password prompt pattern.
func matchesSensitivePrompt(data []byte) bool {
	lower := strings.ToLower(string(data))
	for _, pat := range sensitivePrompts {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}

var version = "0.1.0"

func main() {
	baud := flag.Int("baud", 115200, "baud rate")
	flag.IntVar(baud, "b", 115200, "baud rate (shorthand)")
	noLog := flag.Bool("no-log", false, "disable I/O logging")
	flag.BoolVar(noLog, "l", false, "disable I/O logging (shorthand)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.BoolVar(showVersion, "v", false, "print version and exit (shorthand)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: rekonsole <device> [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Establish a raw USB serial console connection.\n\n")
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  <device>    Serial device path (e.g. /dev/ttyUSB0)\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Printf("rekonsole %s\n", version)
		return
	}

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}
	device := flag.Arg(0)

	// Open serial port
	mode := &serial.Mode{
		BaudRate: *baud,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}
	port, err := serial.Open(device, mode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot open %s: %v\n", device, err)
		os.Exit(1)
	}
	defer port.Close()

	// Set a read timeout so Read returns periodically even when no data
	// arrives. Without this, Read blocks forever if the device powers off.
	port.SetReadTimeout(500 * time.Millisecond)

	fmt.Fprintf(os.Stderr, "rekonsole %s — Connected to %s at %d baud. Press Ctrl+C to exit.\n", version, device, *baud)

	// Set up log file
	var logFile *os.File
	if !*noLog {
		filename := time.Now().Format("20060102-150405") + "-session.log"
		logFile, err = os.Create(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cannot create log file: %v\n", err)
		} else {
			defer logFile.Close()
			fmt.Fprintf(os.Stderr, "Logging to %s\n", filename)
		}
	}

	// Put terminal into raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot set raw mode: %v\n", err)
	} else {
		defer term.Restore(int(os.Stdin.Fd()), oldState)
	}

	// Shared password mode flag: when true, both TX and RX logging are suppressed.
	passwordMode := &atomic.Bool{}

	// Set up line loggers for TX/RX
	var txLog, rxLog *lineLogger
	if logFile != nil {
		txLog = &lineLogger{prefix: "TX", out: logFile, passwordMode: passwordMode}
		rxLog = &lineLogger{prefix: "RX", out: logFile, passwordMode: passwordMode}
	}

	// Handle signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})

	// rxBuf accumulates RX data for sensitive prompt detection.
	// It is reset on each newline.
	var rxBufMu sync.Mutex
	var rxBuf []byte

	// serial -> stdout (+ log)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := port.Read(buf)
			if n > 0 {
				os.Stdout.Write(buf[:n])

				// Detect password prompts in the RX stream.
				rxBufMu.Lock()
				rxBuf = append(rxBuf, buf[:n]...)
				if matchesSensitivePrompt(rxBuf) && !passwordMode.Load() {
					passwordMode.Store(true)
				}
				// Reset detection buffer on newline (password entry complete).
				if bytes.ContainsAny(buf[:n], "\r\n") {
					if passwordMode.Load() {
						// The newline after a password prompt means the
						// user has finished entering the password.
						passwordMode.Store(false)
					}
					rxBuf = nil
				}
				rxBufMu.Unlock()

				if rxLog != nil {
					rxLog.Write(buf[:n])
				}
			}
			if err != nil {
				if err != io.EOF {
					// Restore terminal before printing error
					if oldState != nil {
						term.Restore(int(os.Stdin.Fd()), oldState)
					}
					fmt.Fprintf(os.Stderr, "\nSerial read error: %v\n", err)
				}
				close(done)
				return
			}
			// On timeout (0 bytes, nil error), just loop back.
			// The timeout exists only to prevent Read from blocking
			// forever, so that port.Close() on signal can take effect.
		}
	}()

	// stdin -> serial (+ log)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				// In raw mode, Ctrl+C arrives as byte 0x03 instead of
				// generating SIGINT (especially on Windows). Detect it
				// and trigger shutdown.
				if bytes.ContainsRune(buf[:n], 0x03) {
					close(done)
					return
				}
				port.Write(buf[:n])
				if txLog != nil {
					txLog.Write(buf[:n])
				}
			}
			if err != nil {
				close(done)
				return
			}
		}
	}()

	// Wait for signal or connection end
	select {
	case <-sigCh:
		// Close the port immediately so blocked Read returns an error
		// instead of waiting for the next timeout cycle.
		port.Close()
	case <-done:
	}

	// Flush any partial lines before exit
	if txLog != nil {
		txLog.Flush()
	}
	if rxLog != nil {
		rxLog.Flush()
	}
}
