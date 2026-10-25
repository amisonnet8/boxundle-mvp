package syntax

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/amisonnet8/boxundle-mvp/internal/diag"
)

// Kind はトークンの種類。
type Kind int

const (
	EOF     Kind = iota
	Keyword      // module box in … true false
	Name         // NAME または QNAME（Text は "std.text.split" のように . でつないだもの）
	NodeID       // #4.1（Text）。後ろのフィールドは Fields
	Dollar       // $order（Text）。後ろのフィールドは Fields
	Int          // -?[0-9]+
	Dec          // -?[0-9]+\.[0-9]+
	String       // "…"（Value は解読した中身）
	Label        // @"…"（Value は解読した中身）
	LBrace
	RBrace
	LBrack
	RBrack
	LParen
	RParen
	Comma
	Colon
	Equals
	Pipe
	Arrow
)

var kindNames = [...]string{
	EOF: "ファイルの終わり", Keyword: "キーワード", Name: "名前", NodeID: "ノードID",
	Dollar: "ポート・束縛子", Int: "整数", Dec: "小数", String: "文字列", Label: "ラベル",
	LBrace: "{", RBrace: "}", LBrack: "[", RBrack: "]", LParen: "(", RParen: ")",
	Comma: ",", Colon: ":", Equals: "=", Pipe: "|", Arrow: "->",
}

// String は、利用者向けのメッセージに使う種類の名前。記号は記号そのもの。
func (k Kind) String() string { return kindNames[k] }

func (k Kind) isPunct() bool { return k >= LBrace }

var puncts = map[rune]Kind{
	'{': LBrace, '}': RBrace, '[': LBrack, ']': RBrack, '(': LParen, ')': RParen,
	',': Comma, ':': Colon, '=': Equals, '|': Pipe,
}

var keywords = map[string]bool{
	"module": true, "externs": true, "extern": true, "type": true, "box": true,
	"in": true, "out": true, "perm": true, "none": true, "test": true,
	"given": true, "expect": true, "stub": true, "body": true, "emit": true,
	"impl": true, "as": true, "true": true, "false": true,
}

// IsKeyword は、s がキーワードかを返す。
func IsKeyword(s string) bool { return keywords[s] }

// Token は1つの字句。
type Token struct {
	Kind   Kind
	Text   string   // 書かれたとおりの本文（String・Label は引用符を含む）
	Value  string   // String・Label の解読した中身
	Fields []string // NodeID・Dollar の後ろの .名前
	Span   diag.Span
	Start  int // バイトの位置 [Start, End)
	End    int
}

// Lex は src をトークンに分ける。最後のトークンは EOF。
// 誤りは最初の1つを BX001 の diag.Diagnostic として返す。
func Lex(file string, src []byte) ([]Token, error) {
	l := &lexer{file: file, src: src, line: 1, col: 1}
	if err := l.checkUTF8(); err != nil {
		return nil, err
	}
	for {
		tok, err := l.next()
		if err != nil {
			return nil, err
		}
		l.toks = append(l.toks, tok)
		if tok.Kind == EOF {
			return l.toks, nil
		}
	}
}

type lexer struct {
	file      string
	src       []byte
	off       int // 今のバイトの位置
	line, col int // 今の位置（1始まり、列はコードポイント単位）
	toks      []Token
}

type mark struct{ off, line, col int }

func (l *lexer) mark() mark { return mark{l.off, l.line, l.col} }

func (l *lexer) span(from mark) diag.Span {
	return diag.Span{File: l.file, Start: diag.Pos{Line: from.line, Col: from.col}, End: diag.Pos{Line: l.line, Col: l.col}}
}

// peek は今の位置から n 個先（0始まり）の文字を返す。無ければ -1。
func (l *lexer) peek(n int) rune {
	off := l.off
	for range n {
		if off >= len(l.src) {
			return -1
		}
		_, size := utf8.DecodeRune(l.src[off:])
		off += size
	}
	if off >= len(l.src) {
		return -1
	}
	r, _ := utf8.DecodeRune(l.src[off:])
	return r
}

func (l *lexer) advance() {
	r, size := utf8.DecodeRune(l.src[l.off:])
	l.off += size
	if r == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
}

// fail は from から今の位置の1文字先までを指す BX001 を作る。
func (l *lexer) fail(from mark, format string, args ...any) diag.Diagnostic {
	save := l.mark()
	if l.off == from.off && l.off < len(l.src) {
		l.advance()
	}
	d := diag.New(diag.BX001, fmt.Sprintf(format, args...))
	d.Span = l.span(from)
	l.off, l.line, l.col = save.off, save.line, save.col
	return d
}

// failHere は今の位置の1文字を指す BX001 を作る。
func (l *lexer) failHere(format string, args ...any) diag.Diagnostic {
	return l.fail(l.mark(), format, args...)
}

// checkUTF8 は、字句に分ける前に、src 全体が正しい UTF-8 かを確かめる。
func (l *lexer) checkUTF8() error {
	for l.off < len(l.src) {
		r, size := utf8.DecodeRune(l.src[l.off:])
		if r == utf8.RuneError && size == 1 {
			return l.failHere("不正な UTF-8 のバイト列です")
		}
		l.advance()
	}
	l.off, l.line, l.col = 0, 1, 1
	return nil
}

func (l *lexer) skipSpace() error {
	for {
		switch l.peek(0) {
		case ' ', '\t', '\n':
			l.advance()
		case '\r':
			if l.peek(1) != '\n' {
				return l.failHere("単独の CR は改行に使えません")
			}
			l.advance()
			l.advance()
		default:
			return nil
		}
	}
}

func (l *lexer) next() (Token, error) {
	if err := l.skipSpace(); err != nil {
		return Token{}, err
	}
	from := l.mark()
	r := l.peek(0)
	switch {
	case r == -1:
		return l.token(EOF, from), nil
	case puncts[r] != 0:
		l.advance()
		return l.token(puncts[r], from), nil
	case r == '-' && l.peek(1) == '>':
		l.advance()
		l.advance()
		return l.token(Arrow, from), nil
	case r == '-' && isDigit(l.peek(1)), isDigit(r):
		return l.number(from)
	case r == '-':
		return Token{}, l.failHere("- の後には数字か > が要ります")
	case r == '"':
		v, err := l.str()
		if err != nil {
			return Token{}, err
		}
		tok := l.token(String, from)
		tok.Value = v
		return tok, nil
	case r == '@':
		l.advance()
		if l.peek(0) != '"' {
			return Token{}, l.fail(from, "@ の後には、空白を挟まずに文字列が要ります")
		}
		v, err := l.str()
		if err != nil {
			return Token{}, err
		}
		tok := l.token(Label, from)
		tok.Value = v
		return tok, nil
	case r == '#':
		return l.nodeID(from)
	case r == '$':
		return l.dollar(from)
	case isIDStart(r):
		return l.name(from)
	default:
		return Token{}, l.unexpected(r)
	}
}

func (l *lexer) token(k Kind, from mark) Token {
	return Token{Kind: k, Text: string(l.src[from.off:l.off]), Span: l.span(from), Start: from.off, End: l.off}
}

func (l *lexer) unexpected(r rune) diag.Diagnostic {
	var d diag.Diagnostic
	switch {
	case r == '.':
		d = l.failHere("ここに . は書けません（. は修飾名・ノードID・フィールドの中だけ）")
	case unicode.IsSpace(r) || r == 0xFEFF:
		d = l.failHere("使えない空白の文字 %U です", r)
		d.Hint = "空白はスペース・タブ・改行（LF・CRLF）だけです"
	case unicode.IsGraphic(r):
		d = l.failHere("使えない文字 %q（%U）です", r, r)
	default:
		d = l.failHere("使えない文字 %U です", r)
	}
	return d
}

func isDigit(r rune) bool { return '0' <= r && r <= '9' }

func (l *lexer) digits() {
	for isDigit(l.peek(0)) {
		l.advance()
	}
}

// boundary は、数・ノードIDの直後に名前の文字が続いていないかを確かめる。
func (l *lexer) boundary(what string) error {
	if isIDContinue(l.peek(0)) {
		return l.failHere("%sの直後に、空白を挟まずに名前の文字は続けられません", what)
	}
	return nil
}

func (l *lexer) number(from mark) (Token, error) {
	if l.peek(0) == '-' {
		l.advance()
	}
	l.digits()
	kind := Int
	if l.peek(0) == '.' && isDigit(l.peek(1)) {
		l.advance()
		l.digits()
		kind = Dec
	}
	if err := l.boundary("数"); err != nil {
		return Token{}, err
	}
	return l.token(kind, from), nil
}

// str は "…" を読み、解読した中身を返す。今の位置は開きの " 。
func (l *lexer) str() (string, error) {
	open := l.mark()
	l.advance()
	var b strings.Builder
	for {
		r := l.peek(0)
		switch r {
		case -1:
			return "", l.fail(open, "文字列が閉じていません")
		case '"':
			l.advance()
			return b.String(), nil
		case '\n', '\r':
			d := l.failHere("文字列の中に改行は書けません")
			d.Hint = `改行は \n と書いてください`
			return "", d
		case '\\':
			esc := l.mark()
			l.advance()
			switch e := l.peek(0); e {
			case -1:
				return "", l.fail(open, "文字列が閉じていません")
			case '"', '\\':
				b.WriteRune(e)
				l.advance()
			case 'n':
				b.WriteByte('\n')
				l.advance()
			case 't':
				b.WriteByte('\t')
				l.advance()
			case 'u':
				l.advance()
				hex := make([]rune, 0, 4)
				var v rune
				for range 4 {
					h := l.peek(0)
					if !strings.ContainsRune("0123456789abcdefABCDEF", h) {
						break
					}
					hex = append(hex, h)
					v = v*16 + hexValue(h)
					l.advance()
				}
				if len(hex) != 4 {
					return "", l.fail(esc, `\u の後には16進数ちょうど4桁が要ります`)
				}
				if 0xD800 <= v && v <= 0xDFFF {
					return "", l.fail(esc, `\u%s はサロゲートで、文字ではありません`, string(hex))
				}
				b.WriteRune(v)
			default:
				l.advance()
				d := l.fail(esc, `未知のエスケープ \%c です`, e)
				d.Hint = `使えるのは \" \\ \n \t \uXXXX です`
				return "", d
			}
		default:
			b.WriteRune(r)
			l.advance()
		}
	}
}

// hexValue は16進数の1桁の値。h は 0-9a-fA-F のどれか。
func hexValue(h rune) rune {
	switch {
	case h <= '9':
		return h - '0'
	case h >= 'a':
		return h - 'a' + 10
	default:
		return h - 'A' + 10
	}
}

func (l *lexer) word() string {
	from := l.off
	l.advance()
	for isIDContinue(l.peek(0)) {
		l.advance()
	}
	return string(l.src[from:l.off])
}

// fields は、参照の後ろの .名前 の並びを読む。
func (l *lexer) fields(from mark) ([]string, error) {
	var fs []string
	for l.peek(0) == '.' {
		dot := l.mark()
		l.advance()
		if !isIDStart(l.peek(0)) {
			return nil, l.fail(dot, ". の後には名前が要ります（名前は数字で始まりません）")
		}
		w := l.word()
		if keywords[w] {
			return nil, l.fail(from, "キーワード %s はフィールドの名前に使えません", w)
		}
		fs = append(fs, w)
	}
	return fs, nil
}

func (l *lexer) nodeID(from mark) (Token, error) {
	l.advance() // #
	for {
		if !isDigit(l.peek(0)) {
			return Token{}, l.fail(from, "# の後には、ドットで区切った正の整数が要ります")
		}
		if l.peek(0) == '0' {
			return Token{}, l.fail(from, "ノードIDの数は正の整数で、先頭に0は書けません")
		}
		l.digits()
		if l.peek(0) == '.' && isDigit(l.peek(1)) {
			l.advance()
			continue
		}
		break
	}
	if err := l.boundary("ノードID"); err != nil {
		return Token{}, err
	}
	tok := l.token(NodeID, from)
	fs, err := l.fields(from)
	if err != nil {
		return Token{}, err
	}
	tok.Fields = fs
	tok.Span, tok.End = l.span(from), l.off
	return tok, nil
}

func (l *lexer) dollar(from mark) (Token, error) {
	l.advance() // $
	if !isIDStart(l.peek(0)) {
		return Token{}, l.fail(from, "$ の後には、空白を挟まずに名前が要ります")
	}
	w := l.word()
	if keywords[w] {
		return Token{}, l.fail(from, "キーワード %s はポート・束縛子の名前に使えません", w)
	}
	tok := l.token(Dollar, from)
	fs, err := l.fields(from)
	if err != nil {
		return Token{}, err
	}
	tok.Fields = fs
	tok.Span, tok.End = l.span(from), l.off
	return tok, nil
}

func (l *lexer) name(from mark) (Token, error) {
	parts := []string{l.word()}
	for l.peek(0) == '.' {
		dot := l.mark()
		l.advance()
		if !isIDStart(l.peek(0)) {
			return Token{}, l.fail(dot, ". の後には名前が要ります")
		}
		parts = append(parts, l.word())
	}
	if len(parts) == 1 && keywords[parts[0]] {
		return l.token(Keyword, from), nil
	}
	for _, p := range parts {
		if keywords[p] {
			return Token{}, l.fail(from, "キーワード %s は名前の一部に使えません", p)
		}
	}
	return l.token(Name, from), nil
}

// isIDStart・isIDContinue は、UAX #31 の ID_Start・ID_Continue を Go の unicode の表で求める。
// XID との差（NFKC で閉じるために除く数文字）は扱わない。
func isIDStart(r rune) bool {
	if r == '_' {
		return true
	}
	if unicode.In(r, unicode.Pattern_Syntax, unicode.Pattern_White_Space) {
		return false
	}
	return unicode.In(r, unicode.L, unicode.Nl, unicode.Other_ID_Start)
}

func isIDContinue(r rune) bool {
	if isIDStart(r) {
		return true
	}
	if unicode.In(r, unicode.Pattern_Syntax, unicode.Pattern_White_Space) {
		return false
	}
	return unicode.In(r, unicode.Mn, unicode.Mc, unicode.Nd, unicode.Pc, unicode.Other_ID_Continue)
}
