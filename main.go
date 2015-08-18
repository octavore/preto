package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
)

type itemType int

const (
	itemError itemType = iota
	itemPackage
	itemMessageType
	itemIdentifier
	itemCommentStart
	itemLeftMeta
	itemRightMeta
	itemEqual
	itemNumber
	itemText
	itemFieldType
	itemFieldName
	itemFieldNum
	itemNewline
	itemWhitespace
)

func (i itemType) String() string {
	switch i {
	case itemError:
		return "ERROR"
	case itemPackage:
		return "PACKAGE"
	case itemMessageType:
		return "MESSAGETYPE"
	case itemIdentifier:
		return "IDENT"
	case itemCommentStart:
		return "COMMENT"
	case itemFieldType:
		return "FIELDTYPE"
	case itemFieldName:
		return "FIELDNAME"
	case itemFieldNum:
		return "FIELDNUM"
	case itemNewline:
		return "NL"
	case itemWhitespace:
		return "WS"
	default:
		return "LOL"
	}
}
func main() {
	fn := os.Args[1]
	fmt.Println(fn)
	f, err := os.Open(fn)
	if err != nil {
		panic(err)
	}

	l := lexer{buf: bufio.NewReader(f), c: make(chan item)}
	go func() {
		for s := range l.c {
			fmt.Printf("%s: %q\n", s.t, s.s)
		}
	}()
	l.lex()
}

type lexer struct {
	buf *bufio.Reader
	c   chan item
}

type item struct {
	t itemType
	s string
}

func (l *lexer) emit(t itemType, s string) {
	l.c <- item{t, s}
}

func (l *lexer) read() rune {
	ch, _, err := l.buf.ReadRune()
	if err == io.EOF {
		return rune(0)
	}
	if err != nil {
		panic(err)
	}
	return ch
}

func (l *lexer) unread() {
	_ = l.buf.UnreadRune()
}

func (l *lexer) lex() {
	state := scanText
	for state != nil {
		state = state(l)
	}
}

type reader interface {
	read() rune
	unread()
}

func readNum(l reader) string {
	b := &bytes.Buffer{}
	for {
		ch := l.read()
		if !isNumber(ch) {
			l.unread()
			break
		}
		_, err := b.WriteRune(ch)
		if err != nil {
			panic(err)
		}
	}
	// consume whitespaces until we have no more
	_ = readWhitespace(l)
	return b.String()
}
func readAlphanum(l reader) string {
	b := &bytes.Buffer{}
	for {
		ch := l.read()
		if !isLetter(ch) && !isNumber(ch) {
			l.unread()
			break
		}
		_, err := b.WriteRune(ch)
		if err != nil {
			panic(err)
		}
	}
	// consume whitespaces until we have no more
	_ = readWhitespace(l)
	return b.String()
}

func readWhitespace(l reader) string {
	b := &bytes.Buffer{}
	for {
		ch := l.read()
		if ch != ' ' && ch != '\t' {
			l.unread()
			break
		}
		_, err := b.WriteRune(ch)
		if err != nil {
			panic(err)
		}
	}
	return b.String()
}

type scanFn func(*lexer) scanFn

// scan reads in an unindented line
// package, message, comment
func scanText(l *lexer) scanFn {
	ch := l.read()
	switch {
	case ch == '\n':
		l.emit(itemNewline, "")
		return scanText
	case ch == ' ' || ch == '\t':
		l.unread()
		return scanIndent
	case ch == rune(0):
		return nil // eof
	case ch == '#':
		l.unread()
		return scanComment
	case ch == 'p':
		l.unread()
		return scanPackage
	case ch == 'm':
		l.unread()
		return scanMessage
	default:
		fmt.Println(ch)
		return nil // shrug
	}
}

func scanComment(l *lexer) scanFn {
	b, isPrefix, err := l.buf.ReadLine()
	if isPrefix {
		panic("not handled: read line is prefix")
	}
	if err != nil {
		panic(err)
	}
	l.emit(itemCommentStart, string(b))
	return scanText
}

func scanPackage(l *lexer) scanFn {
	if readAlphanum(l) != "package" {
		panic("unexpected keyword")
	}
	_ = readWhitespace(l)
	pkg := readAlphanum(l)
	l.emit(itemPackage, pkg)
	return scanEnd
}

func scanMessage(l *lexer) scanFn {
	if readAlphanum(l) != "msg" {
		panic("unexpected keyword")
	}
	_ = readWhitespace(l)
	msgName := readAlphanum(l)
	l.emit(itemMessageType, msgName)
	return scanEnd
}

// scanField scans an indented line, which is either a comment or a field
// todo: nested message, oneof, option, extensions
// todo: enum
func scanIndent(l *lexer) scanFn {
	ws := readWhitespace(l)
	if len(ws) == 0 {
		panic("unexpected 0 whitespace")
	}
	l.emit(itemWhitespace, ws)

	ch := l.read()
	l.unread()
	if ch == '#' {
		return scanEnd
	}
	return scanField
}

// scanType scans a type
// type can be one of the builtin types, a message type,
// or a repeated type
func scanField(l *lexer) scanFn {
	b := &bytes.Buffer{}
	isMap := false
	// check for array syntax
	if l.read() == '[' {
		if l.read() != ']' {
			panic("expected []")
		}
		b.WriteString("[]")
	} else {
		l.unread()
	}

	for {
		ch := l.read()
		if ch == '<' {
			isMap = true
		} else if isWhitespace(ch) {
			if !isMap {
				_ = l.buf.UnreadRune()
				break
			}
		} else if ch == ',' || ch == '>' {
			if !isMap {
				panic("not a map")
			}
		} else if !isLetter(ch) && !isNumber(ch) {
			panic("must be alphanumeric, got " + string(ch))
		}

		_, err := b.WriteRune(ch)
		if err != nil {
			panic(err)
		}
		if ch == '>' {
			// consume all whitespaces? if so no unread above
			break
		}
	}
	// todo: check b.string for enum
	l.emit(itemFieldType, b.String())
	_ = readWhitespace(l)
	return scanFieldName
}

func scanFieldName(l *lexer) scanFn {
	fieldName := readAlphanum(l)
	l.emit(itemFieldName, fieldName)
	_ = readWhitespace(l)
	ch := l.read()
	if ch != '=' {
		panic("expecting '='")
	}
	_ = readWhitespace(l)
	fieldNum := readNum(l)
	l.emit(itemFieldNum, fieldNum)
	return scanEnd
}

func scanEnd(l *lexer) scanFn {
	_ = readWhitespace(l)
	ch := l.read()
	if ch == '#' {
		l.unread()
		return scanComment
	}
	if ch == '\n' {
		l.emit(itemNewline, "")
		return scanText
	}
	panic("unexpected line end")
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\n'
}

func isNumber(ch rune) bool {
	return ch >= '0' && ch <= '9'
}
