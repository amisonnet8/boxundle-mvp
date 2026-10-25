package check

import (
	"strings"
	"testing"

	"github.com/amisonnet8/boxundle-mvp/internal/syntax"
)

// typeSrc は、いろいろな型の入力を持つ箱 T の本体に body を入れたモジュール。body は23行目から始まる。
func typeSrc(body string) string {
	return "module m\n\ntype Line = {product: Str, qty: Int}\n\ntype A = {x: Int}\n\ntype B = {y: Int}\n\ntype U = A | B\n\n" +
		"box T @\"x\" {\n  in $i : Int\n  in $d : Dec\n  in $s : Str\n  in $b : Bool\n  in $xs : [Int]\n  in $m : {Str -> Int}\n  in $l : Line\n  in $u : U\n  out $r : Int\n  perm none\n  body {\n" +
		body + "  }\n}\n"
}

// 全演算を正しく使った本体には診断が無く、各ノードの型が language.md §4.3 の表どおりになる。
func TestTypecheckAllOps(t *testing.T) {
	body := "    #1 add $i 1\n    #2 add $d 1.5\n    #3 div $d 2.0\n    #4 idiv $i 2\n    #5 mod $i 2\n    #6 neg $d\n    #7 abs $i\n" +
		"    #8 min $i 3\n    #9 max $d 0.5\n    #10 to_dec $i\n    #11 floor $d\n    #12 round $d 2\n    #13 to_str $b\n" +
		"    #14 eq $u A{x: 1}\n    #15 ne $s \"a\"\n    #16 lt $s \"b\"\n    #17 ge $d 1.0\n    #18 and $b #16\n    #19 not #18\n" +
		"    #20 concat $xs [1, 2]\n    #21 concat $s \"x\"\n    #22 len $m\n    #23 empty $s\n    #24 sum $xs\n    #25 reverse $xs\n" +
		"    #26 take $xs 2\n    #27 distinct $xs\n    #28 range 3\n    #29 lookup $m \"a\" 0\n    #30 has $m \"a\"\n    #31 set $m \"a\" 1\n" +
		"    #32 remove $m \"a\"\n    #33 keys $m\n    #34 values $m\n    #35 with $l {qty: 3}\n" +
		"    #36 map $xs as $e {\n      #36.1 to_str $e\n    }\n" +
		"    #37 flat_map $xs as $e {\n      #37.1 val [$e, $e]\n    }\n" +
		"    #38 filter $xs as $e {\n      #38.1 gt $e 0\n    }\n" +
		"    #39 any $xs as $e {\n      #39.1 eq $e 1\n    }\n" +
		"    #40 all $xs as $e {\n      #40.1 lt $e 9\n    }\n" +
		"    #41 sort_by $xs as $e {\n      #41.1 neg $e\n    }\n" +
		"    #42 group_by $xs as $e {\n      #42.1 mod $e 2\n    }\n" +
		"    #43 fold $xs ([] : [U]) as ($acc, $e) {\n      #43.1 route #18 {\n        true {\n          #43.1.1 val [A{x: $e}]\n        }\n        false {\n          #43.1.2 val $acc\n        }\n      }\n    }\n" +
		"    #44 match $u {\n      A $a {\n        #44.1 val $a.x\n      }\n      B $bb {\n        #44.2 val $bb.y\n      }\n    }\n" +
		"    #45 call std.text.split {s: $s, sep: \",\"}\n    #46 len #45.parts\n" +
		"    #47 val Line{product: \"p\", qty: #44}\n    #48 val {k: [Line{product: $l.product, qty: 1}]}\n    #49 val {\"a\" -> [0]}\n" +
		"    #50 fold $xs ({->} : {Str -> [U]}) as ($acc, $e) {\n      #50.1 val {\"k\" -> []}\n    }\n" +
		"    #51 call T2 {v: []}\n    #52 add #51.w 1\n    #53 val Line{product: \"p\", qty: 1}\n" +
		"    #54 val ({k: A{x: 1}} : {k: U})\n    #55 sub $i 1\n    #56 mul $d 2.0\n    #57 le $i 2\n    #58 or $b #57\n    #59 drop $xs 1\n" +
		"    emit $r #24\n"
	src := typeSrc(body) + "\nbox T2 @\"y\" {\n  in $v : [Int]\n  out $w : Int\n  perm none\n  body {\n    #1 len $v\n    emit $w #1\n  }\n}\n"
	f := mustParse(t, "t.bx", src)
	c := checkModule(f, stdDecls(t))
	if len(c.diags) > 0 {
		t.Fatalf("診断:\n%s", diagText(t, c.diags))
	}
	want := []string{
		"#1 Int", "#2 Dec", "#3 Dec", "#4 Int", "#5 Int", "#6 Dec", "#7 Int", "#8 Int", "#9 Dec", "#10 Dec",
		"#11 Int", "#12 Dec", "#13 Str", "#14 Bool", "#15 Bool", "#16 Bool", "#17 Bool", "#18 Bool", "#19 Bool",
		"#20 [Int]", "#21 Str", "#22 Int", "#23 Bool", "#24 Int", "#25 [Int]", "#26 [Int]", "#27 [Int]", "#28 [Int]",
		"#29 Int", "#30 Bool", "#31 {Str -> Int}", "#32 {Str -> Int}", "#33 [Str]", "#34 [Int]", "#35 Line",
		"#36 [Str]", "#36.1 Str", "#37 [Int]", "#37.1 [Int]", "#38 [Int]", "#38.1 Bool", "#39 Bool", "#39.1 Bool",
		"#40 Bool", "#40.1 Bool", "#41 [Int]", "#41.1 Int", "#42 {Int -> [Int]}", "#42.1 Int",
		"#43 [A | B]", "#43.1 [A | B]", "#43.1.1 [A | B]", "#43.1.2 [A | B]",
		"#44 Int", "#44.1 Int", "#44.2 Int", "#45 {parts: [Str]}", "#46 Int", "#47 Line", "#48 {k: [Line]}",
		"#49 {Str -> [Int]}", "#50 {Str -> [A | B]}", "#50.1 {Str -> [A | B]}", "#51 {w: Int}", "#52 Int", "#53 Line",
		"#54 {k: A | B}", "#55 Int", "#56 Dec", "#57 Bool", "#58 Bool", "#59 [Int]",
	}
	var box *syntax.Box
	for _, d := range f.Decls {
		if b, ok := d.(*syntax.Box); ok && b.Name.Name == "T" {
			box = b
		}
	}
	var got []string
	var walk func(b *syntax.Block)
	walk = func(b *syntax.Block) {
		for _, st := range b.Stmts {
			n, ok := st.(*syntax.Node)
			if !ok {
				continue
			}
			got = append(got, n.ID.Name+" "+c.nodeTypes[n].String())
			for _, inner := range innerBlocks(n) {
				walk(inner)
			}
		}
	}
	walk(box.Body)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// 型検査の演算の表は、パーサの演算名と一致する。
func TestOpNamesMatchParser(t *testing.T) {
	if got, want := opNames(), syntax.OpNames(); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("got  %v\nwant %v", got, want)
	}
}

// bad は、箱 T のノード node の BX020 の全文。
func bad(node, pos, what, want, got string) string {
	head := "エラー BX020  箱 T"
	if node != "" {
		head += "  ノード " + node
	}
	return head + "\n  " + what + "の型が合いません（期待した型 " + want + "、実際の型 " + got + "）\n  位置: t.bx:" + pos + "\n"
}

// 演算ごとの引数の誤りと、そのときのノード #1 の型（結果が引数に依らなければその型、依れば ?）。
func TestTypecheckOpArgs(t *testing.T) {
	stdFiles := stdDecls(t)
	block := func(op, arg, inner string) string {
		return "#1 " + op + " " + arg + " as $e {\n      #1.1 " + inner + "\n    }"
	}
	route := "#1 route $b {\n      true {\n        #1.1 val 1\n      }\n    }"
	mixed := "#1 route $b {\n      true {\n        #1.1 val 1\n      }\n      false {\n        emit $r 1\n      }\n    }"
	emptyBranch := "#1 route $b {\n      true {\n      }\n      false {\n        #1.2 val 1\n      }\n    }"
	tests := []struct {
		body, want, typ string
	}{
		{"#1 div $i $d", bad("#1", "23:12", "div の1つ目の引数", "Dec", "Int"), "?"},
		{"#1 div $d $i", bad("#1", "23:15", "div の2つ目の引数", "Dec", "Int"), "?"},
		{"#1 idiv $d $i", bad("#1", "23:13", "idiv の1つ目の引数", "Int", "Dec"), "?"},
		{"#1 mod $i $d", bad("#1", "23:15", "mod の2つ目の引数", "Int", "Dec"), "?"},
		{"#1 neg $s", bad("#1", "23:12", "neg の引数", "Int か Dec", "Str"), "?"},
		{"#1 to_dec $d", bad("#1", "23:15", "to_dec の引数", "Int", "Dec"), "Dec"},
		{"#1 floor $i", bad("#1", "23:14", "floor の引数", "Dec", "Int"), "Int"},
		{"#1 round $i 2", bad("#1", "23:14", "round の1つ目の引数", "Dec", "Int"), "Dec"},
		{"#1 round $d $d", bad("#1", "23:17", "round の2つ目の引数", "Int", "Dec"), "Dec"},
		{"#1 to_str $l", bad("#1", "23:15", "to_str の引数", "Int・Dec・Bool のどれか", "Line"), "Str"},
		{"#1 le $b $b", bad("#1", "23:11", "le の1つ目の引数", "Int・Dec・Str のどれか", "Bool"), "Bool"},
		{"#1 gt $i $s", bad("#1", "23:14", "gt の2つ目の引数", "Int", "Str"), "Bool"},
		{"#1 and $i $b", bad("#1", "23:12", "and の1つ目の引数", "Bool", "Int"), "Bool"},
		{"#1 or $b $i", bad("#1", "23:14", "or の2つ目の引数", "Bool", "Int"), "Bool"},
		{"#1 not $i", bad("#1", "23:12", "not の引数", "Bool", "Int"), "Bool"},
		{"#1 concat $i $i", bad("#1", "23:15", "concat の1つ目の引数", "Str かリスト", "Int"), "?"},
		{"#1 concat $s $xs", bad("#1", "23:18", "concat の2つ目の引数", "Str", "[Int]"), "?"},
		{"#1 len $i", bad("#1", "23:12", "len の引数", "Str・リスト・マップのどれか", "Int"), "Int"},
		{"#1 empty $i", bad("#1", "23:14", "empty の引数", "Str・リスト・マップのどれか", "Int"), "Bool"},
		{"#1 sum [$s]", bad("#1", "23:12", "sum の引数", "[Int] か [Dec]", "[Str]"), "?"},
		{"#1 reverse $s", bad("#1", "23:16", "reverse の引数", "リスト", "Str"), "?"},
		{"#1 drop $s 1", bad("#1", "23:13", "drop の1つ目の引数", "リスト", "Str"), "?"},
		{"#1 take $xs $s", bad("#1", "23:17", "take の2つ目の引数", "Int", "Str"), "?"},
		{"#1 range $s", bad("#1", "23:14", "range の引数", "Int", "Str"), "[Int]"},
		{"#1 lookup $i \"a\" 0", bad("#1", "23:15", "lookup の1つ目の引数", "マップ", "Int"), "?"},
		{"#1 lookup $m 1 0", bad("#1", "23:18", "lookup の2つ目の引数", "Str", "Int"), "?"},
		{"#1 lookup $m \"a\" \"b\"", bad("#1", "23:22", "lookup の3つ目の引数", "Int", "Str"), "?"},
		{"#1 has $i \"a\"", bad("#1", "23:12", "has の1つ目の引数", "マップ", "Int"), "Bool"},
		{"#1 has $m 1", bad("#1", "23:15", "has の2つ目の引数", "Str", "Int"), "Bool"},
		{"#1 set $m 1 1", bad("#1", "23:15", "set の2つ目の引数", "Str", "Int"), "?"},
		{"#1 set $m \"a\" \"b\"", bad("#1", "23:19", "set の3つ目の引数", "Int", "Str"), "?"},
		{"#1 remove $m 1", bad("#1", "23:18", "remove の2つ目の引数", "Str", "Int"), "?"},
		{"#1 keys $i", bad("#1", "23:13", "keys の引数", "マップ", "Int"), "?"},
		{"#1 values $l", bad("#1", "23:15", "values の引数", "マップ", "Line"), "?"},
		{"#1 eq $u $l", bad("#1", "23:14", "eq の2つ目の引数", "A | B", "Line"), "Bool"},
		{block("sort_by", "$i", "val 1"), bad("#1", "23:16", "sort_by の引数", "リスト", "Int"), "?"},
		{block("group_by", "$i", "val 1"), bad("#1", "23:17", "group_by の引数", "リスト", "Int"), "?"},
		{block("filter", "$i", "val true"), bad("#1", "23:15", "filter の引数", "リスト", "Int"), "?"},
		{block("any", "$i", "val true"), bad("#1", "23:12", "any の引数", "リスト", "Int"), "Bool"},
		{block("sort_by", "$xs", "add $s 1"), bad("#1.1", "24:16", "add の1つ目の引数", "Int か Dec", "Str"), "?"},
		{block("group_by", "$xs", "to_dec $e"), bad("#1.1", "24:7", "group_by のブロックの値", "Int・Str・Bool のどれか", "Dec"), "?"},
		{"#1 eq ({1 -> 1} : {Nope -> Int}) $m", "エラー BX002  箱 T\n  型 Nope は定義されていません\n  位置: t.bx:23:24\n", "Bool"},
		{"#1 route $b", "エラー BX021  箱 T  ノード #1\n  route に true の分岐がありません\n  位置: t.bx:23:8\n" +
			"\nエラー BX021  箱 T  ノード #1\n  route に false の分岐がありません\n  位置: t.bx:23:8\n", "?"},
		{route, "エラー BX021  箱 T  ノード #1\n  route に false の分岐がありません\n  位置: t.bx:23:8\n", "Int"},
		{mixed, "", "?"},
		{emptyBranch, "", "?"},
		{"#1 val [[], [1]]", "エラー BX024  箱 T  ノード #1\n  空のリストの型が決まりません\n  位置: t.bx:23:13\n" + emptyListHint, "[?]"},
		{"#1 val {\"a\" -> [], \"b\" -> [1]}", "エラー BX024  箱 T  ノード #1\n  空のリストの型が決まりません\n  位置: t.bx:23:20\n" + emptyListHint, "{Str -> ?}"},
		{block("filter", "$xs", "val $e") + "\n    emit $r \"x\"",
			bad("#1.1", "24:7", "filter のブロックの値", "Bool", "Int") + "\n" + bad("", "26:13", "emit $r の値", "Int", "Str"), "[Int]"},
		{"#1 sum $nothing", "エラー BX013  箱 T  ノード #1  $nothing\n  ポート・束縛子 $nothing はありません\n  位置: t.bx:23:12\n", "?"},
		{"#1 div $nothing 1.0", "エラー BX013  箱 T  ノード #1  $nothing\n  ポート・束縛子 $nothing はありません\n  位置: t.bx:23:12\n", "?"},
		{"#1 map $xs as $e {\n    }", "エラー BX021  箱 T  ノード #1\n  map のブロックが空です\n  位置: t.bx:23:8\n", "?"},
		{"#1 val [[1, \"a\"], [2]]", bad("#1", "23:17", "リストの要素", "Int", "Str"), "[[Int]]"},
		{"#1 val {\"a\" -> [1, \"b\"], \"c\" -> [2]}", bad("#1", "23:24", "リストの要素", "Int", "Str"), "{Str -> [Int]}"},
		{"#1 len $xs as $e", "エラー BX021  箱 T  ノード #1\n  len は束縛子を取りません（書かれた数: 1）\n  位置: t.bx:23:8\n", "?"},
		{"#1 with $l []", "エラー BX021  箱 T  ノード #1\n  with の2つ目の引数は {フィールド: 値, …} です\n  位置: t.bx:23:16\n" +
			"\nエラー BX024  箱 T  ノード #1\n  空のリストの型が決まりません\n  位置: t.bx:23:16\n" + emptyListHint, "Line"},
	}
	for _, tt := range tests {
		t.Run(strings.SplitN(tt.body, "\n", 2)[0], func(t *testing.T) {
			f := mustParse(t, "t.bx", typeSrc("    "+tt.body+"\n    emit $r 1\n"))
			c := checkModule(f, stdFiles)
			if got := diagText(t, c.sorted()); got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
			body := f.Decls[len(f.Decls)-1].(*syntax.Box).Body
			n := body.Stmts[0].(*syntax.Node)
			if got := c.nodeTypes[n].String(); got != tt.typ {
				t.Errorf("#1 の型 = %s, want %s", got, tt.typ)
			}
		})
	}
}

const fieldHint = "  ヒント: フィールド名の綴りを確認してください\n"
const emptyListHint = "  ヒント: ([] : [Int]) のように型注釈を付けてください\n"
const emptyMapHint = "  ヒント: ({->} : {Str -> Int}) のように型注釈を付けてください\n"

func TestTypecheckErrors(t *testing.T) {
	stdFiles := stdDecls(t)
	tests := []struct {
		name, src, want string
	}{
		{"未知の演算（誤りのあるノードを使っても、診断を重ねない）",
			typeSrc("    #1 lookpu $m \"a\" 0\n    emit $r #1\n"),
			"エラー BX021  箱 T  ノード #1\n  演算 lookpu はありません\n  位置: t.bx:23:8\n  候補: lookup\n"},
		{"引数の数",
			typeSrc("    #1 add $i\n    emit $r 1\n"),
			"エラー BX021  箱 T  ノード #1\n  add は引数を2つ取ります（書かれた数: 1）\n  位置: t.bx:23:8\n"},
		{"束縛子とブロックの形",
			typeSrc("    #1 map $xs\n    #2 len $xs as $e {\n      #2.1 val 1\n    }\n    #3 filter $xs as $e\n    #4 map $xs as $e {\n    }\n    emit $r 1\n"),
			"エラー BX021  箱 T  ノード #1\n  map は束縛子を1つ取ります（書かれた数: 0）\n  位置: t.bx:23:8\n" +
				"\nエラー BX021  箱 T  ノード #1\n  map はブロックを取ります（書かれていません）\n  位置: t.bx:23:8\n" +
				"\nエラー BX021  箱 T  ノード #2\n  len は束縛子を取りません（書かれた数: 1）\n  位置: t.bx:24:8\n" +
				"\nエラー BX021  箱 T  ノード #2\n  len はブロックを取りません\n  位置: t.bx:24:8\n" +
				"\nエラー BX021  箱 T  ノード #3\n  filter はブロックを取ります（書かれていません）\n  位置: t.bx:27:8\n" +
				"\nエラー BX021  箱 T  ノード #4\n  map のブロックが空です\n  位置: t.bx:28:8\n"},
		{"route と match の分岐の形",
			typeSrc("    #1 route $b {\n      true {\n        #1.1 val 1\n      }\n      true {\n        #1.2 val 2\n      }\n      A $a {\n        #1.3 val 3\n      }\n    }\n" +
				"    #2 match $u {\n      A $a {\n        #2.1 val 1\n      }\n      B $bb {\n        #2.2 val 2\n      }\n      true {\n        #2.3 val 3\n      }\n    }\n    emit $r 1\n"),
			"エラー BX021  箱 T  ノード #1\n  route に false の分岐がありません\n  位置: t.bx:23:8\n" +
				"\nエラー BX021  箱 T  ノード #1\n  route の true の分岐が重なっています\n  位置: t.bx:27:7\n" +
				"\nエラー BX021  箱 T  ノード #1\n  route の分岐は true と false です\n  位置: t.bx:30:7\n" +
				"\nエラー BX021  箱 T  ノード #2\n  match の分岐は「型の名前 $束縛子」です\n  位置: t.bx:41:7\n"},
		{"算術の引数（誤りのある値を使っても、診断を重ねない）",
			typeSrc("    #1 add $i $d\n    #2 add $s $s\n    #3 add #2 1\n    emit $r #3\n"),
			"エラー BX020  箱 T  ノード #1\n  add の2つ目の引数の型が合いません（期待した型 Int、実際の型 Dec）\n  位置: t.bx:23:15\n" +
				"\nエラー BX020  箱 T  ノード #2\n  add の1つ目の引数の型が合いません（期待した型 Int か Dec、実際の型 Str）\n  位置: t.bx:24:12\n"},
		{"emit の値",
			typeSrc("    emit $r \"x\"\n"),
			"エラー BX020  箱 T\n  emit $r の値の型が合いません（期待した型 Int、実際の型 Str）\n  位置: t.bx:23:13\n"},
		{"存在しないフィールド",
			typeSrc("    #1 val $l.prodcut\n    #2 val $i.x\n    #3 call std.text.split {s: $s, sep: \",\"}\n    #4 val #3.part\n    emit $r 1\n"),
			"エラー BX022  箱 T  ノード #1  $l.prodcut\n  レコード型 Line にフィールド prodcut はありません\n  位置: t.bx:23:12\n" + fieldHint + "  候補: product\n" +
				"\nエラー BX022  箱 T  ノード #2  $i.x\n  型 Int にはフィールドがありません\n  位置: t.bx:24:12\n" +
				"\nエラー BX022  箱 T  ノード #4  #3.part\n  レコード型 {parts: [Str]} にフィールド part はありません\n  位置: t.bx:26:12\n" + fieldHint + "  候補: parts\n"},
		{"名前付きレコードの構築",
			typeSrc("    #1 val Lin{product: \"p\", qty: 1}\n    #2 val Line{product: \"p\"}\n    #3 val Line{product: \"p\", qty: 1, qtx: 2}\n    #4 val Line{product: 1, qty: 1}\n    #5 val U{x: 1}\n    emit $r 1\n"),
			"エラー BX002  箱 T  ノード #1\n  型 Lin は定義されていません\n  位置: t.bx:23:12\n  候補: Line\n" +
				"\nエラー BX020  箱 T  ノード #2\n  Line の構築にフィールド qty がありません\n  位置: t.bx:24:12\n" +
				"\nエラー BX022  箱 T  ノード #3\n  レコード型 Line にフィールド qtx はありません\n  位置: t.bx:25:39\n" + fieldHint + "  候補: qty\n" +
				"\nエラー BX020  箱 T  ノード #4\n  フィールド product の型が合いません（期待した型 Str、実際の型 Int）\n  位置: t.bx:26:26\n" +
				"\nエラー BX020  箱 T  ノード #5\n  型 U は名前付きレコード型ではないので、U{…} とは書けません\n  位置: t.bx:27:12\n"},
		{"型の決まらない空リスト・空マップ",
			typeSrc("    #1 val []\n    #2 len {->}\n    #3 concat $xs []\n    emit $r 1\n"),
			"エラー BX024  箱 T  ノード #1\n  空のリストの型が決まりません\n  位置: t.bx:23:12\n" + emptyListHint +
				"\nエラー BX024  箱 T  ノード #2\n  空のマップの型が決まりません\n  位置: t.bx:24:12\n" + emptyMapHint +
				"\nエラー BX024  箱 T  ノード #3\n  空のリストの型が決まりません\n  位置: t.bx:25:19\n" + emptyListHint},
		{"eq の2つの型",
			typeSrc("    #1 eq $i $s\n    emit $r 1\n"),
			"エラー BX020  箱 T  ノード #1\n  eq の2つ目の引数の型が合いません（期待した型 Int、実際の型 Str）\n  位置: t.bx:23:14\n"},
		{"リストの要素とマップのキー",
			typeSrc("    #1 val [1, \"a\"]\n    #2 val {1.5 -> 1}\n    emit $r 1\n"),
			"エラー BX020  箱 T  ノード #1\n  リストの要素の型が合いません（期待した型 Int、実際の型 Str）\n  位置: t.bx:23:16\n" +
				"\nエラー BX020  箱 T  ノード #2\n  マップのキーの型が合いません（期待した型 Int・Str・Bool のどれか、実際の型 Dec）\n  位置: t.bx:24:13\n"},
		{"繰り返しの集まりとブロックの値",
			typeSrc("    #1 map $i as $e {\n      #1.1 val $e\n    }\n    #2 filter $xs as $e {\n      #2.1 val $e\n    }\n    #3 sort_by $xs as $e {\n      #3.1 eq $e 1\n    }\n" +
				"    #4 fold $xs 0 as ($acc, $e) {\n      #4.1 to_str $e\n    }\n    #5 flat_map $xs as $e {\n      #5.1 val $e\n    }\n    emit $r 1\n"),
			"エラー BX020  箱 T  ノード #1\n  map の引数の型が合いません（期待した型 リスト、実際の型 Int）\n  位置: t.bx:23:12\n" +
				"\nエラー BX020  箱 T  ノード #2.1\n  filter のブロックの値の型が合いません（期待した型 Bool、実際の型 Int）\n  位置: t.bx:27:7\n" +
				"\nエラー BX020  箱 T  ノード #3.1\n  sort_by のブロックの値の型が合いません（期待した型 Int・Dec・Str のどれか、実際の型 Bool）\n  位置: t.bx:30:7\n" +
				"\nエラー BX020  箱 T  ノード #4.1\n  fold のブロックの値の型が合いません（期待した型 Int、実際の型 Str）\n  位置: t.bx:33:7\n" +
				"\nエラー BX020  箱 T  ノード #5.1\n  flat_map のブロックの値の型が合いません（期待した型 リスト、実際の型 Int）\n  位置: t.bx:36:7\n"},
		{"route の条件と、分岐の値",
			typeSrc("    #1 route $b {\n      true {\n        #1.1 val 1\n      }\n      false {\n        #1.2 val \"a\"\n      }\n    }\n" +
				"    #2 route $i {\n      true {\n        #2.1 val 1\n      }\n      false {\n        #2.2 val 2\n      }\n    }\n    emit $r 1\n"),
			"エラー BX020  箱 T  ノード #1.2\n  route の分岐の値の型が合いません（期待した型 Int、実際の型 Str）\n  位置: t.bx:28:9\n" +
				"\nエラー BX020  箱 T  ノード #2\n  route の引数の型が合いません（期待した型 Bool、実際の型 Int）\n  位置: t.bx:31:14\n"},
		{"match の値と網羅",
			typeSrc("    #1 match $i {\n      A $a {\n        #1.1 val 1\n      }\n    }\n" +
				"    #2 match $u {\n      A $a {\n        #2.1 val 1\n      }\n      A $a2 {\n        #2.2 val 2\n      }\n      C $c {\n        #2.3 val 3\n      }\n      Line $ln {\n        #2.4 val 4\n      }\n    }\n    emit $r 1\n"),
			"エラー BX020  箱 T  ノード #1\n  match の引数の型が合いません（期待した型 ユニオン型、実際の型 Int）\n  位置: t.bx:23:14\n" +
				"\nエラー BX023  箱 T  ノード #2\n  match の分岐が足りません（B）\n  位置: t.bx:28:8\n" +
				"\nエラー BX023  箱 T  ノード #2\n  タグ A の分岐が重なっています\n  位置: t.bx:32:7\n" +
				"\nエラー BX023  箱 T  ノード #2\n  ユニオン A | B にタグ C はありません\n  位置: t.bx:35:7\n  候補: A, B\n" +
				"\nエラー BX023  箱 T  ノード #2\n  ユニオン A | B にタグ Line はありません\n  位置: t.bx:38:7\n"},
		{"call の箱とポート",
			typeSrc("    #1 call std.text.splt {s: $s, sep: \",\"}\n    #2 call std.text.split {s: $s}\n    #3 call std.text.split {s: $s, sep: \",\", x: 1}\n" +
				"    #4 call std.text.split {s: 1, sep: \",\"}\n    #5 call std.text.split {s: $s, s: $s, sep: \",\"}\n    emit $r 1\n"),
			"エラー BX040  箱 T  ノード #1\n  箱 std.text.splt はありません\n  位置: t.bx:23:13\n  候補: std.text.split\n" +
				"\nエラー BX040  箱 T  ノード #2\n  std.text.split の入力ポート sep が渡されていません\n  位置: t.bx:24:13\n" +
				"\nエラー BX040  箱 T  ノード #3\n  std.text.split に入力ポート x はありません\n  位置: t.bx:25:46\n  候補: s\n" +
				"\nエラー BX020  箱 T  ノード #4\n  std.text.split のポート s の型が合いません（期待した型 Str、実際の型 Int）\n  位置: t.bx:26:32\n" +
				"\nエラー BX040  箱 T  ノード #5\n  ポート s が重なっています\n  位置: t.bx:27:36\n"},
		{"箱の名前の重複",
			"module m\n\nbox B @\"x\" {\n  out $r : Int\n  perm none\n  body {\n    emit $r 1\n  }\n}\n\nbox B @\"y\" {\n  out $r : Int\n  perm none\n  body {\n    emit $r 2\n  }\n}\n",
			"エラー BX042  箱 B\n  箱 B はすでに定義されています\n  位置: t.bx:11:5\n  ヒント: 最初の定義は 3:5 にあります\n"},
		{"with",
			typeSrc("    #1 with $l {qtx: 1}\n    #2 with $l {qty: \"a\"}\n    #3 with $i {a: 1}\n    #4 with $l $l\n    emit $r 1\n"),
			"エラー BX022  箱 T  ノード #1\n  レコード型 Line にフィールド qtx はありません\n  位置: t.bx:23:17\n" + fieldHint + "  候補: qty\n" +
				"\nエラー BX020  箱 T  ノード #2\n  フィールド qty の型が合いません（期待した型 Int、実際の型 Str）\n  位置: t.bx:24:22\n" +
				"\nエラー BX020  箱 T  ノード #3\n  with の1つ目の引数の型が合いません（期待した型 レコード型、実際の型 Int）\n  位置: t.bx:25:13\n" +
				"\nエラー BX021  箱 T  ノード #4\n  with の2つ目の引数は {フィールド: 値, …} です\n  位置: t.bx:26:16\n"},
		{"型注釈",
			typeSrc("    #1 val ([1] : [Str])\n    #2 val (1 : Str)\n    emit $r 1\n"),
			"エラー BX020  箱 T  ノード #1\n  リストの要素の型が合いません（期待した型 Str、実際の型 Int）\n  位置: t.bx:23:14\n" +
				"\nエラー BX020  箱 T  ノード #2\n  型注釈の値の型が合いません（期待した型 Str、実際の型 Int）\n  位置: t.bx:24:13\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diagText(t, Check(mustParse(t, "t.bx", tt.src), stdFiles))
			if got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}
