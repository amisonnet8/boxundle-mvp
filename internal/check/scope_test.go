package check

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// scopeSrc は、入力 $count・$items と出力 $result を持つ箱 B の本体に body を入れたモジュール。
// body は9行目から始まる。
func scopeSrc(body string) string {
	return "module m\n\nbox B @\"x\" {\n  in $count : Int\n  in $items : [Int]\n  out $result : Int\n  perm none\n  body {\n" + body + "  }\n}\n"
}

const idHint = "  ヒント: 本体の直下のノードのIDは #数、ブロックの中のノードのIDは、ブロックを持つノードのIDに .数 を1つ足した形です\n"
const refHint = "  ヒント: 参照できるのは、同じブロックか祖先のブロックで、前にあるノードだけです\n"
const shadowHint = "  ヒント: スコープの中で、ポート名と束縛子名は重ねられません\n"
const outHint = "  ヒント: 出力ポートは emit で値を確定させるだけで、値は読めません\n"

func TestCheckScope(t *testing.T) {
	stdFiles := stdDecls(t)
	tests := []struct {
		name, src, want string
	}{
		{"ノードIDの重複",
			scopeSrc("    #1 val 1\n    #1 val 2\n    emit $result #1\n"),
			"エラー BX010  箱 B  ノード #1\n  ノードID #1 が重なっています\n  位置: t.bx:10:5\n  ヒント: 最初のノードは 9:5 にあります\n"},
		{"分岐をまたいでもノードIDは一意",
			scopeSrc("    #1 val true\n    #2 route #1 {\n      true {\n        #2.1 val 1\n        emit $result #2.1\n      }\n      false {\n        #2.1 val 2\n        emit $result #2.1\n      }\n    }\n"),
			"エラー BX010  箱 B  ノード #2.1\n  ノードID #2.1 が重なっています\n  位置: t.bx:16:9\n  ヒント: 最初のノードは 12:9 にあります\n"},
		{"本体の直下に深いID",
			scopeSrc("    #1.1 val 1\n    emit $result #1.1\n"),
			"エラー BX011  箱 B  ノード #1.1\n  ノードID #1.1 は、本体の直下のノードの形（#数）ではありません\n  位置: t.bx:9:5\n" + idHint},
		{"ブロックの中で、接頭辞が違う・深さが違う",
			scopeSrc("    #1 map $items as $e {\n      #2.1 val $e\n      #1.1.1 val $e\n    }\n    emit $result 1\n"),
			"エラー BX011  箱 B  ノード #2.1\n  ノードID #2.1 は、#1 のブロックの中のノードの形（#1.数）ではありません\n  位置: t.bx:10:7\n" + idHint +
				"\nエラー BX011  箱 B  ノード #1.1.1\n  ノードID #1.1.1 は、#1 のブロックの中のノードの形（#1.数）ではありません\n  位置: t.bx:11:7\n" + idHint},
		{"子のIDは、親の書かれたIDに合わせる",
			scopeSrc("    #1.1 map $items as $e {\n      #1.1.1 val $e\n    }\n    emit $result 1\n"),
			"エラー BX011  箱 B  ノード #1.1\n  ノードID #1.1 は、本体の直下のノードの形（#数）ではありません\n  位置: t.bx:9:5\n" + idHint},
		{"未定義のノード",
			scopeSrc("    emit $result #9\n"),
			"エラー BX012  箱 B  #9\n  ノード #9 はありません\n  位置: t.bx:9:18\n"},
		{"前方参照",
			scopeSrc("    #1 val #2\n    #2 val 1\n    emit $result #2\n"),
			"エラー BX012  箱 B  ノード #1  #2\n  ノード #2 は、この参照より後ろにあります\n  位置: t.bx:9:12\n" + refHint},
		{"自分自身と、自分を含むノード",
			scopeSrc("    #1 add #1 1\n    #2 map $items as $e {\n      #2.1 add #2 $e\n    }\n    emit $result 1\n"),
			"エラー BX012  箱 B  ノード #1  #1\n  ノード #1 は、この参照を含むノードなので参照できません\n  位置: t.bx:9:12\n" + refHint +
				"\nエラー BX012  箱 B  ノード #2.1  #2\n  ノード #2 は、この参照を含むノードなので参照できません\n  位置: t.bx:11:16\n" + refHint},
		{"同じ行の中の前方参照",
			scopeSrc("    #1 val #2 #2 val 1\n    emit $result #2\n"),
			"エラー BX012  箱 B  ノード #1  #2\n  ノード #2 は、この参照より後ろにあります\n  位置: t.bx:9:12\n" + refHint},
		{"同じ行の中で、前にあるが見えないノード",
			scopeSrc("    #1 map $items as $e { #1.1 val 1 } #2 val #1.1\n    emit $result 1\n"),
			"エラー BX012  箱 B  ノード #2  #1.1\n  ノード #1.1 は、ここから見えないブロックの中にあります\n  位置: t.bx:9:47\n" + refHint},
		{"match の分岐の束縛子は、その分岐で見え、ポートと重ねられない",
			"module m\n\ntype A = {x: Int}\n\ntype B = {y: Int}\n\ntype U = A | B\n\nbox M @\"x\" {\n  in $u : U\n  out $r : Int\n  perm none\n  body {\n    #1 match $u {\n      A $a {\n        #1.1 val $a.x\n      }\n      B $u {\n        #1.2 val $u.y\n      }\n    }\n    emit $r #1\n  }\n}\n",
			"エラー BX014  箱 M  ノード #1  $u\n  束縛子 $u は、ポート $u と同じ名前です\n  位置: t.bx:18:9\n" + shadowHint},
		{"スコープの外（祖先のブロックの前のノードは見える）",
			scopeSrc("    #1 val 1\n    #2 map $items as $e {\n      #2.1 add $e #1\n    }\n    #3 val #2.1\n    emit $result #3\n"),
			"エラー BX012  箱 B  ノード #3  #2.1\n  ノード #2.1 は、ここから見えないブロックの中にあります\n  位置: t.bx:13:12\n" + refHint},
		{"未定義のポート（候補は入力ポートと束縛子）",
			scopeSrc("    emit $result $cuont.x\n"),
			"エラー BX013  箱 B  $cuont.x\n  ポート・束縛子 $cuont はありません\n  位置: t.bx:9:18\n  候補: count\n"},
		{"束縛子も候補にする",
			scopeSrc("    #1 map $items as $elem {\n      #1.1 val $elme\n    }\n    emit $result 1\n"),
			"エラー BX013  箱 B  ノード #1.1  $elme\n  ポート・束縛子 $elme はありません\n  位置: t.bx:10:16\n  候補: elem\n"},
		{"出力ポートは候補にしない",
			scopeSrc("    #1 val $reslt\n    emit $result 1\n"),
			"エラー BX013  箱 B  ノード #1  $reslt\n  ポート・束縛子 $reslt はありません\n  位置: t.bx:9:12\n"},
		{"束縛子はブロックの外から見えない",
			scopeSrc("    #1 map $items as $e {\n      #1.1 val $e\n    }\n    #2 val $e\n    emit $result 1\n"),
			"エラー BX013  箱 B  ノード #2  $e\n  ポート・束縛子 $e はありません\n  位置: t.bx:12:12\n"},
		{"emit の対象は出力ポート",
			scopeSrc("    emit $count 1\n    emit $reslt 1\n"),
			"エラー BX030  箱 B  $result\n  出力ポート $result が emit されていません\n  位置: t.bx:6:7\n" + onceHint +
				"\nエラー BX013  箱 B  $count\n  出力ポート $count はありません\n  位置: t.bx:9:10\n  ヒント: $count は入力ポートです。emit できるのは出力ポートだけです\n" +
				"\nエラー BX013  箱 B  $reslt\n  出力ポート $reslt はありません\n  位置: t.bx:10:10\n  候補: result\n"},
		{"ポート名の重複",
			"module m\n\nbox B @\"x\" {\n  in $a : Int\n  out $a : Int\n  perm none\n  body {\n    emit $a 1\n  }\n}\n",
			"エラー BX014  箱 B  $a\n  ポート $a が重なっています\n  位置: t.bx:5:7\n  ヒント: 最初のポートは 4:6 にあります\n"},
		{"束縛子のシャドーイング（兄弟のブロックでは同じ名前でよい）",
			scopeSrc("    #1 map $items as $count {\n      #1.1 val 1\n    }\n    #2 map $items as $e {\n      #2.1 map $items as $e {\n        #2.1.1 val $e\n      }\n    }\n    #3 fold $items 0 as ($s, $s) {\n      #3.1 val $s\n    }\n    #4 map $items as $e {\n      #4.1 val $e\n    }\n    emit $result 1\n"),
			"エラー BX014  箱 B  ノード #1  $count\n  束縛子 $count は、ポート $count と同じ名前です\n  位置: t.bx:9:22\n" + shadowHint +
				"\nエラー BX014  箱 B  ノード #2.1  $e\n  束縛子 $e は、外側の束縛子 $e と同じ名前です\n  位置: t.bx:13:26\n" + shadowHint +
				"\nエラー BX014  箱 B  ノード #3  $s\n  束縛子 $s が重なっています\n  位置: t.bx:17:30\n" + shadowHint},
		{"出力ポートの参照",
			scopeSrc("    #1 add $result 1\n    #2 map $items as $e {\n      #2.1 add $e $result\n    }\n    emit $result #1\n"),
			"エラー BX015  箱 B  ノード #1  $result\n  出力ポート $result は参照できません\n  位置: t.bx:9:12\n" + outHint +
				"\nエラー BX015  箱 B  ノード #2.1  $result\n  出力ポート $result は参照できません\n  位置: t.bx:11:19\n" + outHint},
		{"構築・型注釈・call の引数の中の参照と、分岐の中の emit",
			scopeSrc("    #1 call B {count: #9, items: [#8]}\n    #2 val {k: {\"a\" -> (#7 : Int)}}\n    #3 val true\n    #4 route #3 {\n      true {\n        emit $result #6\n      }\n      false {\n        emit $result 1\n      }\n    }\n"),
			"エラー BX012  箱 B  ノード #1  #9\n  ノード #9 はありません\n  位置: t.bx:9:23\n" +
				"\nエラー BX012  箱 B  ノード #1  #8\n  ノード #8 はありません\n  位置: t.bx:9:35\n" +
				"\nエラー BX012  箱 B  ノード #2  #7\n  ノード #7 はありません\n  位置: t.bx:10:25\n" +
				"\nエラー BX012  箱 B  ノード #4  #6\n  ノード #6 はありません\n  位置: t.bx:14:22\n"},
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

// 参照が、見えている定義に結び付く（型検査が使う）。
func TestScopeResolution(t *testing.T) {
	src := scopeSrc("    #1 val $count\n    #2 map $items as $e {\n      #2.1 add $e #1\n    }\n    #3 fold #2 0 as ($acc, $e) {\n      #3.1 add $acc $e\n    }\n    emit $result #3\n")
	c := checkModule(mustParse(t, "t.bx", src), stdDecls(t))
	if len(c.diags) > 0 {
		t.Fatalf("診断:\n%s", diagText(t, c.diags))
	}
	var got []string
	for r, n := range c.nodeRefs {
		got = append(got, fmt.Sprintf("%s %s → ノード %s %s", posText(r.Span.Start), r.ID, n.ID.Name, posText(n.Span.Start)))
	}
	for r, b := range c.dollarRefs {
		var target string
		if b.port != nil {
			target = "ポート $" + b.port.Name.Name
		} else {
			target = "束縛子 $" + b.binder.Name + " " + posText(b.binder.Span.Start)
		}
		got = append(got, fmt.Sprintf("%s $%s → %s", posText(r.Span.Start), r.Name, target))
	}
	slices.SortFunc(got, func(a, b string) int {
		return cmp.Compare(sortKey(a), sortKey(b))
	})
	want := []string{
		"9:12 $count → ポート $count",
		"10:12 $items → ポート $items",
		"11:16 $e → 束縛子 $e 10:22",
		"11:19 #1 → ノード #1 9:5",
		"13:13 #2 → ノード #2 10:5",
		"14:16 $acc → 束縛子 $acc 13:22",
		"14:21 $e → 束縛子 $e 13:28",
		"16:18 #3 → ノード #3 13:5",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// sortKey は「行:列 …」の行と列を、並べられる数にする。
func sortKey(s string) int {
	var line, col int
	if _, err := fmt.Sscanf(s, "%d:%d", &line, &col); err != nil {
		panic(err)
	}
	return line*1000 + col
}
