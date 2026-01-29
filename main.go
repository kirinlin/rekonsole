package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.bug.st/serial"
	"golang.org/x/term"
)

func main() {
	baud := flag.Int("baud", 115200, "baud rate")
	flag.IntVar(baud, "b", 115200, "baud rate (shorthand)")
	noLog := flag.Bool("no-log", false, "disable I/O logging")
	flag.BoolVar(noLog, "l", false, "disable I/O logging (shorthand)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: rekonsole <device> [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Establish a raw USB serial console connection.\n\n")
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  <device>    Serial device path (e.g. /dev/ttyUSB0)\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

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

	fmt.Fprintf(os.Stderr, "Connected to %s at %d baud. Press Ctrl+C to exit.\n", device, *baud)

	// Set up log file
	var logFile *os.File
	if !*noLog {
		filename := time.Now().Format("20060102-150405") + "-raw.txt"
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

	// Handle signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})

	// serial -> stdout (+ log)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := port.Read(buf)
			if n > 0 {
				os.Stdout.Write(buf[:n])
				if logFile != nil {
					logFile.Write(buf[:n])
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
		}
	}()

	// stdin -> serial (+ log)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				port.Write(buf[:n])
				if logFile != nil {
					logFile.Write(buf[:n])
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
	case <-done:
	}
}
