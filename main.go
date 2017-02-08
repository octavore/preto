package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

type itemType int

const (
	itemUnknown itemType = iota
	itemError
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
	itemFieldOption
	itemNewline
	itemWhitespace
	itemOption
	itemOptionName
	itemEnum
	itemOneof
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
	case itemFieldOption:
		return "FIELDOPTION"
	case itemFieldNum:
		return "FIELDNUM"
	case itemNewline:
		return "NL"
	case itemWhitespace:
		return "WS"
	case itemOption:
		return "OPTIONTYPE"
	case itemOptionName:
		return "OPTIONVAL"
	default:
		return "LOL"
	}
}
func main() {
	fn := os.Args[1]
	f, err := os.Open(fn)
	if err != nil {
		panic(err)
	}

	l := lexer{buf: bufio.NewReader(f), c: make(chan item)}
	go l.lex()
	p := parser{w: os.Stdout, c: l.c}
	p.parse()
	// for s := range l.c {
	// 	fmt.Printf("%s: %q\n", s.t, s.s)
	// }
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
	close(l.c)
}

type reader interface {
	read() rune
	unread()
}

func readFunc(l reader, ok func(rune) bool) string {
	b := &bytes.Buffer{}
	for {
		ch := l.read()
		if !ok(ch) {
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

func readNum(l reader) string {
	return readFunc(l, isNumber)
}

func readAlphanum(l reader) string {
	return readFunc(l, func(ch rune) bool {
		return isLetter(ch) || isNumber(ch) || ch == '_' || ch == '.'
	})
}

func readFieldType(l reader) string {
	return readFunc(l, func(ch rune) bool {
		return isLetter(ch) || isNumber(ch) || ch == '_' || ch == '[' || ch == ']'
	})
}
func readOption(l reader) string {
	return readFunc(l, func(ch rune) bool {
		return isLetter(ch) || isNumber(ch) || ch == '_' || ch == '(' || ch == ')'
	})
}

func readStr(l reader) string {
	b := &bytes.Buffer{}
	ch := l.read()
	if ch != '"' {
		panic("string missing opening quote")
	}
	b.WriteRune('"')

	b.WriteString(readFunc(l, func(ch rune) bool {
		return ch != '"' && ch != '\n'
	}))

	ch = l.read()
	if ch != '"' {
		panic("string missing end quote")
	}
	b.WriteRune('"')
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
	case ch == ' ' || ch == '\t' || isLetter(ch):
		l.unread()
		return scanIndent
	case ch == rune(0):
		return nil // eof
	case ch == '#':
		l.unread()
		return scanComment
	default:
		return nil // wut
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
	l.emit(itemNewline, "")
	return scanText
}

// scanField scans an indented line, which is either a comment or a field
// todo: nested message, oneof, option, extensions
// todo: enum
func scanIndent(l *lexer) scanFn {
	ws := readWhitespace(l)
	if len(ws) > 0 {
		l.emit(itemWhitespace, ws)
	}
	// check for comment
	peek := l.read()
	l.unread()
	if peek == '#' {
		return scanEnd // todo: scanComment?
	}

	identType := itemUnknown
	x := readAlphanum(l)
	switch x {
	case "option":
		return scanFileOption
	case "msg":
		identType = itemMessageType
	case "package":
		identType = itemPackage
	case "enum":
		identType = itemEnum
	case "oneof":
		identType = itemOneof
	default:
		l.emit(itemIdentifier, x)
		_ = readWhitespace(l)
		return scanField
	}
	if identType != itemUnknown {
		x := readAlphanum(l)
		l.emit(identType, x)
		return scanEnd
	}
	panic("unreachable")
}

func scanFileOption(l *lexer) scanFn {
	o := readOption(l)
	l.emit(itemOption, o)

	_ = readWhitespace(l)

	s := readStr(l)
	l.emit(itemOptionName, s)
	return scanEnd
}

func scanField(l *lexer) scanFn {
	ch := l.read()
	l.unread()
	if isNumber(ch) {
		return scanFieldNum
	}
	return scanFieldType
}

func scanFieldType(l *lexer) scanFn {
	l.emit(itemFieldType, readFieldType(l))
	return scanFieldNum
}

func scanFieldNum(l *lexer) scanFn {
	l.emit(itemFieldNum, readNum(l))
	return scanFieldEnd
}

func scanFieldEnd(l *lexer) scanFn {
	_ = readWhitespace(l)
	ch := l.read()
	defer l.unread()
	if ch == '[' {
		return scanFieldOptions
	}
	return scanEnd
}

func scanFieldOptions(l *lexer) scanFn {
	ch := l.read()
	if ch != '[' {
		panic("expecting opening [ for option but got")
	}
	s := readFunc(l, func(ch rune) bool {
		return ch != ']' && ch != '\n'
	})
	l.emit(itemFieldOption, s)
	ch = l.read()
	if ch != ']' {
		panic("expecting opening ] for option")
	}
	return scanEnd
}

// scan until end, comment or newlines
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
	panic("unexpected line end " + string(ch))
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\n'
}

func isNumber(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

// PARSER

type parser struct {
	c    <-chan item
	head *item
	w    io.Writer

	line   int
	indent int
}

// return the next item. what to do when channel closes?
func (p *parser) next() item {
	o := item{}
	if p.head != nil {
		o = *p.head
		p.head = nil
	} else {
		o = <-p.c
	}
	// fmt.Println(">> ", o.t.String(), o.s)
	return o
}

// peek at the next item
func (p *parser) peek() item {
	if p.head != nil {
		return *p.head
	}
	i := p.next()
	p.head = &i
	return i
}

func (p *parser) write(lvl int, s string) {
	l := strings.Repeat(indentSpace, lvl)
	p.w.Write([]byte(l + s))
}

const indentSpace = "  "

func (p *parser) writef(lvl int, f string, args ...interface{}) {
	p.write(lvl, fmt.Sprintf(f, args...))
}

// toplevel parse
func (p *parser) parse() {
	for {
		i := p.peek()
		switch i.t {
		case itemUnknown:
			return
		case itemNewline:
			p.write(0, "\n")
			p.line++
			p.next()
		case itemWhitespace:
			p.parseNewline()
		case itemPackage:
			p.writef(0, "package %s;", i.s)
			p.next()
		case itemOption:
			j := <-p.c
			if j.t != itemOptionName {
				panic("parser: expected option value")
			}
			p.writef(0, "option %s = %s;", i.s, j.s)
			p.next()
		case itemEnum:
			// parseEnum()
		case itemCommentStart:
			// printComment(i)
		case itemMessageType:
			p.parseMessage(0)
		}
	}
	p.write(0, "\n")
}

func (p *parser) consumeNewlines() {
	for p.peek().t == itemNewline {
		p.next()
		// p.line++ // ??
		// p.indent = 0
	}
}

func (p *parser) parseNewline() {
	nl := p.next()
	for nl.t == itemWhitespace {
		nl = p.next()
	}
	if nl.t != itemNewline {
		panic("parser: expected newline, got " + nl.t.String())
	}
	p.write(0, "\n")
	p.line++
	p.indent = 0
}

func (p *parser) parseMessage(lvl int) {
	i := p.next()
	if i.t != itemMessageType {
		panic("expected message type")
	}
	p.writef(lvl, "message %s {", i.s)
	p.parseNewline()
	messageLevel := 0
	for {
		j := p.peek()
		if j.t == itemNewline {
			p.consumeNewlines()
			continue
		}
		if j.t != itemWhitespace {
			break
		}
		// basically in here we wanted something
		// indented, either a field or enum or oneof or message
		if messageLevel == 0 {
			messageLevel = len(j.s)
		}
		if len(j.s) < messageLevel {
			break
		}
		p.next()
		p.parseMessageInner(messageLevel)
	}
	p.write(lvl, "}\n")
}

func toProtoType(t string) string {
	switch t {
	case "str":
		return "string"
	}
	return t
}

func convertType(s string) string {
	if strings.HasPrefix(s, "map[") {
		i := strings.Index(s, "]")
		s = fmt.Sprintf("map<%s, %s>",
			toProtoType(s[4:i]),
			toProtoType(s[i+1:]),
		)
		return s
	}

	o := "optional"
	if strings.HasPrefix(s, "[]") {
		o = "repeated"
		s = toProtoType(s[2:])
	} else {
		s = toProtoType(s)
	}
	return o + " " + s
}

func (p *parser) parseMessageInner(lvl int) {
	i := p.peek()
	switch i.t {
	case itemCommentStart:
		p.writef(lvl, "// %s", strings.TrimLeft(i.s, "# "))
		p.next()
		p.parseNewline()
		return
	case itemIdentifier: // IDENT FIELDTYPE FIELDNUM
		p.parseField(lvl)
	case itemEnum:
		p.parseEnum(lvl)
	case itemMessageType:
		p.parseMessage(lvl)
	case itemOneof:
		p.parseOneof(lvl)
	case itemNewline:
		break
	default:
		panic("parser: unknown message contents" + i.t.String())
	}
}

func (p *parser) parseField(lvl int) {
	ident := p.next() // consume the peeked token
	if ident.t != itemIdentifier {
		panic("expected identifier")
	}
	fieldType := p.next()
	if fieldType.t != itemFieldType {
		panic("parser: expected field type but got " + fieldType.t.String())
	}
	fieldNum := p.next()
	if fieldNum.t != itemFieldNum {
		panic("parser expected field num")
	}
	p.writef(lvl, "%s %s = %s", convertType(fieldType.s), ident.s, fieldNum.s)

	// parse remainder of line
	rem := p.next()
	if rem.t == itemFieldOption {
		p.writef(0, " [%s]", rem.s)
		rem = p.next()
	}

	switch rem.t {
	case itemCommentStart:
		p.writef(0, "; // %s", strings.TrimLeft(rem.s, "# "))
		p.parseNewline()
		return
	case itemNewline:
		p.write(0, ";\n")
		p.line++
		return
	default:
		panic("parser: unknown field comment")
	}
}

func (p *parser) parseEnum(lvl int) {
	i := p.next()
	if i.t != itemEnum {
		panic("expected enum type")
	}
	p.writef(lvl, "enum %s {", i.s)
	p.parseNewline()

	// expect WS IDENT FIELDNUM (COMMENT) NEWLINE
	// expect WS COMMENT NEWLINE
	// expect WS NEWLINE
	messageLevel := 0
	for {
		j := p.peek()
		if j.t == itemNewline {
			p.next()
			continue
		}
		if j.t != itemWhitespace {
			break
		}
		if messageLevel == 0 {
			messageLevel = len(j.s)
		}
		if len(j.s) < messageLevel {
			// bug: actually okay if the next thing is a newline?
			break
		}
		p.next() // consume ws
		j = p.next()
		if j.t == itemIdentifier {
			k := p.next()
			if k.t != itemFieldNum {
				panic("expected field num")
			}
			p.writef(messageLevel, "%s = %s;", j.s, k.s)
		} else if j.t == itemCommentStart {
			p.writef(messageLevel, "// %s", j.s[2:])
		}
		j = p.peek()
		if j.t == itemCommentStart {
			p.next()
			p.writef(0, " // %s", j.s)
		}
		p.parseNewline()
	}
	p.write(lvl, "}\n")
}

func (p *parser) parseOneof(lvl int) {
	i := p.next()
	if i.t != itemOneof {
		panic("expected oneof type")
	}
	p.writef(lvl, "oneof %s {", i.s)
	p.parseNewline()

	messageLevel := 0
	for {
		j := p.peek()
		if j.t == itemNewline {
			p.next()
			continue
		}
		if j.t != itemWhitespace {
			break
		}
		if messageLevel == 0 {
			messageLevel = len(j.s)
		}
		if len(j.s) < messageLevel {
			// bug: actually okay if the next thing is a newline?
			break
		}
		p.next() // consume ws
		p.parseField(messageLevel)
	}
	p.write(lvl, "}\n")
}
