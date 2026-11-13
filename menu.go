package main

import (
	"fmt"
	"io"
	"os"
	"syscall"
	"unsafe"
)

// Values 1-9 are the digit keys.
type key int

const (
	keyNone key = 0
	keyUp   key = iota + 100
	keyDown
	keyEnter
	keyCancel
)

// nextKey decodes one key press; read reports false when no byte arrives in time.
func nextKey(read func() (byte, bool)) key {
	b, ok := read()
	if !ok {
		return keyNone
	}
	switch {
	case b == '\r' || b == '\n':
		return keyEnter
	case b == 3 || b == 4 || b == 'q':
		return keyCancel
	case b == 'k':
		return keyUp
	case b == 'j':
		return keyDown
	case b >= '1' && b <= '9':
		return key(b - '0')
	case b != 0x1b:
		return keyNone
	}

	// Esc on its own, or the start of an escape sequence such as an arrow key.
	if b, ok = read(); !ok {
		return keyCancel
	}
	if b != '[' && b != 'O' {
		return keyNone
	}
	for {
		if b, ok = read(); !ok {
			return keyNone
		}
		if b >= 0x40 && b <= 0x7e {
			break
		}
	}
	switch b {
	case 'A':
		return keyUp
	case 'B':
		return keyDown
	}
	return keyNone
}

// selectMenu draws items with a movable cursor and returns the index chosen.
func selectMenu(next func() key, out io.Writer, items []string, at int, color bool) (int, error) {
	style := func(code, s string) string {
		if !color {
			return s
		}
		return "\x1b[" + code + "m" + s + "\x1b[0m"
	}
	fmt.Fprint(out, "\x1b[?25l")
	defer fmt.Fprint(out, "\r\x1b[2K\x1b[?25h")

	for {
		for i, item := range items {
			if i == at {
				item = style("1;36", "> "+item)
			} else {
				item = "  " + item
			}
			fmt.Fprintf(out, "\x1b[2K%s\n", item)
		}
		fmt.Fprint(out, "\x1b[2K"+style("2", "↑/↓ move · enter select · esc cancel"))

		k := keyNone
		for k == keyNone {
			k = next()
		}
		done := false
		switch {
		case k == keyUp:
			at = (at + len(items) - 1) % len(items)
		case k == keyDown:
			at = (at + 1) % len(items)
		case k == keyEnter:
			return at, nil
		case k == keyCancel:
			return 0, errCancelled
		case int(k) <= len(items):
			at, done = int(k)-1, true
		}
		fmt.Fprintf(out, "\r\x1b[%dA", len(items))
		if done {
			// Show the row a digit key jumped to before leaving.
			next = func() key { return keyEnter }
		}
	}
}

// rawMode delivers key presses unbuffered and unechoed, with reads timing out after 100ms.
func rawMode(fd uintptr) (restore func(), err error) {
	var saved syscall.Termios
	set := func(req uintptr, t *syscall.Termios) error {
		if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(unsafe.Pointer(t))); errno != 0 {
			return errno
		}
		return nil
	}
	if err := set(ioctlGetTermios, &saved); err != nil {
		return nil, err
	}
	raw := saved
	raw.Lflag &^= syscall.ICANON | syscall.ECHO | syscall.ISIG
	raw.Cc[syscall.VMIN] = 0
	raw.Cc[syscall.VTIME] = 1
	if err := set(ioctlSetTermios, &raw); err != nil {
		return nil, err
	}
	return func() { set(ioctlSetTermios, &saved) }, nil
}

func terminalMenu(items []string, start int) (int, error) {
	restore, err := rawMode(os.Stdin.Fd())
	if err != nil {
		return 0, err
	}
	defer restore()

	var readErr error
	read := func() (byte, bool) {
		var b [1]byte
		for {
			n, err := syscall.Read(int(os.Stdin.Fd()), b[:])
			if err == syscall.EINTR {
				continue
			}
			readErr = err
			return b[0], n == 1
		}
	}
	next := func() key {
		k := nextKey(read)
		if readErr != nil {
			return keyCancel
		}
		return k
	}
	return selectMenu(next, os.Stderr, items, start, os.Getenv("NO_COLOR") == "")
}
