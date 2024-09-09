package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

type EditorConfig struct {
	screenRows  int
	screenCols  int
	origTermios *unix.Termios
}

var e EditorConfig

func die(fn string, err error) {
	os.Stdout.WriteString("\x1b[2J")
	os.Stdout.WriteString("\x1b[H")

	fmt.Fprintf(os.Stderr, "%s: %v", fn, err)
	os.Exit(1)
}

func enableRawMode() {
	raw, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TCGETS)
	if err != nil {
		die("IoctlGetTermios", err)
	}

	e.origTermios = raw

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
	if e.origTermios != nil {
		err := unix.IoctlSetTermios(int(os.Stderr.Fd()), unix.TCSETS, e.origTermios)
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

func getWindowSize() (int, int, error) {
	size, err := unix.IoctlGetWinsize(int(os.Stdin.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0, err
	}

	return int(size.Col), int(size.Row), nil
}

func editorProcessKeypress() {
	ch := editorReadKey()

	switch ch {
	case ctrlKey('q'):
		os.Stdout.WriteString("\x1b[2J")
		os.Stdout.WriteString("\x1b[H")
		os.Exit(0)
		break
	}
}

func editorDrawRows() {
	for y := 0; y < e.screenRows; y++ {
		os.Stdout.WriteString("~\r\n")
	}
}

func editorRefreshScreen() {
	os.Stdout.WriteString("\x1b[2J")
	os.Stdout.WriteString("\x1b[H")

	editorDrawRows()

	os.Stdout.WriteString("\x1b[H")
}

func initEditor() {
	c, r, err := getWindowSize()
	if err != nil {
		die("getWindowSize", err)
	}

	e.screenCols = c
	e.screenRows = r
}

func main() {
	enableRawMode()
	defer disableRawMode()

	initEditor()

	for {
		editorRefreshScreen()
		editorProcessKeypress()
	}
}
