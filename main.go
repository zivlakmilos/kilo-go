package main

import (
	"os"

	"golang.org/x/sys/unix"
)

var origTermios *unix.Termios

func enableRawMode() {
	raw, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TCGETS)
	if err != nil {
		return
	}

	origTermios = raw

	raw.Lflag &= ^uint32(unix.ECHO | unix.ICANON)

	err = unix.IoctlSetTermios(int(os.Stderr.Fd()), unix.TCSETS, raw)
	if err != nil {
		return
	}
}

func disableRawMode() {
	if origTermios != nil {
		_ = unix.IoctlSetTermios(int(os.Stderr.Fd()), unix.TCSETS, origTermios)
	}
}

func main() {
	enableRawMode()
	defer disableRawMode()

	b := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(b)
		if err != nil || n < 1 {
			return
		}
		c := b[0]

		if c == 'q' {
			break
		}
	}
}
