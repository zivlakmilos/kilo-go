package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

var origTermios *unix.Termios

func die(fn string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v", fn, err)
	os.Exit(1)
}

func enableRawMode() {
	raw, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TCGETS)
	if err != nil {
		die("IoctlGetTermios", err)
	}

	origTermios = raw

	raw.Iflag &= ^uint32(unix.IXON | unix.ICRNL | unix.BRKINT | unix.INPCK | unix.ISTRIP)
	raw.Oflag &= ^uint32(unix.OPOST)
	raw.Cflag |= (unix.CS8)
	raw.Lflag &= ^uint32(unix.ECHO | unix.ICANON | unix.ISIG | unix.IEXTEN)

	raw.Cc[unix.VMIN] = 0
	raw.Cc[unix.VTIME] = 1

	err = unix.IoctlSetTermios(int(os.Stderr.Fd()), unix.TCSETS, raw)
	if err != nil {
		die("IoctlSetTermios", err)
	}
}

func disableRawMode() {
	if origTermios != nil {
		err := unix.IoctlSetTermios(int(os.Stderr.Fd()), unix.TCSETS, origTermios)
		if err != nil {
			die("IoctlSetTermios", err)
		}
	}
}

func ctrlKey(k byte) byte {
	return (k & 0x1f)
}

func editorReadKey() byte {
	b := make([]byte, 1)

	for {
		n, err := os.Stdin.Read(b)
		if err != nil {
			if err.Error() != "EOF" {
				die("editorReadKey", err)
			}
		}

		if n > 0 {
			break
		}
	}

	return b[0]
}

func editorProcessKeypress() {
	ch := editorReadKey()

	switch ch {
	case ctrlKey('q'):
		os.Exit(0)
		break
	}
}

func main() {
	enableRawMode()
	defer disableRawMode()

	for {
		editorProcessKeypress()
	}
}
