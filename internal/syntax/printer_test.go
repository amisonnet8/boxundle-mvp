package syntax

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// exampleSources は、正規形で書かれた docs の例（order.bx・checkout.bx・std.md の宣言の例）。
func exampleSources(t *testing.T) map[string]string {
	t.Helper()
	srcs := map[string]string{}
	for _, file := range []string{"../../docs/examples/order.bx", "../../docs/examples/checkout.bx"} {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		srcs[file] = string(b)
	}
	for i, b := range stdDocExterns(t) {
		srcs["std.md の例 "+string(rune('0'+i))] = strings.TrimPrefix(b, "\n")
	}
	return srcs
}

// docs の例は正規形で書いてあるので、fmt にかけても変わらない。
// 改行を空白にしたものを fmt にかけても、元の例に戻る。
func TestPrintExamples(t *testing.T) {
	for name, src := range exampleSources(t) {
		for _, in := range []string{src, strings.ReplaceAll(src, "\n", " ")} {
			if got := string(Print(mustParse(t, name, in))); got != src {
				t.Errorf("%s: fmt の結果が例と違う:\n%s", name, got)
			}
		}
	}
}

// roundTripSources は、parse(print(ast)) == ast と冪等を確かめる入力。
func roundTripSources(t *testing.T) []string {
	srcs := []string{
		`module m type T = {a: A | B, b: {Int -> [Str]}} type U = X | std.y.Z`,
		`module m box B @"l" { out $o : Int in $a : Int perm clock, net test "t" { stub S {} stub T {a: [1, {->}]} given {} expect {o: ([] : [Int])} } body {} }`,
		`externs std.x type std.x.T = Int extern box std.x.f @"l" { in $a : std.x.T perm none test "t" { given {a: 1} expect {} } impl builtin "x.f" }`,
		`module m extern box std.x.f { perm none impl builtin "x.f" }`,
		`module m box B { perm none body { ` +
			`#1 route $c { true { #1.1 val 1 } false {} } @"r" ` +
			`#2 fold $xs 0 as ($acc, $e) { #2.1 add $acc $e } ` +
			`#3 call B {} @"c" #4 call std.text.split {s: "a", sep: ","} ` +
			`#5 val ([] : [Int]) #6 with $r {a: 1} #7 val {->} #8 val {"a" -> F{x: -1.50}} ` +
			`#9 match $v { A $a { emit $o 1 } std.x.B $b {} } ` +
			`#10 eq #9 Foo{a: 1} #11 val 1 #12 #13 #14 eq #2.a map{a: 1} ` +
			`#15 map $xs as $e #16 map $xs {a: 1} #17 route $c #18 route $c as $x { true {} } ` +
			`#19 val "a\"b\\c\n\t" @"" #20 foo #1 ` +
			`emit $o $e.a.b } }`,
	}
	for _, src := range exampleSources(t) {
		srcs = append(srcs, src)
	}
	return srcs
}

func TestPrintRoundTrip(t *testing.T) {
	for _, src := range roundTripSources(t) {
		a := mustParse(t, "t.bx", src)
		out := Print(a)
		b := mustParse(t, "t.bx", string(out))
		if again := Print(b); string(again) != string(out) {
			t.Errorf("fmt が冪等でない:\n%s\n%s", out, again)
		}
		stripSpans(reflect.ValueOf(a))
		stripSpans(reflect.ValueOf(b))
		if !reflect.DeepEqual(a, b) {
			t.Errorf("parse(print(ast)) が元と違う:\n%s\n%s\n%s", out, dump(a), dump(b))
		}
	}
}

func TestPrint(t *testing.T) {
	cases := []struct{ src, want string }{
		{"module m", "module m\n"},
		{
			`module m type T = {a: A | B, b: {Int -> [Str]}} box B @"l" { out $o : Int in $a : Int out $p : Str perm clock, net body { } }`,
			"module m\n" +
				"\n" +
				"type T = {a: A | B, b: {Int -> [Str]}}\n" +
				"\n" +
				"box B @\"l\" {\n" +
				"  in $a : Int\n" +
				"  out $o : Int\n" +
				"  out $p : Str\n" +
				"  perm clock, net\n" +
				"  body {\n" +
				"  }\n" +
				"}\n",
		},
		{
			`module m box B { perm none body { #1 route $c { true { #1.1 val 1 } false { } } @"r" ` +
				`#2 fold $xs 0 as ($acc, $e) { #2.1 add $acc $e } #3 call B {} @"c" #4 val ([] : [Int]) ` +
				`#5 with $r {a: 1} #6 val {->} #7 match $v { A $a { emit $o 1 } } } }`,
			"module m\n" +
				"\n" +
				"box B {\n" +
				"  perm none\n" +
				"  body {\n" +
				"    #1 route $c {\n" +
				"      true {\n" +
				"        #1.1 val 1\n" +
				"      }\n" +
				"      false {\n" +
				"      }\n" +
				"    } @\"r\"\n" +
				"    #2 fold $xs 0 as ($acc, $e) {\n" +
				"      #2.1 add $acc $e\n" +
				"    }\n" +
				"    #3 call B {} @\"c\"\n" +
				"    #4 val ([] : [Int])\n" +
				"    #5 with $r {a: 1}\n" +
				"    #6 val {->}\n" +
				"    #7 match $v {\n" +
				"      A $a {\n" +
				"        emit $o 1\n" +
				"      }\n" +
				"    }\n" +
				"  }\n" +
				"}\n",
		},
		{
			`module m box B { perm none test "t" { stub S {} stub std.clock.now {now: "x", y: 1} given {} expect {o: 1} } ` +
				`test "u" { given {a: 1, b: [2, 3]} expect {} } body {} }`,
			"module m\n" +
				"\n" +
				"box B {\n" +
				"  perm none\n" +
				"\n" +
				"  test \"t\" {\n" +
				"    stub S {}\n" +
				"    stub std.clock.now { now: \"x\", y: 1 }\n" +
				"    given {}\n" +
				"    expect {\n" +
				"      o: 1\n" +
				"    }\n" +
				"  }\n" +
				"\n" +
				"  test \"u\" {\n" +
				"    given {\n" +
				"      a: 1,\n" +
				"      b: [2, 3]\n" +
				"    }\n" +
				"    expect {}\n" +
				"  }\n" +
				"\n" +
				"  body {\n" +
				"  }\n" +
				"}\n",
		},
		{
			`externs std.x type std.x.T = Int extern box std.x.f { in $a : std.x.T perm none impl builtin "x.f" }`,
			"externs std.x\n" +
				"\n" +
				"type std.x.T = Int\n" +
				"\n" +
				"extern box std.x.f {\n" +
				"  in $a : std.x.T\n" +
				"  perm none\n" +
				"  impl builtin \"x.f\"\n" +
				"}\n",
		},
	}
	for _, c := range cases {
		if got := string(Print(mustParse(t, "t.bx", c.src))); got != c.want {
			t.Errorf("%s:\n got:\n%s\nwant:\n%s", c.src, got, c.want)
		}
	}
}

func TestQuote(t *testing.T) {
	const bs = "\\"
	cases := []struct{ in, want string }{
		{"", `""`},
		{"a\"b\\c\nd\te", `"a\"b\\c\nd\te"`},
		{"\x00\x1f\r\x7f", `"` + bs + "u0000" + bs + "u001F" + bs + "u000D" + bs + "u007F" + `"`},
		{"\xc2\x9f\xc2\xa0", `"` + bs + "u009F" + "\xc2\xa0" + `"`},                                                                     // U+009F は書き換え、U+00A0 はそのまま
		{"\xe2\x80\xa7\xe2\x80\xa8\xe2\x80\xa9\xe2\x80\xaa", `"` + "\xe2\x80\xa7" + bs + "u2028" + bs + "u2029" + "\xe2\x80\xaa" + `"`}, // U+2027〜U+202A
		{" ~あ", `" ~あ"`},
	}
	for _, c := range cases {
		if got := quote(c.in); got != c.want {
			t.Errorf("quote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// quote の結果を字句解析で読むと、すべての文字（サロゲートを除く）が元に戻る。
func TestQuoteLexRoundTrip(t *testing.T) {
	var b strings.Builder
	for r := rune(0); r <= utf8.MaxRune; r++ {
		if utf8.ValidRune(r) {
			b.WriteRune(r)
		}
	}
	s := b.String()
	toks, err := Lex("t.bx", []byte(quote(s)))
	if err != nil {
		t.Fatal(err)
	}
	if toks[0].Kind != String || toks[0].Value != s || toks[1].Kind != EOF {
		t.Errorf("quote の結果を読み戻すと、元の文字列にならない")
	}
}
