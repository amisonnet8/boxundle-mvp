package syntax

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/amisonnet8/boxundle-mvp/internal/diag"
)

// show は、トークンを比べやすい1つの文字列にする。記号はそのまま、
// それ以外は「種類(本文)」。フィールドは本文の後ろに「 .名前」で足す。
func show(toks []Token) string {
	var out []string
	for _, t := range toks {
		if t.Kind == EOF {
			continue
		}
		if t.Kind.isPunct() {
			out = append(out, t.Text)
			continue
		}
		s := t.Text
		if t.Kind == String || t.Kind == Label {
			s = t.Value
		}
		for _, f := range t.Fields {
			s += " ." + f
		}
		out = append(out, t.Kind.String()+"("+s+")")
	}
	return strings.Join(out, " ")
}

func mustLex(t *testing.T, src string) []Token {
	t.Helper()
	toks, err := Lex("t.bx", []byte(src))
	if err != nil {
		t.Fatalf("Lex(%q): %v", src, err)
	}
	return toks
}

func TestLexKinds(t *testing.T) {
	src := `module shop box B7 @"注文" { in $order : Order out $r : {Str -> Int} perm none
body { #1 call std.text.split {s: "a,b", sep: ","} #2 val #1.parts emit $r #4.1.total.x } }`
	want := "キーワード(module) 名前(shop) キーワード(box) 名前(B7) ラベル(注文) { " +
		"キーワード(in) ポート・束縛子($order) : 名前(Order) キーワード(out) ポート・束縛子($r) : { 名前(Str) -> 名前(Int) } " +
		"キーワード(perm) キーワード(none) キーワード(body) { " +
		"ノードID(#1) 名前(call) 名前(std.text.split) { 名前(s) : 文字列(a,b) , 名前(sep) : 文字列(,) } " +
		"ノードID(#2) 名前(val) ノードID(#1 .parts) キーワード(emit) ポート・束縛子($r) ノードID(#4.1 .total .x) } }"
	if got := show(mustLex(t, src)); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestLexLiterals(t *testing.T) {
	// \u のエスケープは srwr の引数で本物の文字に化けるので、バックスラッシュをつないで作る（testing.md）。
	src := `0 12 -7 007 3.50 -1.25 true false -> "a\"b\\c\nd\te` + "\\" + `u3042" $e.product 名前 _x x1 Int`
	want := "整数(0) 整数(12) 整数(-7) 整数(007) 小数(3.50) 小数(-1.25) キーワード(true) キーワード(false) -> " +
		"文字列(a\"b\\c\nd\teあ) ポート・束縛子($e .product) 名前(名前) 名前(_x) 名前(x1) 名前(Int)"
	if got := show(mustLex(t, src)); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// 数字の上端（9）と、16進数の各範囲の境目・小文字の \u。
func TestLexDigitEdges(t *testing.T) {
	if got, want := show(mustLex(t, "9 -9 1.9 #9.19")), "整数(9) 整数(-9) 小数(1.9) ノードID(#9.19)"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	bs := "\\"
	tests := []struct{ src, want string }{
		{`"` + bs + `u0039` + bs + `u00aa` + bs + `u00AF"`, "9\xc2\xaa\xc2\xaf"}, // 9・ª・¯
		{`"` + bs + `u30420"`, "あ0"},                                             // 5桁目は普通の文字
	}
	for _, tt := range tests {
		if got := mustLex(t, tt.src)[0].Value; got != tt.want {
			t.Errorf("%s: value %q, want %q", tt.src, got, tt.want)
		}
	}
}

func TestLexKeywords(t *testing.T) {
	kw := "module externs extern type box in out perm none test given expect stub body emit impl as true false"
	for _, tok := range mustLex(t, kw) {
		if tok.Kind != Keyword && tok.Kind != EOF {
			t.Errorf("%q: kind %v, want キーワード", tok.Text, tok.Kind)
		}
	}
	// 演算名・組み込み型名はキーワードでない。
	for _, tok := range mustLex(t, "map route match call val Int Dec Bool Str") {
		if tok.Kind != Name && tok.Kind != EOF {
			t.Errorf("%q: kind %v, want 名前", tok.Text, tok.Kind)
		}
	}
}

func TestLexPunct(t *testing.T) {
	if got, want := show(mustLex(t, "{}[](),:=|->")), "{ } [ ] ( ) , : = | ->"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func pos(line, col int) diag.Pos { return diag.Pos{Line: line, Col: col} }

func TestLexSpans(t *testing.T) {
	// 列はコードポイント単位で1始まり。終わりは字句の直後。CRLF は1つの改行。
	toks := mustLex(t, "\"日本\" x\r\n  #1.f\n@\"ラ\"")
	want := []diag.Span{
		{File: "t.bx", Start: pos(1, 1), End: pos(1, 5)},
		{File: "t.bx", Start: pos(1, 6), End: pos(1, 7)},
		{File: "t.bx", Start: pos(2, 3), End: pos(2, 7)},
		{File: "t.bx", Start: pos(3, 1), End: pos(3, 5)},
		{File: "t.bx", Start: pos(3, 5), End: pos(3, 5)}, // EOF
	}
	var got []diag.Span
	for _, tok := range toks {
		got = append(got, tok.Span)
	}
	if !slices.Equal(got, want) {
		t.Errorf("spans\n got %v\nwant %v", got, want)
	}
	if toks[len(toks)-1].Kind != EOF {
		t.Errorf("last token is %v, want EOF", toks[len(toks)-1].Kind)
	}
}

func TestLexOffsets(t *testing.T) {
	toks := mustLex(t, "あ #1")
	if got := toks[1]; got.Start != 4 || got.End != 6 {
		t.Errorf("#1: offsets [%d, %d), want [4, 6)", got.Start, got.End)
	}
}

// docs/examples の .bx は字句解析でき、改行をすべて空白にしても同じトークン列になる。
func TestLexExamples(t *testing.T) {
	files, err := filepath.Glob("../../docs/examples/*.bx")
	if err != nil || len(files) == 0 {
		t.Fatalf("examples: %v %v", files, err)
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		a := mustLex(t, string(src))
		b := mustLex(t, strings.NewReplacer("\r\n", " ", "\n", " ").Replace(string(src)))
		if show(a) != show(b) {
			t.Errorf("%s: 改行を空白にするとトークン列が変わる", f)
		}
		if len(a) < 100 {
			t.Errorf("%s: トークンが %d 個しかない", f, len(a))
		}
	}
}

func TestLexErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		at   diag.Pos
		msg  string // message に含まれるべき語
	}{
		{"不正なUTF-8", "a \xff b", pos(1, 3), "UTF-8"},
		{"2行目の不正なUTF-8", "a\n日\xe6", pos(2, 2), "UTF-8"},
		{"閉じていない文字列", `x "abc`, pos(1, 3), "閉じていません"},
		{"文字列の中のLF", "\"ab\ncd\"", pos(1, 4), "改行"},
		{"文字列の中のCR", "\"ab\rcd\"", pos(1, 4), "改行"},
		{"未知のエスケープ", `"a\qb"`, pos(1, 3), `\q`},
		{"\\u が3桁", `"\u12"`, pos(1, 2), "4桁"},
		{"\\u が16進数でない", `"\u12G4"`, pos(1, 2), "4桁"},
		{"\\u がサロゲート", `"\uD800"`, pos(1, 2), "サロゲート"},
		{"文字列の終わりのバックスラッシュ", `"a\`, pos(1, 1), "閉じていません"},
		{"#0", "#0", pos(1, 1), "先頭に0"},
		{"#01", "#01", pos(1, 1), "先頭に0"},
		{"#4.0", "#4.0", pos(1, 1), "先頭に0"},
		{"# だけ", "# 1", pos(1, 1), "正の整数"},
		{"ノードIDの直後の名前", "#4a", pos(1, 3), "直後"},
		{"ノードIDの後の . だけ", "#4. x", pos(1, 3), "名前"},
		{"$ だけ", "$ x", pos(1, 1), "名前"},
		{"$ の後のキーワード", "$box", pos(1, 1), "box"},
		{"フィールドが数字で始まる", "$e.1", pos(1, 3), "名前"},
		{"フィールドがキーワード", "$e.type", pos(1, 1), "type"},
		{"@ の後が文字列でない", "@x", pos(1, 1), "文字列"},
		{"@ の後の空白", `@ "x"`, pos(1, 1), "文字列"},
		{"修飾名の中のキーワード", "std.in.x", pos(1, 1), "in"},
		{"修飾名の終わりの .", "std. x", pos(1, 4), "名前"},
		{"空白を挟んだ .", "$order . lines", pos(1, 8), "."},
		{"数の直後の名前", "12ab", pos(1, 3), "直後"},
		{"小数の後の .", "1.5.3", pos(1, 4), "."},
		{"- の後が数字でない", "-x", pos(1, 1), "-"},
		{"未知の記号", "a % b", pos(1, 3), "'%'（U+0025）"},
		{"全角の空白", "a\xe3\x80\x80b", pos(1, 2), "空白の文字 U+3000"},
		{"単独のCR", "a\rb", pos(1, 2), "CR"},
		{"BOM", "\xef\xbb\xbfmodule", pos(1, 1), "空白の文字 U+FEFF"},
		{"\\u がサロゲートの上端", `"` + "\\" + `uDFFF"`, pos(1, 2), "サロゲート"},
	}
	for _, tt := range tests {
		_, err := Lex("t.bx", []byte(tt.src))
		var d diag.Diagnostic
		if !errors.As(err, &d) {
			t.Errorf("%s: err = %v, want a diag.Diagnostic", tt.name, err)
			continue
		}
		if d.Code != diag.BX001 || d.Span.File != "t.bx" || d.Span.Start != tt.at {
			t.Errorf("%s: got %s at %s %v, want BX001 at t.bx %v", tt.name, d.Code, d.Span.File, d.Span.Start, tt.at)
		}
		if !strings.Contains(d.Message, tt.msg) {
			t.Errorf("%s: message %q does not contain %q", tt.name, d.Message, tt.msg)
		}
	}
}

// 誤りの span は、誤りの始まりから、読み進めた位置（進んでいなければ1文字）まで。
func TestLexErrorEnds(t *testing.T) {
	tests := []struct {
		src        string
		start, end diag.Pos
	}{
		{"#0 x", pos(1, 1), pos(1, 2)},
		{"a % b", pos(1, 3), pos(1, 4)},
		{`x "abc`, pos(1, 3), pos(1, 7)},
		{"#4a", pos(1, 3), pos(1, 4)},
	}
	for _, tt := range tests {
		_, err := Lex("t.bx", []byte(tt.src))
		var d diag.Diagnostic
		if !errors.As(err, &d) {
			t.Errorf("%q: err = %v", tt.src, err)
			continue
		}
		if d.Span.Start != tt.start || d.Span.End != tt.end {
			t.Errorf("%q: span %v-%v, want %v-%v", tt.src, d.Span.Start, d.Span.End, tt.start, tt.end)
		}
	}
}

func TestLexWhitespaceHint(t *testing.T) {
	_, err := Lex("t.bx", []byte("a\xe3\x80\x80b"))
	var d diag.Diagnostic
	if !errors.As(err, &d) || d.Hint == "" {
		t.Errorf("全角の空白にヒントがない: %v", err)
	}
}

func TestIdentChars(t *testing.T) {
	tests := []struct {
		r            rune
		start, conti bool
	}{
		{'a', true, true},
		{'_', true, true},
		{'あ', true, true},
		{'ー', true, true},     // Lm
		{'1', false, true},    // Nd
		{0x0301, false, true}, // 結合用アキュートアクセント（Mn）
		{'-', false, false},
		{'.', false, false},
		{'$', false, false},
		{0x3000, false, false}, // 全角の空白
		{0x2118, true, true},   // Other_ID_Start
		{0x00b7, false, true},  // Other_ID_Continue（中点）
		{0x2e2f, false, false}, // Lm だが Pattern_Syntax（垂直のチルダ）
	}
	for _, tt := range tests {
		if got := isIDStart(tt.r); got != tt.start {
			t.Errorf("isIDStart(%U) = %v, want %v", tt.r, got, tt.start)
		}
		if got := isIDContinue(tt.r); got != tt.conti {
			t.Errorf("isIDContinue(%U) = %v, want %v", tt.r, got, tt.conti)
		}
	}
}
