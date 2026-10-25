package check

import (
	"testing"

	"github.com/amisonnet8/boxundle-mvp/internal/syntax"
)

// テストの given・expect とスタブの出力は、ポートを過不足なく書き、ポートの型に合わせる（BX060）。
func TestCheckTestValues(t *testing.T) {
	src := "module m\n\ntype L = {xs: [Int], n: Int}\n\nbox T @\"x\" {\n  in $i : Int\n  in $l : L\n  out $r : Int\n  out $s : Str\n  perm clock\n\n" +
		"  test \"正しい\" {\n    stub std.clock.now { now: \"2026-10-25T09:00:00Z\" }\n    given {\n      i: 1,\n      l: L{xs: [], n: 2}\n    }\n    expect {\n      r: 1,\n      s: \"a\"\n    }\n  }\n\n" +
		"  test \"誤り\" {\n    stub std.clock.nwo { now: \"x\" }\n    stub std.clock.now { at: 2 }\n    stub std.clock.now { now: 1 }\n" +
		"    given {\n      i: \"1\",\n      l: L{xs: [\"a\"], n: 2},\n      i: 2,\n      j: 3\n    }\n    expect {\n      r: 1\n    }\n  }\n\n" +
		"  test \"スタブの型\" {\n    stub std.clock.now { now: 1 }\n    given {\n      i: 1,\n      l: L{xs: [1], n: 2}\n    }\n    expect {\n      r: true,\n      s: \"a\"\n    }\n  }\n\n" +
		"  body {\n    #1 call std.clock.now {}\n    emit $r $i\n    emit $s #1.now\n  }\n}\n"
	want := "エラー BX040  箱 T\n  箱 std.clock.nwo はありません\n  位置: t.bx:25:10\n  候補: std.clock.now\n" +
		"\nエラー BX060  箱 T\n  テスト「誤り」の std.clock.now のスタブに、出力ポート now がありません\n  位置: t.bx:26:24\n" +
		"\nエラー BX060  箱 T\n  テスト「誤り」の std.clock.now のスタブに、出力ポート at はありません\n  位置: t.bx:26:26\n" +
		"\nエラー BX060  箱 T\n  テスト「誤り」で、箱 std.clock.now のスタブが重なっています\n  位置: t.bx:27:10\n  ヒント: 最初のスタブは 26:10 にあります\n" +
		"\nエラー BX060  箱 T\n  テスト「誤り」の given の i の型が合いません（期待した型 Int、実際の型 Str）\n  位置: t.bx:29:10\n" +
		"\nエラー BX060  箱 T\n  リストの要素の型が合いません（期待した型 Int、実際の型 Str）\n  位置: t.bx:30:17\n" +
		"\nエラー BX060  箱 T\n  テスト「誤り」の given で、ポート i が重なっています\n  位置: t.bx:31:7\n" +
		"\nエラー BX060  箱 T\n  テスト「誤り」の given に、入力ポート j はありません\n  位置: t.bx:32:7\n  候補: i, l\n" +
		"\nエラー BX060  箱 T\n  テスト「誤り」の expect に、出力ポート s がありません\n  位置: t.bx:34:12\n" +
		"\nエラー BX060  箱 T\n  テスト「スタブの型」の std.clock.now のスタブの now の型が合いません（期待した型 Str、実際の型 Int）\n  位置: t.bx:40:31\n" +
		"\nエラー BX060  箱 T\n  テスト「スタブの型」の expect の r の型が合いません（期待した型 Int、実際の型 Bool）\n  位置: t.bx:46:10\n"
	if got := diagText(t, Check(mustParse(t, "t.bx", src), stdDecls(t))); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// ポート名が重なる箱では、重なりだけを知らせる（同じ名前の2つ目のポートの emit や、テストの値の不足を重ねない）。
func TestCheckDuplicatePorts(t *testing.T) {
	src := "module m\n\nbox D @\"x\" {\n  in $a : Int\n  in $a : Int\n  out $r : Int\n  out $r : Int\n  perm none\n\n" +
		"  test \"t\" {\n    given {}\n    expect {\n      r: 1\n    }\n  }\n\n  body {\n    emit $r 1\n  }\n}\n"
	want := "エラー BX014  箱 D  $a\n  ポート $a が重なっています\n  位置: t.bx:5:6\n  ヒント: 最初のポートは 4:6 にあります\n" +
		"\nエラー BX014  箱 D  $r\n  ポート $r が重なっています\n  位置: t.bx:7:7\n  ヒント: 最初のポートは 6:7 にあります\n" +
		"\nエラー BX060  箱 D\n  テスト「t」の given に、入力ポート a がありません\n  位置: t.bx:11:11\n"
	if got := diagText(t, Check(mustParse(t, "t.bx", src), stdDecls(t))); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// 標準の箱の宣言のテストの値も、CheckStd が確かめる。
func TestCheckStdTestValues(t *testing.T) {
	src := "externs std.t\n\nextern box std.t.f @\"x\" {\n  in $s : Str\n  out $n : Int\n  perm none\n\n" +
		"  test \"t\" {\n    given {\n      s: 1\n    }\n    expect {\n      n: 1\n    }\n  }\n\n  impl builtin \"t.f\"\n}\n"
	got := diagText(t, CheckStd([]*syntax.File{mustParse(t, "std.bx", src)}))
	want := "エラー BX060  箱 std.t.f\n  テスト「t」の given の s の型が合いません（期待した型 Str、実際の型 Int）\n  位置: std.bx:10:10\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
