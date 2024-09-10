package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

const KILO_VERSION = "0.0.1"

const (
	ARROW_LEFT  int = 1000
	ARROW_RIGHT int = 1001
	ARROW_UP    int = 1002
	ARROW_DOWN  int = 1003
)

type EditorConfig struct {
	cx          int
	cy          int
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

func editorReadKey() int {
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

	if b[0] == '\x1b' {
		seq := make([]byte, 3)

		n, err := os.Stdin.Read(b)
		if err != nil || n < 1 {
			return '\x1b'
		}
		seq[0] = b[0]

		n, err = os.Stdin.Read(b)
		if err != nil || n < 1 {
			return '\x1b'
		}
		seq[1] = b[0]

		if seq[0] == '[' {
			switch seq[1] {
			case 'A':
				return ARROW_UP
			case 'B':
				return ARROW_DOWN
			case 'C':
				return ARROW_RIGHT
			case 'D':
				return ARROW_LEFT
			}
		}

		return '\x1b'
	}

	return int(b[0])
}

func getCursorPosition() (int, int, error) {
	_, err := os.Stdout.WriteString("\x1b[6n")
	if err != nil {
		return 0, 0, err
	}

	buff := make([]byte, 32)
	i := 0

	for {
		b := make([]byte, 1)
		n, _ := os.Stdin.Read(b)
		if n < 1 {
			break
		}

		buff[i] = b[0]
		if buff[i] == 'R' {
			break
		}

		i++
	}

	buff[i] = 0

	var rows int
	var cols int
	fmt.Sscanf(string(buff[2:]), "%d;%d", &rows, &cols)

	return rows, cols, nil
}

func getWindowSize() (int, int, error) {
	size, err := unix.IoctlGetWinsize(int(os.Stdin.Fd()), unix.TIOCGWINSZ)
	if err != nil || size.Col == 0 {
		_, err = os.Stdout.WriteString("\x1b[999C\x1b[999B")
		if err != nil {
			return 0, 0, err
		}

		return getCursorPosition()
	}

	return int(size.Col), int(size.Row), nil
}

func editorMoveCursor(key int) {
	switch key {
	case ARROW_LEFT:
		e.cx--
	case ARROW_RIGHT:
		e.cx++
	case ARROW_UP:
		e.cy--
	case ARROW_DOWN:
		e.cy++
	}
}

func editorProcessKeypress() {
	ch := editorReadKey()

	switch ch {
	case int(ctrlKey('q')):
		os.Stdout.WriteString("\x1b[2J")
		os.Stdout.WriteString("\x1b[H")
		os.Exit(0)

	case ARROW_UP,
		ARROW_DOWN,
		ARROW_LEFT,
		ARROW_RIGHT:
		editorMoveCursor(ch)
	}
}

func editorDrawRows(sw io.StringWriter) {
	for y := 0; y < e.screenRows; y++ {
		if y == e.screenRows/3 {
			welcome := fmt.Sprintf("Kilo editor -- version %s", KILO_VERSION)
			if len(welcome) > e.screenCols {
				welcome = welcome[:e.screenCols]
			}
			padding := (e.screenCols - len(welcome)) / 2
			if padding > 0 {
				sw.WriteString("~")
				padding--
			}
			for padding > 0 {
				sw.WriteString(" ")
				padding--
			}
			sw.WriteString(welcome)
		} else {
			sw.WriteString("~")
		}

		sw.WriteString("\x1b[K")
		if y < e.screenRows-1 {
			sw.WriteString("\r\n")
		}
	}
}

func editorRefreshScreen() {
	buff := bytes.NewBuffer([]byte{})

	buff.WriteString("\x1b[?25l")
	buff.WriteString("\x1b[H")

	editorDrawRows(buff)

	buff.WriteString(fmt.Sprintf("\x1b[%d;%dH", e.cy+1, e.cx+1))
	buff.WriteString("\x1b[?25h")

	os.Stdout.WriteString(buff.String())
}

func initEditor() {
	e.cx = 0
	e.cy = 0

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
