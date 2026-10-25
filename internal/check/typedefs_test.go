package check

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/amisonnet8/boxundle-mvp/internal/diag"
	"github.com/amisonnet8/boxundle-mvp/internal/std"
	"github.com/amisonnet8/boxundle-mvp/internal/syntax"
	"github.com/amisonnet8/boxundle-mvp/internal/types"
)

func mustParse(t *testing.T, name, src string) *syntax.File {
	t.Helper()
	f, err := syntax.Parse(name, []byte(src))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return f
}

func stdDecls(t *testing.T) []*syntax.File {
	t.Helper()
	files, err := std.Decls()
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func diagText(t *testing.T, ds []diag.Diagnostic) string {
	t.Helper()
	var b bytes.Buffer
	if err := diag.WriteText(&b, ds); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// docs の例と、標準の箱の宣言には、誤りが無い。
func TestCheckNoErrors(t *testing.T) {
	stdFiles := stdDecls(t)
	if ds := CheckStd(stdFiles); len(ds) > 0 {
		t.Errorf("標準の箱の宣言:\n%s", diagText(t, ds))
	}
	for _, name := range []string{"order.bx", "checkout.bx"} {
		src, err := os.ReadFile("../../docs/examples/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if ds := Check(mustParse(t, name, string(src)), stdFiles); len(ds) > 0 {
			t.Errorf("%s:\n%s", name, diagText(t, ds))
		}
	}
}

const unionHint = "  ヒント: ユニオンに並べられるのは、type 名前 = {…} で定義したレコード型です\n"
const keyHint = "  ヒント: マップのキーに使えるのは Int・Str・Bool です\n"
const stdHint = "  ヒント: std で始まる名前は、標準の箱のためにあります\n"
const cycleHint = "  ヒント: 再帰する型は定義できません\n"

func TestCheckTypes(t *testing.T) {
	stdFiles := stdDecls(t)
	tests := []struct {
		name, src, want string
	}{
		{"未定義の型（型定義の中）",
			"module m\n\ntype Order = {lines: [Lin]}\n\ntype Line = {qty: Int}\n",
			"エラー BX002\n  型 Lin は定義されていません\n  位置: t.bx:3:23\n  候補: Line\n"},
		{"未定義の型（ポート。型定義は箱の後ろでもよい）",
			"module m\n\nbox B @\"x\" {\n  in $o : Ordr\n  out $r : Int\n  perm none\n  body {\n    emit $r 1\n  }\n}\n\ntype Order = {a: Int}\n",
			"エラー BX002  箱 B\n  型 Ordr は定義されていません\n  位置: t.bx:4:11\n  候補: Order\n"},
		{"組み込みの型名は大文字小文字を区別する",
			"module m\n\ntype T = {a: str}\n",
			"エラー BX002\n  型 str は定義されていません\n  位置: t.bx:3:14\n  候補: Str\n"},
		{"候補は近い順、同じなら名前の順",
			"module m\n\ntype T = {a: Tx}\n\ntype Ta = Int\n\ntype Tb = Int\n",
			"エラー BX002\n  型 Tx は定義されていません\n  位置: t.bx:3:14\n  候補: T, Ta, Tb\n"},
		{"自分自身への循環（リストを通して）",
			"module m\n\ntype T = {a: [T]}\n",
			"エラー BX003\n  型定義が循環しています: T → T\n  位置: t.bx:3:6\n" + cycleHint},
		{"マップと別名を通した循環は1つの診断",
			"module m\n\ntype A = {b: B}\n\ntype B = {Str -> C}\n\ntype C = A\n",
			"エラー BX003\n  型定義が循環しています: A → B → C → A\n  位置: t.bx:3:6\n" + cycleHint},
		{"循環ごとに1つ。循環を参照するだけの型には出さない",
			"module m\n\ntype X = {y: Y}\n\ntype Y = [X]\n\ntype Z = {x: X}\n\ntype P = P\n",
			"エラー BX003\n  型定義が循環しています: X → Y → X\n  位置: t.bx:3:6\n" + cycleHint +
				"\nエラー BX003\n  型定義が循環しています: P → P\n  位置: t.bx:9:6\n" + cycleHint},
		{"循環の経路には、循環に入らない型を含めない",
			"module m\n\ntype A = {b: B, c: C}\n\ntype B = Int\n\ntype C = [A]\n",
			"エラー BX003\n  型定義が循環しています: A → C → A\n  位置: t.bx:3:6\n" + cycleHint},
		{"循環する型を使う所では、診断を重ねない",
			"module m\n\ntype P = P\n\ntype A = {x: Int}\n\ntype U = P | A\n\ntype M = {P -> Int}\n",
			"エラー BX003\n  型定義が循環しています: P → P\n  位置: t.bx:3:6\n" + cycleHint},
		{"ユニオンの構成要素が名前付きレコード型でない",
			"module m\n\ntype A = {x: Int}\n\ntype S = {Str -> Int}\n\ntype U = A | S | [Int] | Int | {x: Int}\n",
			"エラー BX004\n  ユニオンの構成要素 S は名前付きレコード型ではありません\n  位置: t.bx:7:14\n" + unionHint +
				"\nエラー BX004\n  ユニオンの構成要素 [Int] は名前付きレコード型ではありません\n  位置: t.bx:7:18\n" + unionHint +
				"\nエラー BX004\n  ユニオンの構成要素 Int は名前付きレコード型ではありません\n  位置: t.bx:7:26\n" + unionHint +
				"\nエラー BX004\n  ユニオンの構成要素 {x: Int} は名前付きレコード型ではありません\n  位置: t.bx:7:32\n" + unionHint},
		{"名前付きレコード型の別名はユニオンに並べられる",
			"module m\n\ntype A = {x: Int}\n\ntype AA = A\n\ntype B = {y: Int}\n\ntype U = AA | B\n",
			""},
		{"ユニオンの構成要素の重なり（別名を含む）",
			"module m\n\ntype A = {x: Int}\n\ntype AA = A\n\ntype B = {y: Int}\n\ntype U = A | AA\n\ntype V = A | B | A\n",
			"エラー BX006\n  ユニオンの構成要素 AA が重なっています（タグ A）\n  位置: t.bx:9:14\n" +
				"\nエラー BX006\n  ユニオンの構成要素 A が重なっています（タグ A）\n  位置: t.bx:11:18\n"},
		{"型名の重複",
			"module m\n\ntype T = Int\n\ntype T = Str\n",
			"エラー BX006\n  型 T はすでに定義されています\n  位置: t.bx:5:6\n  ヒント: 最初の定義は 3:6 にあります\n"},
		{"組み込みの型名の定義",
			"module m\n\ntype Int = Str\n",
			"エラー BX006\n  組み込みの型名 Int は定義できません\n  位置: t.bx:3:6\n"},
		{"フィールド名の重複（型定義とポートの無名レコード）",
			"module m\n\ntype T = {a: Int, a: Str}\n\nbox B {\n  in $x : {b: Int, b: Int}\n  out $r : Int\n  perm none\n  body {\n    emit $r 1\n  }\n}\n",
			"エラー BX006\n  フィールド a が重なっています\n  位置: t.bx:3:19\n" +
				"\nエラー BX006  箱 B\n  フィールド b が重なっています\n  位置: t.bx:6:20\n"},
		{"マップのキーに使えない型（別名の Str はよい。未定義なら BX002 だけ）",
			"module m\n\ntype A = {x: Int}\n\ntype K = Str\n\ntype M = {Dec -> Int}\n\ntype M2 = {[Str] -> Int}\n\ntype M3 = {A -> Int}\n\ntype M4 = {K -> Int}\n\ntype M5 = {Nope -> Int}\n",
			"エラー BX006\n  マップのキーに Dec は使えません\n  位置: t.bx:7:11\n" + keyHint +
				"\nエラー BX006\n  マップのキーに [Str] は使えません\n  位置: t.bx:9:12\n" + keyHint +
				"\nエラー BX006\n  マップのキーに A は使えません\n  位置: t.bx:11:12\n" + keyHint +
				"\nエラー BX002\n  型 Nope は定義されていません\n  位置: t.bx:15:12\n"},
		{"std で始まる型名・箱名（大文字の Std はよい）",
			"module m\n\ntype stdout = Int\n\ntype std = Int\n\ntype Std = Int\n\nbox std_x {\n  out $r : Int\n  perm none\n  body {\n    emit $r 1\n  }\n}\n",
			"エラー BX005\n  型の名前 stdout は std で始められません\n  位置: t.bx:3:6\n" + stdHint +
				"\nエラー BX005\n  型の名前 std は std で始められません\n  位置: t.bx:5:6\n" + stdHint +
				"\nエラー BX005  箱 std_x\n  箱の名前 std_x は std で始められません\n  位置: t.bx:9:5\n" + stdHint},
		{"利用者のファイルの externs と extern box",
			"externs std.x\n\nextern box std.x.f {\n  out $a : Int\n  perm none\n  impl builtin \"x.f\"\n}\n",
			"エラー BX070\n  標準の箱の宣言（externs）は、利用者のファイルには書けません\n  位置: t.bx:1:9\n  ヒント: ファイルは module 名前 で始めてください\n" +
				"\nエラー BX070  箱 std.x.f\n  extern box は、利用者のファイルには書けません\n  位置: t.bx:3:12\n  ヒント: 箱は box で定義し、body を書いてください\n"},
		{"モジュールの中の extern box",
			"module m\n\nextern box F {\n  out $a : Int\n  perm none\n  impl builtin \"F\"\n}\n",
			"エラー BX070  箱 F\n  extern box は、利用者のファイルには書けません\n  位置: t.bx:3:12\n  ヒント: 箱は box で定義し、body を書いてください\n"},
		{"誤りはすべて返り、位置の順に並ぶ",
			"module m\n\nbox B {\n  in $x : Nope\n  out $r : Int\n  perm none\n  body {\n    emit $r 1\n  }\n}\n\ntype T = {a: Int, a: Int}\n",
			"エラー BX002  箱 B\n  型 Nope は定義されていません\n  位置: t.bx:4:11\n" +
				"\nエラー BX006\n  フィールド a が重なっています\n  位置: t.bx:12:19\n"},
		{"同じ行でも列の順に並ぶ（フィールドの型の誤りを、名前の重なりより先に見つける）",
			"module m\n\ntype T = {a: Int, a: Nope}\n",
			"エラー BX006\n  フィールド a が重なっています\n  位置: t.bx:3:19\n" +
				"\nエラー BX002\n  型 Nope は定義されていません\n  位置: t.bx:3:22\n"},
		{"1つの誤りから診断を重ねない",
			"module m\n\ntype A = {x: Nope}\n\ntype U = Missing | A\n\ntype M = {Missing -> Int}\n",
			"エラー BX002\n  型 Nope は定義されていません\n  位置: t.bx:3:14\n" +
				"\nエラー BX002\n  型 Missing は定義されていません\n  位置: t.bx:5:10\n" +
				"\nエラー BX002\n  型 Missing は定義されていません\n  位置: t.bx:7:11\n"},
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

// describe は、型を中身まで書く（名前付きレコードはフィールドも、ユニオンはそれと分かる形で）。
func describe(t types.Type) string {
	switch t := t.(type) {
	case *types.Record:
		if t.Name == "" {
			return t.String()
		}
		parts := make([]string, len(t.Fields))
		for i, f := range t.Fields {
			parts[i] = f.Name + ": " + f.Type.String()
		}
		return t.Name + "{" + strings.Join(parts, ", ") + "}"
	case *types.Union:
		return "union(" + t.String() + ")"
	case nil:
		return "nil"
	}
	return t.String()
}

// 型定義とポートの型は、別名を解いた形になり、誤りのあるユニオンは Invalid（?）になる。
func TestResolvedTypes(t *testing.T) {
	src := "module m\n\ntype Line = {product: Str, qty: Int}\n\ntype Stock = {Str -> Int}\n\n" +
		"type Invoice = {total: Int}\n\ntype Shortage = {products: [Str]}\n\ntype Result = Invoice | Shortage\n\n" +
		"type Alias = Invoice\n\ntype S = {Str -> Int}\n\ntype U1 = Invoice | Nope\n\ntype U2 = Invoice | S\n\ntype U3 = Invoice | Alias\n\n" +
		"box B {\n  in $lines : [Line]\n  in $stock : Stock\n  out $r : Result\n  out $x : {a: Int, b: [Str]}\n  perm none\n  body {\n    emit $r 1\n  }\n}\n"
	f := mustParse(t, "t.bx", src)
	c := checkModule(f, stdDecls(t))
	wantTypes := map[string]string{
		"Line":     "Line{product: Str, qty: Int}",
		"Stock":    "{Str -> Int}",
		"Invoice":  "Invoice{total: Int}",
		"Shortage": "Shortage{products: [Str]}",
		"Result":   "union(Invoice | Shortage)",
		"Alias":    "Invoice{total: Int}",
		"S":        "{Str -> Int}",
		"U1":       "?",
		"U2":       "?",
		"U3":       "?",
	}
	for name, want := range wantTypes {
		got, ok := c.env.Lookup(name)
		if !ok || describe(got) != want {
			t.Errorf("型 %s = %s, want %s", name, describe(got), want)
		}
	}
	inv, _ := c.env.Lookup("Invoice")
	alias, _ := c.env.Lookup("Alias")
	if alias != inv {
		t.Error("別名 Alias が Invoice と同じ型でない")
	}
	b := f.Decls[len(f.Decls)-1].(*syntax.Box)
	wantPorts := []string{"[Line]", "{Str -> Int}", "union(Invoice | Shortage)", "{a: Int, b: [Str]}"}
	for i, p := range slices.Concat(b.Ins, b.Outs) {
		if got := describe(c.portTypes[p]); got != wantPorts[i] {
			t.Errorf("ポート %s = %s, want %s", p.Name.Name, got, wantPorts[i])
		}
	}
}

// 標準の箱の宣言の型（std. 付き）は、モジュールから名前で引け、候補にも入る。
func TestCheckStdTypes(t *testing.T) {
	stdFiles := []*syntax.File{mustParse(t, "std.bx", "externs std.t\n\ntype std.t.R = {a: Int}\n")}
	if ds := CheckStd(stdFiles); len(ds) > 0 {
		t.Fatalf("標準の箱の宣言:\n%s", diagText(t, ds))
	}
	got := diagText(t, Check(mustParse(t, "t.bx", "module m\n\ntype T = {a: std.t.R, b: std.t.Q}\n"), stdFiles))
	want := "エラー BX002\n  型 std.t.Q は定義されていません\n  位置: t.bx:3:26\n  候補: std.t.R\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// 標準の箱の宣言そのものの誤りも見つける（std で始まる名前の決まりは、宣言には当てはめない）。
func TestCheckStdErrors(t *testing.T) {
	stdFiles := []*syntax.File{mustParse(t, "std.bx", "externs std.t\n\ntype std.t.R = {a: Nope}\n")}
	got := diagText(t, CheckStd(stdFiles))
	want := "エラー BX002\n  型 Nope は定義されていません\n  位置: std.bx:3:20\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// 診断はファイルの名前の順に並ぶ（b.bx の型定義の誤りを、a.bx のポートの誤りより先に見つけても）。
func TestCheckStdOrder(t *testing.T) {
	stdFiles := []*syntax.File{
		mustParse(t, "a.bx", "externs std.a\n\nextern box std.a.f {\n  out $r : Nope\n  perm none\n  impl builtin \"a.f\"\n}\n"),
		mustParse(t, "b.bx", "externs std.b\n\ntype std.b.T = Nope\n"),
	}
	got := diagText(t, CheckStd(stdFiles))
	want := "エラー BX002  箱 std.a.f\n  型 Nope は定義されていません\n  位置: a.bx:4:12\n" +
		"\nエラー BX002\n  型 Nope は定義されていません\n  位置: b.bx:3:16\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
