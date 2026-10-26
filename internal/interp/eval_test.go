package interp

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/amisonnet8/boxundle-mvp/internal/check"
	"github.com/amisonnet8/boxundle-mvp/internal/diag"
	"github.com/amisonnet8/boxundle-mvp/internal/std"
	"github.com/amisonnet8/boxundle-mvp/internal/syntax"
	"github.com/amisonnet8/boxundle-mvp/internal/value"
)

// prelude は、テストのモジュールの型定義。AA は A の別名。
const prelude = "module m\n\ntype A = {x: Int}\n\ntype B = {y: Int}\n\ntype U = A | B\n\ntype AA = A\n\ntype P = {k: Int, i: Int}\n\n"

// load は、src を構文解析・検証し、エラーが無いことを確かめて、箱の表と型の情報を返す。
func load(t *testing.T, src string) (map[string]*syntax.Box, *check.Info) {
	t.Helper()
	f, err := syntax.Parse("t.bx", []byte(src))
	if err != nil {
		t.Fatalf("構文解析: %v\n%s", err, src)
	}
	decls, err := std.Decls()
	if err != nil {
		t.Fatal(err)
	}
	ds, info := check.Analyze(f, decls)
	if diag.HasError(ds) {
		var b strings.Builder
		_ = diag.WriteText(&b, ds)
		t.Fatalf("検証:\n%s\n%s", b.String(), src)
	}
	boxes := map[string]*syntax.Box{}
	for _, d := range f.Decls {
		if b, ok := d.(*syntax.Box); ok {
			boxes[b.Name.Name] = b
		}
	}
	return boxes, info
}

// evalBody は、出力 $r : rt だけを持つ箱 T の本体を body（各行がノード。#1 の値を出す）で評価する。
func evalBody(t *testing.T, rt, body string, call Caller) (value.Value, error) {
	t.Helper()
	src := prelude + "box T @\"t\" {\n  out $r : " + rt + "\n  perm none\n  body {\n" + body + "\n    emit $r #1\n  }\n}\n"
	boxes, info := load(t, src)
	it := &Interp{Info: info, Call: call}
	out, err := it.Eval(boxes["T"], map[string]value.Value{})
	if err != nil {
		return nil, err
	}
	return out["r"], nil
}

// show は、値を .bx のリテラルに近い形で書く。Dec は apd の String（Python の str と同じ形）で、桁の表記も比べる。
func show(v value.Value) string {
	switch v := v.(type) {
	case value.Int:
		return v.V.String()
	case value.Dec:
		return v.V.String()
	case value.Bool:
		return strconv.FormatBool(bool(v))
	case value.Str:
		return strconv.Quote(string(v))
	case value.List:
		parts := make([]string, len(v))
		for i, x := range v {
			parts[i] = show(x)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case value.Map:
		if len(v.Keys) == 0 {
			return "{->}"
		}
		parts := make([]string, len(v.Keys))
		for i := range v.Keys {
			parts[i] = show(v.Keys[i]) + " -> " + show(v.Vals[i])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case value.Record:
		parts := make([]string, len(v.Names))
		for i, name := range v.Names {
			parts[i] = name + ": " + show(v.Fields[i])
		}
		return v.Tag + "{" + strings.Join(parts, ", ") + "}"
	}
	return "?"
}

// opCase は、本体 body の #1 の値が want になる、という1件。
type opCase struct {
	name, rt, body, want string
}

func runCases(t *testing.T, cases []opCase) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evalBody(t, tt.rt, tt.body, nil)
			if err != nil {
				t.Fatalf("誤り: %v", err)
			}
			if s := show(got); s != tt.want {
				t.Errorf("= %s, want %s", s, tt.want)
			}
		})
	}
}

func TestBasicAndInt(t *testing.T) {
	runCases(t, []opCase{
		{"val", "Int", "#1 val 7", "7"},
		{"add", "Int", "#1 add 2 3", "5"},
		{"sub", "Int", "#1 sub 2 5", "-3"},
		{"mul の大きな整数", "Int", "#1 mul 123456789012345678901234567890 1000000000000", "123456789012345678901234567890000000000000"},
		{"idiv 7 2", "Int", "#1 idiv 7 2", "3"},
		{"mod 7 2", "Int", "#1 mod 7 2", "1"},
		{"idiv -7 2", "Int", "#1 idiv -7 2", "-4"},
		{"mod -7 2", "Int", "#1 mod -7 2", "1"},
		{"idiv 7 -2", "Int", "#1 idiv 7 -2", "-4"},
		{"mod 7 -2", "Int", "#1 mod 7 -2", "-1"},
		{"idiv -7 -2", "Int", "#1 idiv -7 -2", "3"},
		{"mod -7 -2", "Int", "#1 mod -7 -2", "-1"},
		{"idiv -6 3", "Int", "#1 idiv -6 3", "-2"},
		{"mod -6 3", "Int", "#1 mod -6 3", "0"},
		{"idiv 6 -3", "Int", "#1 idiv 6 -3", "-2"},
		{"mod 6 -3", "Int", "#1 mod 6 -3", "0"},
		{"neg", "Int", "#1 neg -5", "5"},
		{"neg 0", "Int", "#1 neg 0", "0"},
		{"abs", "Int", "#1 abs -5", "5"},
		{"abs 正", "Int", "#1 abs 5", "5"},
		{"min", "Int", "#1 min 3 2", "2"},
		{"min 1つ目", "Int", "#1 min 2 3", "2"},
		{"max", "Int", "#1 max 3 2", "3"},
		{"max 2つ目", "Int", "#1 max 2 3", "3"},
		{"to_dec", "Dec", "#1 to_dec 12", "12"},
		{"to_dec 丸めない", "Dec", "#1 to_dec 1234567890123456789012345678901234567890", "1234567890123456789012345678901234567890"},
		{"to_str Int", "Str", "#1 to_str -12", `"-12"`},
		{"to_str Bool true", "Str", "#1 to_str true", `"true"`},
		{"to_str Bool false", "Str", "#1 to_str false", `"false"`},
	})
}

// Dec の期待値は、Python の decimal（有効桁28、ROUND_HALF_EVEN）で求めた。
func TestDec(t *testing.T) {
	runCases(t, []opCase{
		{"add", "Dec", "#1 add 0.1 0.2", "0.3"},
		{"add の指数", "Dec", "#1 add 1.50 2.5", "4.00"},
		{"sub", "Dec", "#1 sub 0.5 2.25", "-1.75"},
		{"sub で 0", "Dec", "#1 sub 1.0 1.0", "0.0"},
		{"mul", "Dec", "#1 mul 0.10 0.20", "0.0200"},
		{"div 1/3", "Dec", "#1 div 1.0 3.0", "0.3333333333333333333333333333"},
		{"div 2/3 は切り上がる", "Dec", "#1 div 2.0 3.0", "0.6666666666666666666666666667"},
		{"div 割り切れる", "Dec", "#1 div 1.0 8.0", "0.125"},
		{"div 負", "Dec", "#1 div -7.5 2.5", "-3"},
		{"div 指数が正", "Dec", "#1 div 1000.0 0.001", "1.0000E+6"},
		{"div 割り切れなければ末尾の0も残す", "Dec", "#1 div 1.0 29.0", "0.03448275862068965517241379310"},
		{"div 理想の指数まで0を除く", "Dec", "#1 div 1.20 0.4", "3.0"},
		{"div 理想の指数が正", "Dec", "#2 to_dec 10\n    #1 div #2 0.5", "2E+1"},
		{"div 被除数の指数が大きい", "Dec", "#2 to_dec 6\n    #1 div #2 2.0", "3"},
		{"div 0", "Dec", "#1 div 0.00 2.0", "0.0"},
		{"div 0 の指数が正", "Dec", "#2 to_dec 0\n    #1 div #2 2.0", "0E+1"},
		{"div -0", "Dec", "#1 div -0.0 2.0", "-0"},
		{"丸めは偶数へ（下）", "Dec", "#1 add 1000000000000000000000000000.5 0.0", "1000000000000000000000000000"},
		{"丸めは偶数へ（上）", "Dec", "#1 add 1000000000000000000000000001.5 0.0", "1000000000000000000000000002"},
		{"neg 0 は正", "Dec", "#1 neg 0.00", "0.00"},
		{"neg", "Dec", "#1 neg 1.5", "-1.5"},
		{"neg は丸める", "Dec", "#1 neg 1234567890123456789012345678.9", "-1234567890123456789012345679"},
		{"abs", "Dec", "#1 abs -2.50", "2.50"},
		{"abs は丸める", "Dec", "#1 abs -1234567890123456789012345678.9", "1234567890123456789012345679"},
		{"max 等しい", "Dec", "#1 max 1.0 1.00", "1.0"},
		{"max 等しい（逆）", "Dec", "#1 max 1.00 1.0", "1.0"},
		{"min 等しい", "Dec", "#1 min 1.0 1.00", "1.00"},
		{"min 等しい（逆）", "Dec", "#1 min 1.00 1.0", "1.00"},
		{"max 負で等しい", "Dec", "#1 max -1.0 -1.00", "-1.00"},
		{"min 負で等しい", "Dec", "#1 min -1.0 -1.00", "-1.0"},
		{"min", "Dec", "#1 min 2.5 1.5", "1.5"},
		{"max", "Dec", "#1 max 2.5 1.5", "2.5"},
		{"min 1つ目", "Dec", "#1 min 1.5 2.5", "1.5"},
		{"max 2つ目", "Dec", "#1 max 1.5 2.5", "2.5"},
		{"max は丸める", "Dec", "#1 max 1234567890123456789012345678.9 0.0", "1234567890123456789012345679"},
		{"floor", "Int", "#1 floor 2.7", "2"},
		{"floor 負", "Int", "#1 floor -2.7", "-3"},
		{"floor 負の整数", "Int", "#1 floor -2.0", "-2"},
		{"floor 負の小数", "Int", "#1 floor -0.5", "-1"},
		{"floor 指数が正", "Int", "#2 div 1000.0 0.001\n    #1 floor #2", "1000000"},
		{"floor 丸めない", "Int", "#1 floor 12345678901234567890123456789012.5", "12345678901234567890123456789012"},
		{"round 偶数へ（下）", "Dec", "#1 round 2.5 0", "2"},
		{"round 偶数へ（上）", "Dec", "#1 round 3.5 0", "4"},
		{"round 0.5", "Dec", "#1 round 0.5 0", "0"},
		{"round -0.5", "Dec", "#1 round -0.5 0", "-0"},
		{"round 桁を揃える", "Dec", "#1 round 1.2 3", "1.200"},
		{"round 負", "Dec", "#1 round -2.345 2", "-2.34"},
		{"round 2.675", "Dec", "#1 round 2.675 2", "2.68"},
		{"round 桁数の上限ちょうど", "Dec", "#1 round 0.0 100000", "0E-100000"},
		{"round 有効桁28ちょうど", "Dec", "#1 round 1.5 27", "1.500000000000000000000000000"},
		{"to_str Dec", "Str", "#1 to_str -0.50", `"-0.50"`},
		{"to_str Dec 指数を使わない", "Str", "#2 div 1000.0 0.001\n    #1 to_str #2", `"1000000"`},
		{"sum Dec", "Dec", "#1 sum [1.50, 2.5]", "4.00"},
		{"sum 空の Dec", "Dec", "#1 sum ([] : [Dec])", "0"},
		{"sum 空の Dec は Dec の 0", "Dec", "#2 sum ([] : [Dec])\n    #1 add #2 0.50", "0.50"},
	})
}

func TestCompareAndLogic(t *testing.T) {
	runCases(t, []opCase{
		{"eq Int", "Bool", "#1 eq 1 1", "true"},
		{"eq Int 違う", "Bool", "#1 eq 1 2", "false"},
		{"eq Dec は数で", "Bool", "#1 eq 1.0 1.00", "true"},
		{"eq レコード", "Bool", "#1 eq A{x: 1} A{x: 1}", "true"},
		{"eq ユニオン", "Bool", "#2 val (B{y: 1} : U)\n    #1 eq #2 A{x: 1}", "false"},
		{"eq 無名レコードの順", "Bool", "#1 eq {a: 1, b: \"s\"} {b: \"s\", a: 1}", "true"},
		{"eq リストの順", "Bool", "#1 eq [1, 2] [2, 1]", "false"},
		{"ne", "Bool", "#1 ne \"a\" \"b\"", "true"},
		{"ne 等しい", "Bool", "#1 ne \"a\" \"a\"", "false"},
		{"lt", "Bool", "#1 lt 1 2", "true"},
		{"lt 等しい", "Bool", "#1 lt 2 2", "false"},
		{"le 等しい", "Bool", "#1 le 2 2", "true"},
		{"le 大きい", "Bool", "#1 le 3 2", "false"},
		{"gt Dec", "Bool", "#1 gt 2.0 1.99", "true"},
		{"gt 等しい", "Bool", "#1 gt 1.0 1.00", "false"},
		{"ge 等しい", "Bool", "#1 ge 1.0 1.00", "true"},
		{"ge 小さい", "Bool", "#1 ge 0.9 1.00", "false"},
		{"lt Str", "Bool", "#1 lt \"ab\" \"b\"", "true"},
		{"and", "Bool", "#1 and true false", "false"},
		{"and 両方", "Bool", "#1 and true true", "true"},
		{"or", "Bool", "#1 or false true", "true"},
		{"or 両方 false", "Bool", "#1 or false false", "false"},
		{"not", "Bool", "#1 not false", "true"},
	})
}

func TestStrListMap(t *testing.T) {
	m := "#2 val {\"b\" -> 2, \"a\" -> 1}\n    "
	runCases(t, []opCase{
		{"concat Str", "Str", "#1 concat \"あ\" \"い\"", `"あい"`},
		{"concat リスト", "[Int]", "#1 concat [1] [2, 3]", "[1, 2, 3]"},
		{"len Str はコードポイント", "Int", "#1 len \"あいう\"", "3"},
		{"len リスト", "Int", "#1 len [1, 2]", "2"},
		{"len マップ", "Int", m + "#1 len #2", "2"},
		{"empty Str", "Bool", "#1 empty \"\"", "true"},
		{"empty Str でない", "Bool", "#1 empty \"a\"", "false"},
		{"empty リスト", "Bool", "#1 empty ([] : [Int])", "true"},
		{"empty リストでない", "Bool", "#1 empty [0]", "false"},
		{"empty マップ", "Bool", "#1 empty ({->} : {Str -> Int})", "true"},
		{"empty マップでない", "Bool", m + "#1 empty #2", "false"},
		{"sum", "Int", "#1 sum [1, 2, 3]", "6"},
		{"sum 空", "Int", "#1 sum ([] : [Int])", "0"},
		{"reverse", "[Int]", "#1 reverse [1, 2, 3]", "[3, 2, 1]"},
		{"take", "[Int]", "#1 take [1, 2, 3] 2", "[1, 2]"},
		{"take 0", "[Int]", "#1 take [1, 2, 3] 0", "[]"},
		{"take 負", "[Int]", "#1 take [1, 2, 3] -1", "[]"},
		{"take 長さちょうど", "[Int]", "#1 take [1, 2, 3] 3", "[1, 2, 3]"},
		{"take 長さを超える", "[Int]", "#1 take [1, 2, 3] 4", "[1, 2, 3]"},
		{"take とても大きい", "[Int]", "#1 take [1, 2, 3] 100000000000000000000000000000", "[1, 2, 3]"},
		{"take とても小さい", "[Int]", "#1 take [1, 2, 3] -100000000000000000000000000000", "[]"},
		{"drop", "[Int]", "#1 drop [1, 2, 3] 1", "[2, 3]"},
		{"drop 負", "[Int]", "#1 drop [1, 2, 3] -1", "[1, 2, 3]"},
		{"drop 長さちょうど", "[Int]", "#1 drop [1, 2, 3] 3", "[]"},
		{"drop 長さを超える", "[Int]", "#1 drop [1, 2, 3] 4", "[]"},
		{"drop とても大きい", "[Int]", "#1 drop [1, 2, 3] 100000000000000000000000000000", "[]"},
		{"distinct は最初を残す", "[Int]", "#1 distinct [3, 1, 3, 2, 1]", "[3, 1, 2]"},
		{"distinct Dec は数で", "[Dec]", "#1 distinct [1.0, 2.0, 1.00]", "[1.0, 2.0]"},
		{"distinct レコード", "[A]", "#1 distinct [A{x: 1}, A{x: 2}, A{x: 1}]", "[A{x: 1}, A{x: 2}]"},
		{"range", "[Int]", "#1 range 3", "[0, 1, 2]"},
		{"range 1", "[Int]", "#1 range 1", "[0]"},
		{"range 0", "[Int]", "#1 range 0", "[]"},
		{"range 負", "[Int]", "#1 range -3", "[]"},
		{"range とても小さい", "[Int]", "#1 range -100000000000000000000000000000", "[]"},
		{"range 上限ちょうど", "Int", "#2 range 1000000\n    #1 len #2", "1000000"},
		{"lookup", "Int", m + "#1 lookup #2 \"a\" 0", "1"},
		{"lookup 無い", "Int", m + "#1 lookup #2 \"c\" 9", "9"},
		{"has", "Bool", m + "#1 has #2 \"b\"", "true"},
		{"has 無い", "Bool", m + "#1 has #2 \"c\"", "false"},
		{"set 足す", "{Str -> Int}", m + "#1 set #2 \"c\" 3", `{"a" -> 1, "b" -> 2, "c" -> 3}`},
		{"set 置き換える", "{Str -> Int}", m + "#1 set #2 \"a\" 5", `{"a" -> 5, "b" -> 2}`},
		{"set は元を変えない", "{Str -> Int}", m + "#3 set #2 \"a\" 5\n    #1 val #2", `{"a" -> 1, "b" -> 2}`},
		{"remove", "{Str -> Int}", m + "#1 remove #2 \"a\"", `{"b" -> 2}`},
		{"remove 無い", "{Str -> Int}", m + "#1 remove #2 \"c\"", `{"a" -> 1, "b" -> 2}`},
		{"keys は昇順", "[Str]", m + "#1 keys #2", `["a", "b"]`},
		{"values はキーの昇順", "[Int]", m + "#1 values #2", "[1, 2]"},
		{"keys Bool", "[Bool]", "#2 val {true -> 1, false -> 0}\n    #1 keys #2", "[false, true]"},
		{"keys Int", "[Int]", "#2 val {10 -> 1, -2 -> 0, 3 -> 5}\n    #1 keys #2", "[-2, 3, 10]"},
	})
}

func TestRecord(t *testing.T) {
	runCases(t, []opCase{
		{"名前付きは型の順", "P", "#1 val P{i: 2, k: 1}", "P{k: 1, i: 2}"},
		{"別名のタグは元の型", "A", "#1 val AA{x: 1}", "A{x: 1}"},
		{"フィールド参照", "Int", "#2 val P{k: 1, i: 2}\n    #1 val #2.i", "2"},
		{"入れ子のフィールド参照", "Int", "#2 val {p: P{k: 1, i: 2}}\n    #1 val #2.p.k", "1"},
		{"with", "P", "#2 val P{k: 1, i: 2}\n    #1 with #2 {i: 5}", "P{k: 1, i: 5}"},
		{"with は元を変えない", "P", "#2 val P{k: 1, i: 2}\n    #3 with #2 {i: 5}\n    #1 val #2", "P{k: 1, i: 2}"},
		{"with 無名", "{a: Int, b: Str}", "#2 val {a: 1, b: \"s\"}\n    #1 with #2 {b: \"t\", a: 3}", `{a: 3, b: "t"}`},
	})
}

func TestIterate(t *testing.T) {
	runCases(t, []opCase{
		{"map", "[Int]", "#1 map [1, 2, 3] as $e {\n      #1.1 mul $e 2\n    }", "[2, 4, 6]"},
		{"ブロックの値は最後のノード", "[Int]", "#1 map [1, 2] as $e {\n      #1.1 mul $e 2\n      #1.2 add #1.1 1\n    }", "[3, 5]"},
		{"map 空", "[Int]", "#1 map ([] : [Int]) as $e {\n      #1.1 mul $e 2\n    }", "[]"},
		{"flat_map", "[Int]", "#1 flat_map [1, 2] as $e {\n      #1.1 val [$e, $e]\n    }", "[1, 1, 2, 2]"},
		{"filter", "[Int]", "#1 filter [1, 2, 3, 4] as $e {\n      #1.1 gt $e 2\n    }", "[3, 4]"},
		{"any", "Bool", "#1 any [1, 2] as $e {\n      #1.1 eq $e 2\n    }", "true"},
		{"any 無い", "Bool", "#1 any [1, 2] as $e {\n      #1.1 eq $e 3\n    }", "false"},
		{"any 空", "Bool", "#1 any ([] : [Int]) as $e {\n      #1.1 eq $e 3\n    }", "false"},
		{"all", "Bool", "#1 all [1, 2] as $e {\n      #1.1 gt $e 0\n    }", "true"},
		{"all 1つ違う", "Bool", "#1 all [1, 2] as $e {\n      #1.1 gt $e 1\n    }", "false"},
		{"all 空", "Bool", "#1 all ([] : [Int]) as $e {\n      #1.1 gt $e 1\n    }", "true"},
		{"fold", "[Int]", "#1 fold [1, 2, 3] ([] : [Int]) as ($acc, $e) {\n      #1.1 concat [$e] $acc\n    }", "[3, 2, 1]"},
		{"fold 空は init", "Int", "#1 fold ([] : [Int]) 7 as ($acc, $e) {\n      #1.1 add $acc $e\n    }", "7"},
		{"sort_by は安定", "[P]", "#1 sort_by [P{k: 2, i: 0}, P{k: 1, i: 1}, P{k: 2, i: 2}, P{k: 1, i: 3}] as $e {\n      #1.1 val $e.k\n    }",
			"[P{k: 1, i: 1}, P{k: 1, i: 3}, P{k: 2, i: 0}, P{k: 2, i: 2}]"},
		{"sort_by 降順", "[Int]", "#1 sort_by [2, 3, 1] as $e {\n      #1.1 neg $e\n    }", "[3, 2, 1]"},
		{"sort_by Str", "[Str]", "#1 sort_by [\"b\", \"ab\", \"a\"] as $e {\n      #1.1 val $e\n    }", `["a", "ab", "b"]`},
		{"sort_by Dec は数で、等しければ元の順", "[Dec]", "#1 sort_by [2.0, 1.00, 0.5, 1.0] as $e {\n      #1.1 val $e\n    }", "[0.5, 1.00, 1.0, 2.0]"},
		{"group_by は各グループが元の順", "{Int -> [Int]}", "#1 group_by [3, 1, 4, 1, 5, 9, 2, 6] as $e {\n      #1.1 mod $e 2\n    }",
			"{0 -> [4, 2, 6], 1 -> [3, 1, 1, 5, 9]}"},
		{"group_by Bool", "{Bool -> [Int]}", "#1 group_by [5, 1, 4] as $e {\n      #1.1 gt $e 3\n    }", "{false -> [1], true -> [5, 4]}"},
		{"group_by Str", "{Str -> [P]}", "#1 group_by [P{k: 2, i: 0}, P{k: 1, i: 1}, P{k: 2, i: 2}] as $e {\n      #1.1 to_str $e.k\n    }",
			`{"1" -> [P{k: 1, i: 1}], "2" -> [P{k: 2, i: 0}, P{k: 2, i: 2}]}`},
		{"group_by 空", "{Int -> [Int]}", "#1 group_by ([] : [Int]) as $e {\n      #1.1 val $e\n    }", "{->}"},
		{"ブロックから外のノードを参照", "[Int]", "#2 val 10\n    #1 map [1, 2] as $e {\n      #1.1 add $e #2\n    }", "[11, 12]"},
		{"入れ子の繰り返し", "[[Int]]", "#1 map [1, 2] as $a {\n      #1.1 map [10, 20] as $b {\n        #1.1.1 add $a $b\n      }\n    }", "[[11, 21], [12, 22]]"},
	})
}

func TestBranch(t *testing.T) {
	u := "#2 val (A{x: 5} : U)\n    "
	runCases(t, []opCase{
		{"route true", "Int", "#1 route true {\n      true {\n        #1.1 val 1\n      }\n      false {\n        #1.2 val 2\n      }\n    }", "1"},
		{"route false", "Int", "#1 route false {\n      true {\n        #1.1 val 1\n      }\n      false {\n        #1.2 val 2\n      }\n    }", "2"},
		{"選ばれなかった true の分岐を評価しない", "Int", "#1 route false {\n      true {\n        #1.1 idiv 1 0\n      }\n      false {\n        #1.2 val 2\n      }\n    }", "2"},
		{"選ばれなかった false の分岐を評価しない", "Dec", "#1 route true {\n      true {\n        #1.1 val 1.5\n      }\n      false {\n        #1.2 div 1.0 0.0\n      }\n    }", "1.5"},
		{"match", "Int", u + "#1 match #2 {\n      A $a {\n        #1.1 val $a.x\n      }\n      B $b {\n        #1.2 idiv $b.y 0\n      }\n    }", "5"},
		{"match 選ばれなかった分岐を評価しない", "Int", u + "#1 match #2 {\n      A $a {\n        #1.1 val 1\n      }\n      B $b {\n        #1.2 idiv 1 0\n      }\n    }", "1"},
		{"match B", "Int", "#2 val (B{y: 7} : U)\n    #1 match #2 {\n      A $a {\n        #1.1 idiv $a.x 0\n      }\n      B $b {\n        #1.2 val $b.y\n      }\n    }", "7"},
		{"match の束縛子はレコード", "A", u + "#1 match #2 {\n      A $a {\n        #1.1 val $a\n      }\n      B $b {\n        #1.2 val A{x: $b.y}\n      }\n    }", "A{x: 5}"},
		{"match 別名のタグ", "Int", u + "#1 match #2 {\n      AA $a {\n        #1.1 val $a.x\n      }\n      B $b {\n        #1.2 val 0\n      }\n    }", "5"},
	})
}

// 出口の分岐：分岐の前の emit はどの経路にも数え、選ばれた分岐の emit だけが出力になる。入れ子の出口の分岐も同じ。
func TestEvalExitBranches(t *testing.T) {
	src := prelude + "box T @\"t\" {\n  in $n : Int\n  out $a : Int\n  out $b : Str\n  perm none\n  body {\n" +
		"    #1 gt $n 0\n    emit $a $n\n" +
		"    #2 route #1 {\n      true {\n        #2.1 to_str $n\n        emit $b #2.1\n      }\n" +
		"      false {\n        #2.2 eq $n 0\n        #2.3 route #2.2 {\n          true {\n            emit $b \"zero\"\n          }\n" +
		"          false {\n            #2.3.1 idiv 1 0\n            emit $b \"neg\"\n          }\n        }\n      }\n    }\n  }\n}\n"
	boxes, info := load(t, src)
	it := &Interp{Info: info}
	for _, tt := range []struct {
		n       int64
		a, b    string
		errNode string
	}{
		{n: 5, a: "5", b: `"5"`},
		{n: 0, a: "0", b: `"zero"`},
		{n: -3, errNode: "#2.3.1"},
	} {
		out, err := it.Eval(boxes["T"], map[string]value.Value{"n": value.NewInt(tt.n)})
		if tt.errNode != "" {
			if d, ok := errors.AsType[diag.Diagnostic](err); !ok || d.Code != diag.BXRun001 || d.Node != tt.errNode {
				t.Errorf("n=%d: 誤り = %v, want %s のノード %s", tt.n, err, diag.BXRun001, tt.errNode)
			}
			continue
		}
		if err != nil {
			t.Fatalf("n=%d: 誤り: %v", tt.n, err)
		}
		if a, b := show(out["a"]), show(out["b"]); a != tt.a || b != tt.b || len(out) != 2 {
			t.Errorf("n=%d: 出力 = %v, want a=%s b=%s", tt.n, out, tt.a, tt.b)
		}
	}
}

// call は Interp.Call に呼び出し先と入力を渡し、返ったレコードをノードの値にする。選ばれなかった分岐の call は呼ばない。
func TestEvalCall(t *testing.T) {
	src := prelude + "box T @\"t\" {\n  in $s : Str\n  out $r : Int\n  perm none\n  body {\n" +
		"    #1 call std.text.split {s: $s, sep: \",\"}\n    #2 len #1.parts\n    #3 eq #2 0\n" +
		"    #4 route #3 {\n      true {\n        #4.1 call S {v: 1}\n        #4.2 val #4.1.w\n      }\n      false {\n        #4.3 val #2\n      }\n    }\n" +
		"    emit $r #4\n  }\n}\n\nbox S @\"s\" {\n  in $v : Int\n  out $w : Int\n  perm none\n  body {\n    emit $w $v\n  }\n}\n"
	boxes, info := load(t, src)
	var calls []string
	fail := diag.New(diag.BXRun010, "呼び出し先の誤り")
	it := &Interp{Info: info, Call: func(box string, in map[string]value.Value) (value.Record, error) {
		calls = append(calls, box+" s="+show(in["s"])+" sep="+show(in["sep"])+" len="+strconv.Itoa(len(in)))
		if in["s"] == value.Str("x") {
			return value.Record{}, fail
		}
		return value.Record{Names: []string{"parts"}, Fields: []value.Value{value.List{value.Str("a"), value.Str("b")}}}, nil
	}}
	out, err := it.Eval(boxes["T"], map[string]value.Value{"s": value.Str("a,b")})
	if err != nil {
		t.Fatal(err)
	}
	if got := show(out["r"]); got != "2" {
		t.Errorf("$r = %s, want 2", got)
	}
	if want := []string{`std.text.split s="a,b" sep="," len=2`}; !slices.Equal(calls, want) {
		t.Errorf("呼び出し = %q, want %q", calls, want)
	}
	// 呼び出し先が返した誤りは、そのまま返す
	_, err = it.Eval(boxes["T"], map[string]value.Value{"s": value.Str("x")})
	if d, ok := errors.AsType[diag.Diagnostic](err); !ok || d.Code != fail.Code || d.Message != fail.Message || d.Box != "" || d.Node != "" {
		t.Errorf("誤り = %#v, want %#v", err, fail)
	}
}

// runErrCase は、本体 body の評価が、ノード node で code の誤りになる、という1件。
type runErrCase struct {
	name, rt, body string
	code           diag.Code
	node, msg      string
}

func TestRunErrors(t *testing.T) {
	for _, tt := range []runErrCase{
		{"div 0", "Dec", "#1 div 1.0 0.0", diag.BXRun001, "#1", "div の除数が 0 です"},
		{"div 0/0", "Dec", "#1 div 0.0 0.0", diag.BXRun001, "#1", "div の除数が 0 です"},
		{"idiv 0", "Int", "#1 idiv 1 0", diag.BXRun001, "#1", "idiv の除数が 0 です"},
		{"mod 0", "Int", "#1 mod 1 0", diag.BXRun001, "#1", "mod の除数が 0 です"},
		{"round 負の桁数", "Dec", "#1 round 1.5 -1", diag.BXRun002, "#1", "round の桁数 -1 が負です"},
		{"round 有効桁28を超える", "Dec", "#1 round 12.5 27", diag.BXRun002, "#1", "round の結果が有効桁 28 に収まりません（桁数 27）"},
		{"round とても大きい桁数", "Dec", "#1 round 0.0 100000000000000000000", diag.BXRun002, "#1", "round の結果が有効桁 28 に収まりません（桁数 100000000000000000000）"},
		{"round int32 に収まらない桁数", "Dec", "#1 round 1.234 4294967298", diag.BXRun002, "#1", "round の結果が有効桁 28 に収まりません（桁数 4294967298）"},
		{"range 上限を超える", "[Int]", "#1 range 1000001", diag.BXRun003, "#1", "range の大きさ 1000001 が上限 1000000 を超えています"},
		{"選ばれた分岐の中", "Int", "#1 route true {\n      true {\n        #1.1 idiv 1 0\n      }\n      false {\n        #1.2 val 2\n      }\n    }", diag.BXRun001, "#1.1", "idiv の除数が 0 です"},
		{"any は答えが決まっても全要素を評価する", "Bool", "#1 any [1, 0] as $e {\n      #1.1 idiv 1 $e\n      #1.2 eq #1.1 1\n    }", diag.BXRun001, "#1.1", "idiv の除数が 0 です"},
		{"all は答えが決まっても全要素を評価する", "Bool", "#1 all [1, 2, 0] as $e {\n      #1.1 idiv 1 $e\n      #1.2 eq #1.1 1\n    }", diag.BXRun001, "#1.1", "idiv の除数が 0 です"},
		// 10 を17回2乗すると 10^131072 になり、apd の指数の上限 100000 を超える
		{"Dec の指数が上限を超える", "Dec", "#2 range 17\n    #1 fold #2 10.0 as ($acc, $e) {\n      #1.1 mul $acc $acc\n    }", diag.BXRun003, "#1.1", "mul の結果が Dec の範囲（指数 -100000〜100000）を超えます"},
		{"Dec の指数が下限を超える", "Dec", "#2 range 17\n    #1 fold #2 0.1 as ($acc, $e) {\n      #1.1 mul $acc $acc\n    }", diag.BXRun003, "#1.1", "mul の結果が Dec の範囲（指数 -100000〜100000）を超えます"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := evalBody(t, tt.rt, tt.body, nil)
			d, ok := errors.AsType[diag.Diagnostic](err)
			if !ok {
				t.Fatalf("誤り = %v, want 診断 %s", err, tt.code)
			}
			if d.Code != tt.code || d.Box != "T" || d.Node != tt.node || d.Message != tt.msg {
				t.Errorf("= %s 箱 %s ノード %s %q, want %s 箱 T ノード %s %q", d.Code, d.Box, d.Node, d.Message, tt.code, tt.node, tt.msg)
			}
		})
	}
}
