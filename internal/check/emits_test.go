package check

import "testing"

// flowSrc は、出力 $a・$c を持つ箱 E の本体に body を入れたモジュール。body は17行目から始まる。
func flowSrc(body string) string {
	return "module m\n\ntype A = {x: Int}\n\ntype B = {y: Int}\n\ntype U = A | B\n\n" +
		"box E @\"x\" {\n  in $b : Bool\n  in $u : U\n  in $xs : [Int]\n  out $a : Int\n  out $c : Int\n  perm none\n  body {\n" +
		body + "  }\n}\n"
}

const exitHint = "  ヒント: 出口の分岐は、どの分岐も emit で終わる route・match です\n"
const onceHint = "  ヒント: 本体のどの経路でも、出力ポートごとにちょうど1回 emit します\n"

func TestCheckFlows(t *testing.T) {
	stdFiles := stdDecls(t)
	tests := []struct {
		name, body, want string
	}{
		{"出口の分岐の中に出口の分岐、emit は分岐の前にも書ける",
			"    emit $a 1\n    #1 match $u {\n      A $p {\n        #1.1 route $b {\n          true {\n            emit $c 1\n          }\n          false {\n            emit $c 2\n          }\n        }\n      }\n      B $q {\n        emit $c $q.y\n      }\n    }\n",
			""},
		{"値の分岐は、本体の途中に置ける",
			"    #1 route $b {\n      true {\n        #1.1 val 1\n      }\n      false {\n        #1.2 val 2\n      }\n    }\n    emit $a #1\n    emit $c #1\n",
			""},
		{"値の分岐と出口の分岐の混在",
			"    #1 route $b {\n      true {\n        #1.1 val 1\n      }\n      false {\n        emit $a 1\n      }\n    }\n    emit $c 1\n",
			"エラー BX032  箱 E  ノード #1\n  route の分岐に、値で終わる分岐（true）と emit で終わる分岐（false）が混ざっています\n  位置: t.bx:17:8\n" +
				"  ヒント: どの分岐も値で終わるか、どの分岐も emit で終わるように揃えてください\n"},
		{"emit は、map などのブロックと値の分岐の中に書けない（そこにしか emit の無い $a に、足りない emit を重ねない）",
			"    #1 map $xs as $e {\n      #1.1 val $e\n      emit $a $e\n      #1.2 val $e\n    }\n    #2 route $b {\n      true {\n        emit $c 1\n        #2.1 val 1\n      }\n      false {\n        #2.2 val 2\n      }\n    }\n    emit $c #2\n",
			"エラー BX031  箱 E  ノード #1\n  emit は、本体の直下か、出口の分岐のブロックの中にしか書けません\n  位置: t.bx:19:7\n" + exitHint +
				"\nエラー BX031  箱 E  ノード #2\n  emit は、本体の直下か、出口の分岐のブロックの中にしか書けません\n  位置: t.bx:24:9\n" + exitHint},
		{"出口の分岐の後ろには emit も書けない",
			"    #1 route $b {\n      true {\n        emit $a 1\n      }\n      false {\n        emit $a 2\n      }\n    }\n    emit $c 1\n",
			"エラー BX031  箱 E  ノード #1\n  route はどの分岐も emit で終わるので、ブロックの最後に置きます（後ろにノードも emit も書けません）\n  位置: t.bx:17:5\n"},
		{"出口の分岐は、map のブロックの中に置けない",
			"    #1 map $xs as $e {\n      #1.1 route $b {\n        true {\n          emit $a 1\n        }\n        false {\n          emit $a 2\n        }\n      }\n    }\n    emit $a 1\n    emit $c 1\n",
			"エラー BX031  箱 E  ノード #1.1\n  route はどの分岐も emit で終わるので、本体か出口の分岐のブロックの中にしか置けません\n  位置: t.bx:18:7\n"},
		{"分岐ごとの経路で、足りない emit と2回目の emit",
			"    emit $a 1\n    #1 route $b {\n      true {\n        emit $c 1\n      }\n      false {\n        emit $a 2\n      }\n    }\n",
			"エラー BX030  箱 E  ノード #1  $c\n  route の false の分岐で、出力ポート $c が emit されていません\n  位置: t.bx:22:7\n" + onceHint +
				"\nエラー BX030  箱 E  ノード #1  $a\n  出力ポート $a は、この経路ですでに emit されています\n  位置: t.bx:23:9\n  ヒント: 最初の emit は 17:5 にあります\n"},
		{"本体の経路で、足りない emit と2回目の emit",
			"    emit $a 1\n    emit $a 2\n",
			"エラー BX030  箱 E  $c\n  出力ポート $c が emit されていません\n  位置: t.bx:14:7\n" + onceHint +
				"\nエラー BX030  箱 E  $a\n  出力ポート $a は、この経路ですでに emit されています\n  位置: t.bx:18:5\n  ヒント: 最初の emit は 17:5 にあります\n"},
		{"match の分岐の経路",
			"    #1 match $u {\n      A $p {\n        emit $a $p.x\n        emit $c 1\n      }\n      B $q {\n        emit $a $q.y\n      }\n    }\n",
			"エラー BX030  箱 E  ノード #1  $c\n  match の B の分岐で、出力ポート $c が emit されていません\n  位置: t.bx:22:7\n" + onceHint},
		{"入れ子の出口の分岐では、分岐ごとに経路を分ける",
			"    #1 route $b {\n      true {\n        emit $a 1\n        #1.1 route $b {\n          true {\n            emit $c 1\n          }\n          false {\n            emit $a 2\n          }\n        }\n      }\n      false {\n        emit $a 3\n        emit $c 3\n      }\n    }\n",
			"エラー BX030  箱 E  ノード #1.1  $c\n  route の false の分岐で、出力ポート $c が emit されていません\n  位置: t.bx:24:11\n" + onceHint +
				"\nエラー BX030  箱 E  ノード #1.1  $a\n  出力ポート $a は、この経路ですでに emit されています\n  位置: t.bx:25:13\n  ヒント: 最初の emit は 19:9 にあります\n"},
		{"分岐のブロックが空なら、形の誤りだけを知らせる",
			"    #1 route $b {\n      true {\n      }\n      false {\n        emit $a 1\n      }\n    }\n",
			"エラー BX021  箱 E  ノード #1\n  route の true の分岐のブロックが空です\n  位置: t.bx:18:7\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diagText(t, Check(mustParse(t, "t.bx", flowSrc(tt.body)), stdFiles))
			if got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}
