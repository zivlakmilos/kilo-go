package main

import (
	"fmt"
	"os"
	"unicode"

	"golang.org/x/sys/unix"
)

var origTermios *unix.Termios

func enableRawMode() {
	raw, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TCGETS)
	if err != nil {
		return
	}

	origTermios = raw

	raw.Iflag &= ^uint32(unix.IXON | unix.ICRNL)
	raw.Oflag &= ^uint32(unix.OPOST)
	raw.Lflag &= ^uint32(unix.ECHO | unix.ICANON | unix.ISIG | unix.IEXTEN)

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

		if unicode.IsControl(rune(c)) {
			fmt.Printf("%d\n", c)
		} else {
			fmt.Printf("%d ('%c')\n", c, c)
		}
	}
}
