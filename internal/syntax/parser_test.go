package syntax

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/amisonnet8/boxundle-mvp/internal/diag"
)

// dump は、木を1行の文字列にする（テストの期待値を文法から手で書くため）。
// printer とは別の形にして、正規形の出力に頼らずに木の形を確かめる。
func dump(v any) string {
	switch v := v.(type) {
	case *File:
		head := "module "
		if v.Externs {
			head = "externs "
		}
		parts := []string{head + v.Name.Name}
		for _, d := range v.Decls {
			parts = append(parts, dump(d))
		}
		return strings.Join(parts, "\n")
	case *TypeDef:
		return "(type " + v.Name.Name + " " + dump(v.Type) + ")"
	case *Box:
		s := "(box "
		if v.Extern {
			s = "(extern box "
		}
		s += v.Name.Name
		if v.Label != nil {
			s += fmt.Sprintf(" @%q", v.Label.Value)
		}
		for _, p := range v.Ins {
			s += " (in " + p.Name.Name + " " + dump(p.Type) + ")"
		}
		for _, p := range v.Outs {
			s += " (out " + p.Name.Name + " " + dump(p.Type) + ")"
		}
		if v.Perm.None {
			s += " (perm none)"
		} else {
			s += " (perm"
			for _, n := range v.Perm.Names {
				s += " " + n.Name
			}
			s += ")"
		}
		for _, t := range v.Tests {
			s += fmt.Sprintf(" (test %q", t.Name.Name)
			for _, st := range t.Stubs {
				s += " (stub " + st.Box.Name + " " + dump(st.Outputs) + ")"
			}
			s += " (given " + dump(t.Given) + ") (expect " + dump(t.Expect) + "))"
		}
		if v.Body != nil {
			s += " (body " + dump(v.Body) + ")"
		}
		if v.Impl != nil {
			s += fmt.Sprintf(" (impl %q)", v.Impl.Name.Name)
		}
		return s + ")"
	case *TypeName:
		return v.Name
	case *ListType:
		return "[" + dump(v.Elem) + "]"
	case *MapType:
		return "{" + dump(v.Key) + " -> " + dump(v.Value) + "}"
	case *RecordType:
		var fs []string
		for _, f := range v.Fields {
			fs = append(fs, f.Name.Name+": "+dump(f.Type))
		}
		return "{" + strings.Join(fs, ", ") + "}"
	case *UnionType:
		var ms []string
		for _, m := range v.Members {
			ms = append(ms, dump(m))
		}
		return "(" + strings.Join(ms, " | ") + ")"
	case *Block:
		var ss []string
		for _, s := range v.Stmts {
			ss = append(ss, dump(s))
		}
		return "{" + strings.Join(ss, " ") + "}"
	case *Node:
		s := "(" + v.ID.Name + " " + v.Op.Name
		if v.Op.Name == "call" {
			s += " " + v.Callee.Name + " " + dumpFields(v.CallArgs)
		}
		for _, a := range v.Args {
			s += " " + dump(a)
		}
		switch len(v.Binders) {
		case 1:
			s += " as $" + v.Binders[0].Name
		case 2:
			s += " as ($" + v.Binders[0].Name + ", $" + v.Binders[1].Name + ")"
		}
		if v.Block != nil {
			s += " " + dump(v.Block)
		}
		if v.Branches != nil {
			var bs []string
			for _, b := range v.Branches {
				c := b.Cond.Name
				if b.Binder.Name != "" {
					c += " $" + b.Binder.Name
				}
				bs = append(bs, c+" "+dump(b.Block))
			}
			s += " {" + strings.Join(bs, " ") + "}"
		}
		if v.Label != nil {
			s += fmt.Sprintf(" @%q", v.Label.Value)
		}
		return s + ")"
	case *Emit:
		return "(emit $" + v.Port.Name + " " + dump(v.Value) + ")"
	case *NodeRef:
		return v.ID + dumpPath(v.Fields)
	case *DollarRef:
		return "$" + v.Name + dumpPath(v.Fields)
	case *IntLit:
		return v.Text
	case *DecLit:
		return v.Text
	case *StrLit:
		return fmt.Sprintf("%q", v.Value)
	case *BoolLit:
		return fmt.Sprint(v.Value)
	case *RecordLit:
		return v.Tag.Name + dumpFields(v.Fields)
	case *ListLit:
		var es []string
		for _, e := range v.Elems {
			es = append(es, dump(e))
		}
		return "[" + strings.Join(es, ", ") + "]"
	case *MapLit:
		if len(v.Entries) == 0 {
			return "{->}"
		}
		var es []string
		for _, e := range v.Entries {
			es = append(es, dump(e.Key)+" -> "+dump(e.Value))
		}
		return "{" + strings.Join(es, ", ") + "}"
	case *Annotated:
		return "(" + dump(v.Value) + " : " + dump(v.Type) + ")"
	}
	return fmt.Sprintf("<%T>", v)
}

func dumpFields(fs []*FieldArg) string {
	var ss []string
	for _, f := range fs {
		ss = append(ss, f.Name.Name+": "+dump(f.Value))
	}
	return "{" + strings.Join(ss, ", ") + "}"
}

func dumpPath(fs []string) string {
	s := ""
	for _, f := range fs {
		s += "." + f
	}
	return s
}

// stripSpans は、木の中の diag.Span をすべて空にする（span を除いて木を比べるため）。
func stripSpans(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			stripSpans(v.Elem())
		}
	case reflect.Slice:
		for i := range v.Len() {
			stripSpans(v.Index(i))
		}
	case reflect.Struct:
		if v.Type() == reflect.TypeFor[diag.Span]() {
			v.SetZero()
			return
		}
		for i := range v.NumField() {
			stripSpans(v.Field(i))
		}
	}
}

func mustParse(t *testing.T, file, src string) *File {
	t.Helper()
	f, err := Parse(file, []byte(src))
	if err != nil {
		t.Fatalf("Parse(%q): %v", file, err)
	}
	return f
}

const (
	typesDump = "(type Line {product: Str, qty: Int, price: Int})\n" +
		"(type Order {customer: Str, lines: [Line]})\n" +
		"(type Stock {Str -> Int})\n" +
		"(type Invoice {customer: Str, total: Int})\n" +
		"(type Shortage {products: [Str]})\n" +
		"(type OrderResult (Invoice | Shortage))"

	b7Dump = `(box B7 @"注文を処理する" (in order Order) (in stock Stock) (out result OrderResult) (out new_stock Stock) (perm none)` +
		` (test "在庫が足りる注文"` +
		` (given {order: Order{customer: "c1", lines: [Line{product: "A", qty: 2, price: 100}, Line{product: "B", qty: 1, price: 50}]}, stock: {"A" -> 5, "B" -> 1}})` +
		` (expect {result: Invoice{customer: "c1", total: 250}, new_stock: {"A" -> 3, "B" -> 0}}))` +
		` (test "1つでも足りなければ在庫不足で、在庫は変わらない"` +
		` (given {order: Order{customer: "c1", lines: [Line{product: "A", qty: 2, price: 100}, Line{product: "B", qty: 3, price: 50}]}, stock: {"A" -> 5, "B" -> 1}})` +
		` (expect {result: Shortage{products: ["B"]}, new_stock: {"A" -> 5, "B" -> 1}}))` +
		` (test "在庫表にない商品は在庫0として扱う"` +
		` (given {order: Order{customer: "c2", lines: [Line{product: "Z", qty: 1, price: 10}]}, stock: {"A" -> 5}})` +
		` (expect {result: Shortage{products: ["Z"]}, new_stock: {"A" -> 5}}))` +
		` (body {(#1 filter $order.lines as $e {(#1.1 lookup $stock $e.product 0) (#1.2 lt #1.1 $e.qty)} @"在庫が足りない明細")` +
		` (#2 map #1 as $e {(#2.1 val $e.product)} @"足りない商品")` +
		` (#3 empty #1 @"全部足りている")` +
		` (#4 route #3 {true {(#4.1 fold $order.lines $stock as ($acc, $e) {(#4.1.1 lookup $acc $e.product 0) (#4.1.2 sub #4.1.1 $e.qty) (#4.1.3 set $acc $e.product #4.1.2)} @"在庫を減らした結果")` +
		` (#4.2 map $order.lines as $e {(#4.2.1 mul $e.qty $e.price)} @"明細ごとの金額")` +
		` (#4.3 sum #4.2 @"合計金額")` +
		` (emit $result Invoice{customer: $order.customer, total: #4.3})` +
		` (emit $new_stock #4.1)}` +
		` false {(emit $result Shortage{products: #2}) (emit $new_stock $stock)}})}))`

	checkoutDump = `(box Checkout @"注文を確定する" (in order Order) (in stock Stock) (out message Str) (out ordered_at Str) (out new_stock Stock) (perm clock)` +
		` (test "在庫が足りれば受け付け、受付時刻を返す" (stub std.clock.now {now: "2026-10-25T09:00:00+09:00"})` +
		` (given {order: Order{customer: "c1", lines: [Line{product: "A", qty: 1, price: 100}]}, stock: {"A" -> 1}})` +
		` (expect {message: "注文を受け付けました", ordered_at: "2026-10-25T09:00:00+09:00", new_stock: {"A" -> 0}}))` +
		` (test "在庫が足りなければ、その旨を返す" (stub std.clock.now {now: "2026-10-25T09:00:00+09:00"})` +
		` (given {order: Order{customer: "c1", lines: [Line{product: "A", qty: 2, price: 100}]}, stock: {"A" -> 1}})` +
		` (expect {message: "在庫が不足しています", ordered_at: "2026-10-25T09:00:00+09:00", new_stock: {"A" -> 1}}))` +
		` (body {(#1 call B7 {order: $order, stock: $stock}) (#2 call std.clock.now {})` +
		` (#3 match #1.result {Invoice $inv {(#3.1 val "注文を受け付けました")} Shortage $s {(#3.2 val "在庫が不足しています")}})` +
		` (emit $message #3) (emit $ordered_at #2.now) (emit $new_stock #1.new_stock)}))`
)

func TestParseExamples(t *testing.T) {
	cases := []struct{ file, want string }{
		{"../../docs/examples/order.bx", "module shop\n" + typesDump + "\n" + b7Dump},
		{"../../docs/examples/checkout.bx", "module shop\n" + typesDump + "\n" + b7Dump + "\n" + checkoutDump},
	}
	for _, c := range cases {
		src, err := os.ReadFile(c.file)
		if err != nil {
			t.Fatal(err)
		}
		f := mustParse(t, c.file, string(src))
		if got := dump(f); got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.file, got, c.want)
		}
	}
}

// stdDocExterns は、std.md の宣言の例（externs で始まるコードブロック）を取り出す。
func stdDocExterns(t *testing.T) []string {
	t.Helper()
	src, err := os.ReadFile("../../docs/reference/std.md")
	if err != nil {
		t.Fatal(err)
	}
	var blocks []string
	for i, part := range strings.Split(string(src), "```") {
		if i%2 == 1 && strings.HasPrefix(strings.TrimPrefix(part, "\n"), "externs ") {
			blocks = append(blocks, part)
		}
	}
	return blocks
}

func TestParseStdDocExterns(t *testing.T) {
	want := []string{
		"externs std.text\n" +
			`(extern box std.text.split @"文字列を区切り文字で分割する" (in s Str) (in sep Str) (out parts [Str]) (perm none)` +
			` (test "カンマで分割" (given {s: "a,b,c", sep: ","}) (expect {parts: ["a", "b", "c"]})) (impl "text.split"))` + "\n" +
			`(extern box std.text.parse_int @"文字列を整数として読む" (in s Str) (out ok Bool) (out value Int) (perm none)` +
			` (test "数字なら読める" (given {s: " 42 "}) (expect {ok: true, value: 42}))` +
			` (test "数字でなければ失敗" (given {s: "abc"}) (expect {ok: false, value: 0})) (impl "text.parse_int"))`,
		"externs std.clock\n" +
			`(extern box std.clock.now @"現在時刻を返す" (out now Str) (perm clock) (impl "clock.now"))`,
	}
	blocks := stdDocExterns(t)
	if len(blocks) != len(want) {
		t.Fatalf("std.md の externs の例が %d 個（%d 個のつもり）", len(blocks), len(want))
	}
	for i, b := range blocks {
		if got := dump(mustParse(t, "std.md", b)); got != want[i] {
			t.Errorf("std.md の例 %d:\n got %s\nwant %s", i, got, want[i])
		}
	}
}

// 改行をすべて空白に置き換えても、span を除いて同じ木になる（mvp.md M1）。
func TestParseNewlinesAsSpaces(t *testing.T) {
	srcs := map[string]string{}
	for _, file := range []string{"../../docs/examples/order.bx", "../../docs/examples/checkout.bx"} {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		srcs[file] = string(b)
	}
	for i, b := range stdDocExterns(t) {
		srcs[fmt.Sprint("std.md の例 ", i)] = b
	}
	for name, src := range srcs {
		a := mustParse(t, name, src)
		b := mustParse(t, name, strings.ReplaceAll(src, "\n", " "))
		c := mustParse(t, name, strings.ReplaceAll(src, "\n", "\r\n"))
		for _, f := range []*File{a, b, c} {
			stripSpans(reflect.ValueOf(f))
		}
		if !reflect.DeepEqual(a, b) {
			t.Errorf("%s: 改行を空白にすると木が変わる:\n%s\n%s", name, dump(a), dump(b))
		}
		if !reflect.DeepEqual(a, c) {
			t.Errorf("%s: 改行を CRLF にすると木が変わる", name)
		}
	}
}

// bodyDump は、1つの箱の本体だけを書いて読み、本体の文の並びを返す。
func bodyDump(t *testing.T, stmts string) string {
	t.Helper()
	f := mustParse(t, "t.bx", "module m box B { perm none body { "+stmts+" } }")
	d := dump(f.Decls[0].(*Box).Body)
	return d[1 : len(d)-1]
}

func TestParseBody(t *testing.T) {
	cases := []struct{ src, want string }{
		{`#1 val 1`, `(#1 val 1)`},
		{`#1 val -2 #2 val 1.50 #3 val -0.5`, `(#1 val -2) (#2 val 1.50) (#3 val -0.5)`},
		{`#1 val "a\"b\\c\n"`, `(#1 val "a\"b\\c\n")`},
		{`#1 val true #2 val false`, `(#1 val true) (#2 val false)`},
		{`#1 len $s.a.b`, `(#1 len $s.a.b)`},
		{`emit $o #1.x.y`, `(emit $o #1.x.y)`},
		{`#1 val [] #2 val [1, [2, 3]]`, `(#1 val []) (#2 val [1, [2, 3]])`},
		{`#1 val {->}`, `(#1 val {->})`},
		{`#1 val {"a" -> 1, "b" -> 2}`, `(#1 val {"a" -> 1, "b" -> 2})`},
		{`#1 val {1 -> Foo{a: 1}}`, `(#1 val {1 -> Foo{a: 1}})`},
		{`#1 val {a: {b: 1}, c: []}`, `(#1 val {a: {b: 1}, c: []})`},
		{`#1 val Foo{} #2 val std.http.Response{status: 200}`, `(#1 val Foo{}) (#2 val std.http.Response{status: 200})`},
		{`#1 val ([] : [Int]) #2 val ({->} : {Str -> Int})`, `(#1 val ([] : [Int])) (#2 val ({->} : {Str -> Int}))`},
		{`#1 with $r {a: 1, b: "x"}`, `(#1 with $r {a: 1, b: "x"})`},
		// 引数の並びの中のノードID：演算名が続けば次のノード、名前と { が続けば参照と構築
		{`#1 val 1 #2 add #1 #1`, `(#1 val 1) (#2 add #1 #1)`},
		{`#1 eq #2 Foo{a: 1}`, `(#1 eq #2 Foo{a: 1})`},
		{`#1 val 1 #2 map{a: 1}`, `(#1 val 1) (#2 map {a: 1})`},
		{`#1 val 1 #2 foo #1`, `(#1 val 1) (#2 foo #1)`},
		{`#1 eq #2.a Foo{a: 1}`, `(#1 eq #2.a Foo{a: 1})`},
		{`#1 eq #2.a map{a: 1}`, `(#1 eq #2.a map{a: 1})`},
		{`#1 val {F{} -> 1}`, `(#1 val {F{} -> 1})`},
		{`#1 val 1 #2 #3`, `(#1 val 1 #2 #3)`},
		{`#1 val 1 #2 emit $o #1`, `(#1 val 1 #2) (emit $o #1)`},
		{`#1 map $xs as $e { #1.1 val $e } @"l"`, `(#1 map $xs as $e {(#1.1 val $e)} @"l")`},
		{`#1 fold $xs 0 as ($acc, $e) { #1.1 add $acc $e }`, `(#1 fold $xs 0 as ($acc, $e) {(#1.1 add $acc $e)})`},
		{`#1 map $xs as $e`, `(#1 map $xs as $e)`},
		{`#1 map $xs {a: 1}`, `(#1 map $xs {a: 1})`},
		{`#1 route $c { true { #1.1 val 1 } false { #1.2 val 2 } } @"x"`, `(#1 route $c {true {(#1.1 val 1)} false {(#1.2 val 2)}} @"x")`},
		{`#1 route $c as $x { true {} }`, `(#1 route $c as $x {true {}})`},
		{`#1 match $v { A $a { emit $o 1 } std.x.B $b { emit $o 2 } }`, `(#1 match $v {A $a {(emit $o 1)} std.x.B $b {(emit $o 2)}})`},
		{`#1 route $c`, `(#1 route $c)`},
		{`#1 call B {} @"l"`, `(#1 call B {} @"l")`},
		{`#1 call std.text.split {s: "a", sep: ","}`, `(#1 call std.text.split {s: "a", sep: ","})`},
		{`#1 val 1 @""`, `(#1 val 1 @"")`},
		{`#1 not true @"a" #2 val 1`, `(#1 not true @"a") (#2 val 1)`},
	}
	for _, c := range cases {
		if got := bodyDump(t, c.src); got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.src, got, c.want)
		}
	}
}

func TestParseDecls(t *testing.T) {
	cases := []struct{ src, want string }{
		{`module m type T = Int | std.x.A | B`, "module m\n(type T (Int | std.x.A | B))"},
		{`module m type T = {Str -> [Int]} type U = [{a: Int}]`, "module m\n(type T {Str -> [Int]})\n(type U [{a: Int}])"},
		{`module m type T = {a: A | B, b: {Int -> A | B}}`, "module m\n(type T {a: (A | B), b: {Int -> (A | B)}})"},
		{
			`module m box B @"l" { out $o : Int in $a : Int in $b : Str perm clock, fs.read ` +
				`test "t" { stub std.clock.now {now: "x"} stub B2 {} given {} expect {o: ([] : [Int])} } body {} }`,
			"module m\n" + `(box B @"l" (in a Int) (in b Str) (out o Int) (perm clock fs.read)` +
				` (test "t" (stub std.clock.now {now: "x"}) (stub B2 {}) (given {}) (expect {o: ([] : [Int])})) (body {}))`,
		},
		{
			`module m box A { perm none body {} } type T = Int box C { perm none test "a" { given {} expect {} } test "b" { given {} expect {} } body {} }`,
			"module m\n(box A (perm none) (body {}))\n(type T Int)\n" + `(box C (perm none) (test "a" (given {}) (expect {})) (test "b" (given {}) (expect {})) (body {}))`,
		},
		// 利用者のモジュールの extern box も読む（BX070 は check で返す）
		{`module m extern box std.x.f { perm none impl builtin "x.f" }`, "module m\n" + `(extern box std.x.f (perm none) (impl "x.f"))`},
		{
			`externs std.x type std.x.T = {a: Int} extern box std.x.f @"l" { in $a : Int out $r : std.x.T perm none ` +
				`test "t" { given {a: 1} expect {r: std.x.T{a: 1}} } impl builtin "x.f" }`,
			"externs std.x\n(type std.x.T {a: Int})\n" +
				`(extern box std.x.f @"l" (in a Int) (out r std.x.T) (perm none) (test "t" (given {a: 1}) (expect {r: std.x.T{a: 1}})) (impl "x.f"))`,
		},
	}
	for _, c := range cases {
		if got := dump(mustParse(t, "t.bx", c.src)); got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.src, got, c.want)
		}
	}
	f := mustParse(t, "t.bx", "module m")
	if f.Externs || f.Decls != nil {
		t.Errorf("module m: Externs=%v Decls=%v", f.Externs, f.Decls)
	}
}

func sp(l1, c1, l2, c2 int) diag.Span {
	return diag.Span{File: "t.bx", Start: diag.Pos{Line: l1, Col: c1}, End: diag.Pos{Line: l2, Col: c2}}
}

func TestParseSpans(t *testing.T) {
	src := "module m\n" +
		"box B {\n" +
		"  in $xs : [Int]\n" +
		"  perm none\n" +
		"  body {\n" +
		"    #1 map $xs as $e {\n" +
		"      #1.1 val Foo{a: $e}\n" +
		"    } @\"l\"\n" +
		"  }\n" +
		"}\n"
	f := mustParse(t, "t.bx", src)
	box := f.Decls[0].(*Box)
	n1 := box.Body.Stmts[0].(*Node)
	n11 := n1.Block.Stmts[0].(*Node)
	cases := []struct {
		what      string
		got, want diag.Span
	}{
		{"ファイル", f.Span, sp(1, 1, 10, 2)},
		{"箱", box.Span, sp(2, 1, 10, 2)},
		{"箱の名前", box.Name.Span, sp(2, 5, 2, 6)},
		{"ポート", box.Ins[0].Span, sp(3, 3, 3, 17)},
		{"ポートの型", box.Ins[0].Type.Pos(), sp(3, 12, 3, 17)},
		{"perm", box.Perm.Span, sp(4, 3, 4, 12)},
		{"本体", box.Body.Span, sp(5, 8, 9, 4)},
		{"#1", n1.Span, sp(6, 5, 8, 11)},
		{"#1 のブロック", n1.Block.Span, sp(6, 22, 8, 6)},
		{"#1 のラベル", n1.Label.Span, sp(8, 7, 8, 11)},
		{"#1.1", n11.Span, sp(7, 7, 7, 26)},
		{"#1.1 の引数", n11.Args[0].Pos(), sp(7, 16, 7, 26)},
		{"#1.1 の引数のフィールド", n11.Args[0].(*RecordLit).Fields[0].Span, sp(7, 20, 7, 25)},
	}
	src2 := "module m\n" +
		"type T = A | {a: Int}\n" +
		"box B {\n" +
		"  perm none\n" +
		"  test \"t\" {\n" +
		"    given {x: {1 -> 2}}\n" +
		"    expect {}\n" +
		"  }\n" +
		"  body {}\n" +
		"}\n"
	f2 := mustParse(t, "t.bx", src2)
	u := f2.Decls[0].(*TypeDef).Type.(*UnionType)
	test := f2.Decls[1].(*Box).Tests[0]
	cases = append(cases, []struct {
		what      string
		got, want diag.Span
	}{
		{"ユニオン", u.Span, sp(2, 10, 2, 22)},
		{"レコード型のフィールド", u.Members[1].(*RecordType).Fields[0].Span, sp(2, 15, 2, 21)},
		{"テスト", test.Span, sp(5, 3, 8, 4)},
		{"given", test.Given.Span, sp(6, 11, 6, 24)},
		{"マップの組", test.Given.Fields[0].Value.(*MapLit).Entries[0].Span, sp(6, 16, 6, 22)},
	}...)
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.what, c.got, c.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	const pre = "module m box B { perm none body { "
	cases := []struct {
		src       string
		msg       string
		span      diag.Span
		box, node string
	}{
		{"", "ここには「module」か「externs」が要りますが、ファイルの終わりがあります", sp(1, 1, 1, 1), "", ""},
		{"module a.b", "モジュールの名前に . は使えません", sp(1, 8, 1, 11), "", ""},
		{"module m foo", "ここには「type」か「box」が要りますが、名前「foo」があります", sp(1, 10, 1, 13), "", ""},
		{"externs std.x box B {", "標準の箱の宣言（externs）には、extern box だけが書けます", sp(1, 15, 1, 18), "", ""},
		{"externs std.x extern B", "ここには「box」が要りますが、名前「B」があります", sp(1, 22, 1, 23), "", ""},
		{"module m type a.b = Int", "型の名前に . は使えません", sp(1, 15, 1, 18), "", ""},
		{"module m type T Int", "ここには「=」が要りますが、名前「Int」があります", sp(1, 17, 1, 20), "", ""},
		{"module m type T = ,", "ここには型が要りますが、「,」があります", sp(1, 19, 1, 20), "", ""},
		{"module m type T = {Str Int}", "ここには「->」が要りますが、名前「Int」があります", sp(1, 24, 1, 27), "", ""},
		{"module m type T = {a: Int b: Str}", "ここには「}」が要りますが、名前「b」があります", sp(1, 27, 1, 28), "", ""},
		{"module m type T = {a.b: Int}", "フィールドの名前に . は使えません", sp(1, 20, 1, 23), "", ""},
		{"module m type T = [Int", "ここには「]」が要りますが、ファイルの終わりがあります", sp(1, 23, 1, 23), "", ""},
		{"module m type T = {Int -> Str", "ここには「}」が要りますが、ファイルの終わりがあります", sp(1, 30, 1, 30), "", ""},
		{"module m type T = Int | ,", "ここには型が要りますが、「,」があります", sp(1, 25, 1, 26), "", ""},
		{"module m box a.b {", "箱の名前に . は使えません", sp(1, 14, 1, 17), "", ""},
		{"module m box B perm", "ここには「{」が要りますが、キーワード「perm」があります", sp(1, 16, 1, 20), "B", ""},
		{"module m box B { body {} }", "ここには「in」「out」か「perm」が要りますが、キーワード「body」があります", sp(1, 18, 1, 22), "B", ""},
		{"module m box B { perm none }", "ここには「test」か「body」が要りますが、「}」があります", sp(1, 28, 1, 29), "B", ""},
		{"module m box B { perm none body {}", "ここには「}」が要りますが、ファイルの終わりがあります", sp(1, 35, 1, 35), "B", ""},
		{"module m box B { in order : Int perm none body {} }", "ここにはポート（$名前）が要りますが、名前「order」があります", sp(1, 21, 1, 26), "B", ""},
		{"module m box B { in $o.x : Int perm none body {} }", "ポート（$名前）にフィールドは付けられません", sp(1, 21, 1, 25), "B", ""},
		{"module m box B { in $o Int perm none body {} }", "ここには「:」が要りますが、名前「Int」があります", sp(1, 24, 1, 27), "B", ""},
		{"module m box B { perm }", "ここには「none」か権限の名前が要りますが、「}」があります", sp(1, 23, 1, 24), "B", ""},
		{"module m box B { perm a, }", "ここには「none」か権限の名前が要りますが、「}」があります", sp(1, 26, 1, 27), "B", ""},
		{"module m box B { perm none test x {} body {} }", "ここにはテストの名前（文字列）が要りますが、名前「x」があります", sp(1, 33, 1, 34), "B", ""},
		{`module m box B { perm none test "t" given }`, "ここには「{」が要りますが、キーワード「given」があります", sp(1, 37, 1, 42), "B", ""},
		{`module m box B { perm none test "t" { expect {} } body {} }`, "ここには「stub」か「given」が要りますが、キーワード「expect」があります", sp(1, 39, 1, 45), "B", ""},
		{`module m box B { perm none test "t" { given {} } body {} }`, "ここには「expect」が要りますが、「}」があります", sp(1, 48, 1, 49), "B", ""},
		{`module m box B { perm none test "t" { given {} expect {} body {} }`, "ここには「}」が要りますが、キーワード「body」があります", sp(1, 58, 1, 62), "B", ""},
		{`module m box B { perm none test "t" { stub {} given {} expect {} } body {} }`, "ここにはスタブにする箱の名前が要りますが、「{」があります", sp(1, 44, 1, 45), "B", ""},
		{`module m box B { perm none test "t" { given {a: $x} expect {} } body {} }`, "テストの値には参照（#…・$…）は書けません。書けるのはリテラルと構築だけです", sp(1, 49, 1, 51), "B", ""},
		{`module m box B { perm none test "t" { given {} expect {a: [1, F{b: {"k" -> #1.x}}]} } body {} }`, "テストの値には参照（#…・$…）は書けません。書けるのはリテラルと構築だけです", sp(1, 76, 1, 80), "B", ""},
		{`module m box B { perm none test "t" { given {a: {#1 -> 1}} expect {} } body {} }`, "テストの値には参照（#…・$…）は書けません。書けるのはリテラルと構築だけです", sp(1, 50, 1, 52), "B", ""},
		{`module m box B { perm none test "t" { given {a: ($x : Int)} expect {} } body {} }`, "テストの値には参照（#…・$…）は書けません。書けるのはリテラルと構築だけです", sp(1, 50, 1, 52), "B", ""},
		{`module m box B { perm none test "t" { stub S {a: $x} given {} expect {} } body {} }`, "テストの値には参照（#…・$…）は書けません。書けるのはリテラルと構築だけです", sp(1, 50, 1, 52), "B", ""},
		{pre + "#1 } }", "ここには演算名が要りますが、「}」があります", sp(1, 38, 1, 39), "B", "#1"},
		{pre + "#1.x val 1 } }", "ノードの始まりのIDにフィールドは付けられません", sp(1, 35, 1, 39), "B", ""},
		{pre + "#1 std.x 1 } }", "演算名に . は使えません", sp(1, 38, 1, 43), "B", "#1"},
		{pre + "1 } }", "ここにはノードか「emit」か「}」が要りますが、整数「1」があります", sp(1, 35, 1, 36), "B", ""},
		{pre + "#1 val , } }", "ここにはノードか「emit」か「}」が要りますが、「,」があります", sp(1, 42, 1, 43), "B", ""},
		{pre + "#1 map $x as $e { #1.1 } } }", "ここには演算名が要りますが、「}」があります", sp(1, 58, 1, 59), "B", "#1.1"},
		{pre + "#1 val {} } }", "値の位置に {} は書けません。空のマップは {->} と書きます", sp(1, 42, 1, 44), "B", "#1"},
		{pre + "#1 val {-> 1} } }", "ここには「}」が要りますが、整数「1」があります", sp(1, 46, 1, 47), "B", "#1"},
		{pre + "#1 val {1 2} } }", "ここには「->」が要りますが、整数「2」があります", sp(1, 45, 1, 46), "B", "#1"},
		{pre + "#1 val {1 -> 2 3} } }", "ここには「}」が要りますが、整数「3」があります", sp(1, 50, 1, 51), "B", "#1"},
		{pre + "#1 val {a: 1 b: 2} } }", "ここには「}」が要りますが、名前「b」があります", sp(1, 48, 1, 49), "B", "#1"},
		{pre + "#1 val {a.b: 1} } }", "フィールドの名前に . は使えません", sp(1, 43, 1, 46), "B", "#1"},
		{pre + "#1 val F{a 1} } }", "ここには「:」が要りますが、整数「1」があります", sp(1, 46, 1, 47), "B", "#1"},
		{pre + "#1 val [1 2] } }", "ここには「]」が要りますが、整数「2」があります", sp(1, 45, 1, 46), "B", "#1"},
		{pre + "#1 val [1, ] } }", "ここには値が要りますが、「]」があります", sp(1, 46, 1, 47), "B", "#1"},
		{pre + "#1 val (1 Int) } }", "ここには「:」が要りますが、名前「Int」があります", sp(1, 45, 1, 48), "B", "#1"},
		{pre + "#1 val (1 : Int } }", "ここには「)」が要りますが、「}」があります", sp(1, 51, 1, 52), "B", "#1"},
		{pre + "#1 val (1 : ) } }", "ここには型が要りますが、「)」があります", sp(1, 47, 1, 48), "B", "#1"},
		{pre + "#1 val ( } }", "ここには値が要りますが、「}」があります", sp(1, 44, 1, 45), "B", "#1"},
		{pre + "#1 map $x as 1 } }", "ここには束縛子（$名前）か「(」が要りますが、整数「1」があります", sp(1, 48, 1, 49), "B", "#1"},
		{pre + "#1 fold $x 0 as ($a $b) {} } }", "ここには「,」が要りますが、ポート・束縛子「$b」があります", sp(1, 55, 1, 57), "B", "#1"},
		{pre + "#1 fold $x 0 as (1, $b) {} } }", "ここには束縛子（$名前）が要りますが、整数「1」があります", sp(1, 52, 1, 53), "B", "#1"},
		{pre + "#1 fold $x 0 as ($a, 1) {} } }", "ここには束縛子（$名前）が要りますが、整数「1」があります", sp(1, 56, 1, 57), "B", "#1"},
		{pre + "#1 fold $x 0 as ($a, $b {} } }", "ここには「)」が要りますが、「{」があります", sp(1, 59, 1, 60), "B", "#1"},
		{pre + "#1 route $c { 1 {} } } }", "ここには分岐（「true」「false」か、型の名前と束縛子）か「}」が要りますが、整数「1」があります", sp(1, 49, 1, 50), "B", "#1"},
		{pre + "#1 match $v { A {} } } }", "ここには束縛子（$名前）が要りますが、「{」があります", sp(1, 51, 1, 52), "B", "#1"},
		{pre + "#1 route $c { true 1 } } }", "ここには「{」が要りますが、整数「1」があります", sp(1, 54, 1, 55), "B", "#1"},
		{pre + "#1 call {} } }", "ここには呼び出す箱の名前が要りますが、「{」があります", sp(1, 43, 1, 44), "B", "#1"},
		{pre + "#1 call B } }", "ここには「{」が要りますが、「}」があります", sp(1, 45, 1, 46), "B", "#1"},
		{pre + "#1 call B {a: 1,} } }", "ここにはフィールドの名前が要りますが、「}」があります", sp(1, 51, 1, 52), "B", "#1"},
		{pre + "emit result 1 } }", "ここには出力ポート（$名前）が要りますが、名前「result」があります", sp(1, 40, 1, 46), "B", ""},
		{pre + "emit $o.x 1 } }", "出力ポート（$名前）にフィールドは付けられません", sp(1, 40, 1, 44), "B", ""},
		{pre + "emit $o } }", "ここには値が要りますが、「}」があります", sp(1, 43, 1, 44), "B", ""},
		{"externs std.x extern box std.x.f { perm none body {} }", "ここには「test」か「impl」が要りますが、キーワード「body」があります", sp(1, 46, 1, 50), "std.x.f", ""},
		{`externs std.x extern box std.x.f { perm none impl foo "x" }`, "ここには「builtin」が要りますが、名前「foo」があります", sp(1, 51, 1, 54), "std.x.f", ""},
		{`externs std.x extern box std.x.f { perm none impl builtin x }`, "ここには実装の登録名（文字列）が要りますが、名前「x」があります", sp(1, 59, 1, 60), "std.x.f", ""},
		{`externs std.x extern box std.x.f { perm none impl "x" }`, "ここには「builtin」が要りますが、文字列「\"x\"」があります", sp(1, 51, 1, 54), "std.x.f", ""},
		{"externs std.x extern box { perm none }", "ここには箱の名前が要りますが、「{」があります", sp(1, 26, 1, 27), "", ""},
		{"externs ,", "ここには名前空間が要りますが、「,」があります", sp(1, 9, 1, 10), "", ""},
		{"module box", "ここにはモジュールの名前が要りますが、キーワード「box」があります", sp(1, 8, 1, 11), "", ""},
		{"module m type T = Int box", "ここには箱の名前が要りますが、ファイルの終わりがあります", sp(1, 26, 1, 26), "", ""},
	}
	for _, c := range cases {
		_, err := Parse("t.bx", []byte(c.src))
		var d diag.Diagnostic
		if !errors.As(err, &d) {
			t.Errorf("%s: 誤りが diag.Diagnostic でない: %v", c.src, err)
			continue
		}
		want := diag.Diagnostic{Code: diag.BX001, Severity: diag.Error, Message: c.msg, Span: c.span, Box: c.box, Node: c.node}
		if !reflect.DeepEqual(d, want) {
			t.Errorf("%s:\n got %+v\nwant %+v", c.src, d, want)
		}
	}
}

// 字句の誤りは、字句解析のものがそのまま返る。
func TestParseLexError(t *testing.T) {
	_, err := Parse("t.bx", []byte("module m\xff"))
	var d diag.Diagnostic
	if !errors.As(err, &d) || d.Code != diag.BX001 || d.Message != "不正な UTF-8 のバイト列です" {
		t.Errorf("got %v", err)
	}
}

// 演算名の一覧は、language.md §4.3 の表の1列目にある名前と一致する。
func TestIsOp(t *testing.T) {
	src, err := os.ReadFile("../../docs/reference/language.md")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	sec := s[strings.Index(s, "### 4.3"):strings.Index(s, "### 4.4")]
	name := regexp.MustCompile("`([a-z_]+)`")
	var want []string
	for line := range strings.SplitSeq(sec, "\n") {
		if !strings.HasPrefix(line, "| ") {
			continue
		}
		cell := strings.Split(line, "|")[1]
		for _, m := range name.FindAllStringSubmatch(cell, -1) {
			want = append(want, m[1])
		}
	}
	slices.Sort(want)
	got := slices.Sorted(maps.Keys(ops))
	if !slices.Equal(got, want) {
		t.Errorf("演算名の一覧が language.md §4.3 と違う:\n got %v\nwant %v", got, want)
	}
	for _, s := range want {
		if !IsOp(s) {
			t.Errorf("IsOp(%q) = false", s)
		}
	}
	for _, s := range []string{"foo", "Map", "emit", ""} {
		if IsOp(s) {
			t.Errorf("IsOp(%q) = true", s)
		}
	}
}
