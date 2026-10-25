package check

import (
	"os"
	"strings"
	"testing"

	"github.com/amisonnet8/boxundle-mvp/internal/syntax"
)

const permHint = "  ヒント: 箱の perm には、本体で呼ぶ箱の権限をすべて宣言します\n"
const callCycleHint = "  ヒント: 箱は、自分自身を（ほかの箱を通しても）呼び出せません\n"

func TestCheckCallGraph(t *testing.T) {
	stdFiles := stdDecls(t)
	tests := []struct {
		name, src, want string
	}{
		{"権限の伝播・未知の権限・権限の重なり・使っていない権限・スタブの対象",
			"module m\n\nbox A @\"a\" {\n  out $r : Str\n  perm clock, net, clokc, clock\n\n" +
				"  test \"t\" {\n    stub std.clock.now { now: \"x\" }\n    stub std.random.uuid { id: \"u\" }\n    stub std.text.split { parts: [] }\n    given {}\n    expect {\n      r: \"x\"\n    }\n  }\n\n" +
				"  body {\n    #1 call B {}\n    #2 call std.clock.now {}\n    #3 call std.clock.now {}\n    emit $r #2.now\n  }\n}\n\n" +
				"box B @\"b\" {\n  out $r : Str\n  perm none\n  body {\n    #1 call std.random.uuid {}\n    #2 call std.env.get {name: \"a\", default: \"b\"}\n    #3 call std.random.int {lo: 1, hi: 2}\n    emit $r #1.id\n  }\n}\n",
			"警告 BX073  箱 A\n  宣言しているが使っていない権限 net があります\n  位置: t.bx:5:15\n  ヒント: 本体で呼ぶ箱は、どれもこの権限を使いません\n" +
				"\nエラー BX072  箱 A\n  権限 clokc はありません\n  位置: t.bx:5:20\n  候補: clock\n" +
				"\nエラー BX072  箱 A\n  権限 clock が重なっています\n  位置: t.bx:5:27\n  ヒント: 最初の clock は 5:8 にあります\n" +
				"\nエラー BX061  箱 A\n  テスト「t」のスタブの対象 std.text.split は、箱 A から呼ばれていません\n  位置: t.bx:10:10\n" +
				"  ヒント: スタブは、その箱から（入れ子を含めて）呼ばれる箱にだけ書けます\n" +
				"\nエラー BX071  箱 B  ノード #1\n  std.random.uuid が使う権限 random が、perm にありません\n  位置: t.bx:29:13\n" + permHint +
				"\nエラー BX071  箱 B  ノード #2\n  std.env.get が使う権限 env が、perm にありません\n  位置: t.bx:30:13\n" + permHint},
		{"呼び出しの循環（間接・直接。たどり終えた箱 Y は経路に入れない）",
			"module m\n\nbox P @\"p\" {\n  out $r : Int\n  perm none\n  body {\n    #1 call Q {}\n    emit $r #1.r\n  }\n}\n\n" +
				"box Q @\"q\" {\n  out $r : Int\n  perm none\n  body {\n    #1 call P {}\n    emit $r #1.r\n  }\n}\n\n" +
				"box S @\"s\" {\n  out $r : Int\n  perm none\n  body {\n    #1 map [1] as $e {\n      #1.1 call S {}\n    }\n    emit $r 1\n  }\n}\n" +
				"\nbox X @\"x\" {\n  out $r : Int\n  perm none\n  body {\n    #1 call Y {}\n    #2 call Z {}\n    emit $r 1\n  }\n}\n\n" +
				"box Y @\"y\" {\n  out $r : Int\n  perm none\n  body {\n    emit $r 1\n  }\n}\n\n" +
				"box Z @\"z\" {\n  out $r : Int\n  perm none\n  body {\n    #1 call X {}\n    emit $r 1\n  }\n}\n",
			"エラー BX041  箱 P  ノード #1\n  箱の呼び出しが循環しています: P → Q → P\n  位置: t.bx:7:13\n" + callCycleHint +
				"\nエラー BX041  箱 S  ノード #1.1\n  箱の呼び出しが循環しています: S → S\n  位置: t.bx:26:17\n" + callCycleHint +
				"\nエラー BX041  箱 X  ノード #2\n  箱の呼び出しが循環しています: X → Z → X\n  位置: t.bx:37:13\n" + callCycleHint},
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

// 標準の箱の宣言の権限名も、CheckStd が確かめる。
func TestCheckStdPerms(t *testing.T) {
	src := "externs std.t\n\nextern box std.t.f @\"x\" {\n  out $n : Int\n  perm clokc\n  impl builtin \"t.f\"\n}\n"
	got := diagText(t, CheckStd([]*syntax.File{mustParse(t, "std.bx", src)}))
	want := "エラー BX072  箱 std.t.f\n  権限 clokc はありません\n  位置: std.bx:5:8\n  候補: clock\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// mvp.md M2 の受け入れ基準：checkout.bx から perm clock を消すと BX071 が返る。
func TestCheckoutWithoutClock(t *testing.T) {
	src, err := os.ReadFile("../../docs/examples/checkout.bx")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(src), "  perm clock\n", "  perm none\n", 1)
	if text == string(src) {
		t.Fatal("checkout.bx に perm clock がありません")
	}
	got := diagText(t, Check(mustParse(t, "checkout.bx", text), stdDecls(t)))
	want := "エラー BX071  箱 Checkout  ノード #2\n  std.clock.now が使う権限 clock が、perm にありません\n  位置: checkout.bx:122:13\n" + permHint
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
