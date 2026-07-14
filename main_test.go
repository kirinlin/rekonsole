package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestConvertEscapeSequences(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"up arrow", "\x1b[A", "↑"},
		{"down arrow", "\x1b[B", "↓"},
		{"right arrow", "\x1b[C", "→"},
		{"left arrow", "\x1b[D", "←"},
		{"home", "\x1b[H", "⇱"},
		{"end", "\x1b[F", "⇲"},
		{"home alt", "\x1b[1~", "⇱"},
		{"end alt", "\x1b[4~", "⇲"},
		{"page up", "\x1b[5~", "⇞"},
		{"page down", "\x1b[6~", "⇟"},
		{"insert", "\x1b[2~", "⎀"},
		{"delete", "\x1b[3~", "⌦"},
		{"f1", "\x1bOP", "[F1]"},
		{"f5", "\x1b[15~", "[F5]"},
		{"f12", "\x1b[24~", "[F12]"},
		{"lone escape", "\x1b", "␛"},
		{"plain text unaffected", "hello world", "hello world"},
		{"escape embedded in text", "foo\x1b[Abar", "foo↑bar"},
		{"longer sequence not partially matched", "\x1b[15~end", "[F5]end"},
		{"multiple sequences", "\x1b[A\x1b[B", "↑↓"},
		{"empty input", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(convertEscapeSequences([]byte(tc.in)))
			if got != tc.want {
				t.Errorf("convertEscapeSequences(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestMatchesSensitivePrompt(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"password lowercase", "password:", true},
		{"password uppercase", "PASSWORD:", true},
		{"password mixed case", "Password:", true},
		{"passphrase", "Enter passphrase: ", true},
		{"secret", "Client secret: ", true},
		{"no match", "login: admin", false},
		{"empty", "", false},
		{"partial word without colon", "password", false},
		{"substring match", "myPasswordFieldSecret: value", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := matchesSensitivePrompt([]byte(tc.in))
			if got != tc.want {
				t.Errorf("matchesSensitivePrompt(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func newTestLogger(t *testing.T, prefix string, passwordMode *atomic.Bool) (*lineLogger, *os.File, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.log")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create log file: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return &lineLogger{prefix: prefix, out: f, passwordMode: passwordMode}, f, path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	return string(data)
}

func TestLineLoggerWriteBasicLine(t *testing.T) {
	l, _, path := newTestLogger(t, "RX", &atomic.Bool{})
	l.Write([]byte("hello\n"))

	content := readFile(t, path)
	if !strings.Contains(content, "[RX] hello") {
		t.Errorf("expected log to contain '[RX] hello', got: %q", content)
	}
}

func TestLineLoggerWriteCRLFDedup(t *testing.T) {
	l, _, path := newTestLogger(t, "RX", &atomic.Bool{})
	l.Write([]byte("line1\r\nline2\n\r"))

	content := readFile(t, path)
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 logged lines from CRLF-paired input, got %d: %q", len(lines), content)
	}
	if !strings.Contains(lines[0], "line1") || !strings.Contains(lines[1], "line2") {
		t.Errorf("unexpected line content: %q", content)
	}
}

func TestLineLoggerWriteSplitAcrossCalls(t *testing.T) {
	l, _, path := newTestLogger(t, "TX", &atomic.Bool{})
	l.Write([]byte("par"))
	l.Write([]byte("tial\n"))

	content := readFile(t, path)
	if !strings.Contains(content, "[TX] partial") {
		t.Errorf("expected buffered write across calls to join into one line, got: %q", content)
	}
}

func TestLineLoggerWriteNoNewlineBuffersUntilFlush(t *testing.T) {
	l, _, path := newTestLogger(t, "TX", nil)
	l.Write([]byte("no newline yet"))

	content := readFile(t, path)
	if content != "" {
		t.Errorf("expected nothing logged before newline/flush, got: %q", content)
	}

	l.Flush()
	content = readFile(t, path)
	if !strings.Contains(content, "[TX] no newline yet") {
		t.Errorf("expected flush to write buffered partial line, got: %q", content)
	}
}

func TestLineLoggerFlushEmptyBufferWritesNothing(t *testing.T) {
	l, _, path := newTestLogger(t, "TX", nil)
	l.Flush()

	content := readFile(t, path)
	if content != "" {
		t.Errorf("expected no output from flushing an empty buffer, got: %q", content)
	}
}

func TestLineLoggerPasswordModeSuppressesLogging(t *testing.T) {
	pm := &atomic.Bool{}
	pm.Store(true)
	l, _, path := newTestLogger(t, "RX", pm)

	l.Write([]byte("supersecretpassword\n"))

	content := readFile(t, path)
	if content != "" {
		t.Errorf("expected logging suppressed while passwordMode is true, got: %q", content)
	}
}

func TestLineLoggerPasswordModeAllowsLoggingWhenFalse(t *testing.T) {
	pm := &atomic.Bool{}
	pm.Store(false)
	l, _, path := newTestLogger(t, "RX", pm)

	l.Write([]byte("normal line\n"))

	content := readFile(t, path)
	if !strings.Contains(content, "[RX] normal line") {
		t.Errorf("expected logging when passwordMode is false, got: %q", content)
	}
}

func TestLineLoggerFlushSuppressedDuringPasswordMode(t *testing.T) {
	pm := &atomic.Bool{}
	pm.Store(true)
	l, _, path := newTestLogger(t, "TX", pm)

	l.Write([]byte("partialsecret"))
	l.Flush()

	content := readFile(t, path)
	if content != "" {
		t.Errorf("expected flush suppressed while passwordMode is true, got: %q", content)
	}
}

func TestLineLoggerEscapeSequenceConversionInLog(t *testing.T) {
	l, _, path := newTestLogger(t, "RX", &atomic.Bool{})
	l.Write([]byte("go\x1b[Aup\n"))

	content := readFile(t, path)
	if !strings.Contains(content, "go↑up") {
		t.Errorf("expected escape sequence converted in logged output, got: %q", content)
	}
}
