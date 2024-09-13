package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

const KILO_VERSION = "0.0.1"

const (
	ARROW_LEFT int = iota + 1000
	ARROW_RIGHT
	ARROW_UP
	ARROW_DOWN
	DEL_KEY
	HOME_KEY
	END_KEY
	PAGE_UP
	PAGE_DOWN
)

type EditorRow struct {
	size  int
	chars string
}

type EditorConfig struct {
	cx          int
	cy          int
	rowOff      int
	colOff      int
	screenRows  int
	screenCols  int
	origTermios *unix.Termios

	numOfRows int
	row       []EditorRow
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
			if seq[1] > '0' && seq[1] < '9' {
				n, err = os.Stdin.Read(b)
				if err != nil || n < 1 {
					return '\x1b'
				}
				seq[2] = b[0]

				if seq[2] == '~' {
					switch seq[1] {
					case '1':
						return HOME_KEY
					case '3':
						return DEL_KEY
					case '4':
						return END_KEY
					case '5':
						return PAGE_UP
					case '6':
						return PAGE_DOWN
					case '7':
						return HOME_KEY
					case '8':
						return END_KEY
					}
				}

				return '\x1b'
			}

			switch seq[1] {
			case 'A':
				return ARROW_UP
			case 'B':
				return ARROW_DOWN
			case 'C':
				return ARROW_RIGHT
			case 'D':
				return ARROW_LEFT
			case 'H':
				return HOME_KEY
			case 'F':
				return END_KEY
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

func editorAppendRow(s string) {
	size := len(s)
	row := EditorRow{
		size:  size,
		chars: s,
	}
	e.row = append(e.row, row)
	e.numOfRows++
}

func editorOpen(filename string) {
	f, err := os.Open(filename)
	if err != nil {
		die("editorOpen", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		editorAppendRow(line)
	}
}

func editorMoveCursor(key int) {
	var row *EditorRow
	if e.cy < e.numOfRows {
		row = &e.row[e.cy]
	}

	switch key {
	case ARROW_LEFT:
		if e.cx > 0 {
			e.cx--
		} else if e.cy > 0 {
			e.cy--
			e.cx = e.row[e.cy].size
		}
	case ARROW_RIGHT:
		if row != nil && e.cx < row.size {
			e.cx++
		} else if row != nil {
			e.cy++
			e.cx = 0
		}
	case ARROW_UP:
		if e.cy > 0 {
			e.cy--
		}
	case ARROW_DOWN:
		if e.cy < e.numOfRows {
			e.cy++
		}
	}

	row = nil
	if e.cy < e.numOfRows {
		row = &e.row[e.cy]
	}

	rowLen := 0
	if row != nil {
		rowLen = row.size
	}
	if e.cx > rowLen {
		e.cx = rowLen
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

	case HOME_KEY:
		e.cx = 0

	case END_KEY:
		e.cx = e.screenCols - 1

	case PAGE_UP, PAGE_DOWN:
		times := e.screenRows
		for times > 0 {
			if ch == PAGE_UP {
				editorMoveCursor(ARROW_UP)
			} else {
				editorMoveCursor(ARROW_DOWN)
			}
			times--
		}
	}
}

func editorScroll() {
	if e.cy < e.rowOff {
		e.rowOff = e.cy
	}
	if e.cy >= e.rowOff+e.screenRows {
		e.rowOff = e.cy - e.screenRows + 1
	}

	if e.cx < e.colOff {
		e.colOff = 0
	}
	if e.cx >= e.colOff+e.screenCols {
		e.colOff = e.cx - e.screenCols + 1
	}
}

func editorDrawRows(sw io.StringWriter) {
	for y := 0; y < e.screenRows; y++ {
		fileRow := y + e.rowOff
		if fileRow >= e.numOfRows {
			if e.numOfRows == 0 && y == e.screenRows/3 {
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
		} else {
			rowLen := e.row[fileRow].size
			if rowLen < 0 {
				rowLen = 0
			}
			if rowLen > e.screenCols {
				rowLen = e.screenCols
			}
			rowStart := e.colOff
			if rowStart < 0 {
				rowStart = 0
			}
			if rowStart > rowLen {
				rowStart = rowLen
			}
			sw.WriteString(e.row[fileRow].chars[rowStart:rowLen])
		}

		sw.WriteString("\x1b[K")
		if y < e.screenRows-1 {
			sw.WriteString("\r\n")
		}
	}
}

func editorRefreshScreen() {
	editorScroll()

	buff := bytes.NewBuffer([]byte{})

	buff.WriteString("\x1b[?25l")
	buff.WriteString("\x1b[H")

	editorDrawRows(buff)

	buff.WriteString(fmt.Sprintf("\x1b[%d;%dH",
		(e.cy - e.rowOff + 1),
		(e.cx - e.colOff + 1)))
	buff.WriteString("\x1b[?25h")

	os.Stdout.WriteString(buff.String())
}

func initEditor() {
	e.cx = 0
	e.cy = 0
	e.rowOff = 0
	e.colOff = 0
	e.numOfRows = 0

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

	if len(os.Args) > 1 {
		editorOpen(os.Args[1])
	}

	for {
		editorRefreshScreen()
		editorProcessKeypress()
	}
}
