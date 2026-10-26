package value

import (
	"testing"

	"github.com/amisonnet8/boxundle-mvp/internal/types"
)

var (
	lineT = &types.Record{Name: "Line", Fields: []types.Field{
		{Name: "product", Type: types.Str}, {Name: "qty", Type: types.Int}, {Name: "price", Type: types.Int},
	}}
	invoiceT  = &types.Record{Name: "Invoice", Fields: []types.Field{{Name: "customer", Type: types.Str}, {Name: "total", Type: types.Int}}}
	shortageT = &types.Record{Name: "Shortage", Fields: []types.Field{{Name: "products", Type: &types.List{Elem: types.Str}}}}
	resultT   = &types.Union{Members: []*types.Record{invoiceT, shortageT}}
	anonT     = &types.Record{Fields: []types.Field{{Name: "a", Type: types.Int}, {Name: "b", Type: &types.List{Elem: lineT}}}}
	strMapT   = &types.Map{Key: types.Str, Value: types.Int}
	intMapT   = &types.Map{Key: types.Int, Value: types.Str}
	boolMapT  = &types.Map{Key: types.Bool, Value: types.Int}
)

// JSON を読んで書き直すと、language.md §7.1 の出力の形になる。Dec の表記は Python の decimal の format(…, "f") で求めた。
func TestJSONRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		t    types.Type
		in   string
		want string
	}{
		{"大きな整数の精度を落とさない", types.Int, "123456789012345678901234567890", "123456789012345678901234567890"},
		{"負の整数", types.Int, "-5", "-5"},
		{"前後の空白", types.Int, "  7\n", "7"},
		{"Dec の数値の入力。出力は文字列で、末尾の0を残す", types.Dec, "12.50", `"12.50"`},
		{"Dec の文字列の入力。指数を使わずに出す", types.Dec, `"1e3"`, `"1000"`},
		{"小さい Dec も指数を使わない", types.Dec, "1.20e-7", `"0.000000120"`},
		{"負の0", types.Dec, `"-0"`, `"-0"`},
		{"入力の Dec は丸めない", types.Dec, "123456789012345678901234567890.5", `"123456789012345678901234567890.5"`},
		{"Bool", types.Bool, "true", "true"},
		{"Str は <>& をそのまま書き、改行はエスケープする", types.Str, `"<a&b> あ\n"`, `"<a&b> あ\n"`},
		{"リスト", &types.List{Elem: types.Int}, "[3, 1, 2]", "[3,1,2]"},
		{"空のリスト", &types.List{Elem: types.Int}, "[]", "[]"},
		{"Str のキーのマップはキー昇順のオブジェクト", strMapT, `{"b": 2, "a": 1}`, `{"a":1,"b":2}`},
		{"空の Str のキーのマップ", strMapT, "{}", "{}"},
		{"Int のキーのマップは、キー昇順の [キー, 値] の配列", intMapT, `[[10, "x"], [-1, "y"]]`, `[[-1,"y"],[10,"x"]]`},
		{"空の Int のキーのマップ", intMapT, "[]", "[]"},
		{"Bool のキーのマップは false が先", boolMapT, "[[true, 1], [false, 0]]", "[[false,0],[true,1]]"},
		{"マップの値の Dec", &types.Map{Key: types.Str, Value: types.Dec}, `{"x": 1.0}`, `{"x":"1.0"}`},
		{"名前付きレコードは $tag を先頭に、フィールドを型の順に出す", lineT,
			`{"qty": 2, "product": "A", "price": 100}`, `{"$tag":"Line","product":"A","qty":2,"price":100}`},
		{"名前付きレコードの入力に $tag を書いてよい", lineT,
			`{"$tag": "Line", "product": "A", "qty": 2, "price": 100}`, `{"$tag":"Line","product":"A","qty":2,"price":100}`},
		{"ユニオンは $tag で型を選ぶ", resultT, `{"products": ["B"], "$tag": "Shortage"}`, `{"$tag":"Shortage","products":["B"]}`},
		{"ユニオンの別の型", resultT, `{"$tag": "Invoice", "total": 250, "customer": "c1"}`, `{"$tag":"Invoice","customer":"c1","total":250}`},
		{"無名レコードには $tag を出さない", anonT, `{"b": [], "a": 1}`, `{"a":1,"b":[]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := FromJSON(tt.t, []byte(tt.in), "p")
			if err != nil {
				t.Fatal(err)
			}
			if got := string(ToJSON(tt.t, v)); got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

// 読んだ値そのものも確かめる（書き出しが直してしまう誤りを見逃さないため）。
func TestFromJSONValue(t *testing.T) {
	v, err := FromJSON(lineT, []byte(`{"price": 100, "qty": 2, "product": "A"}`), "p")
	if err != nil {
		t.Fatal(err)
	}
	want := Record{Tag: "Line", Names: []string{"product", "qty", "price"}, Fields: []Value{Str("A"), NewInt(2), NewInt(100)}}
	r, ok := v.(Record)
	if !ok || !Equal(r, want) || r.Names[0] != "product" || r.Names[2] != "price" {
		t.Errorf("got %#v, want %#v", v, want)
	}
	m, err := FromJSON(strMapT, []byte(`{"b": 2, "a": 1}`), "p")
	if err != nil {
		t.Fatal(err)
	}
	if want := NewMap([]Value{Str("a"), Str("b")}, []Value{NewInt(1), NewInt(2)}); !Equal(m, want) {
		t.Errorf("got %#v, want %#v", m, want)
	}
	d, err := FromJSON(types.Dec, []byte(`"12.50"`), "p")
	if err != nil {
		t.Fatal(err)
	}
	dv := d.(Dec)
	if got := dv.V.Text('f'); got != "12.50" {
		t.Errorf("Dec = %s, want 12.50", got)
	}
}

func TestFromJSONErrors(t *testing.T) {
	line := `"product": "A", "qty": 1, "price": 1`
	tests := []struct {
		name string
		t    types.Type
		in   string
		want string
	}{
		{"整数に小数点", types.Int, "1.5", "p: 整数が要りますが、1.5 は小数点か指数を含みます"},
		{"整数に指数", types.Int, "1e3", "p: 整数が要りますが、1e3 は小数点か指数を含みます"},
		{"整数に文字列", types.Int, `"1"`, "p: 整数が要りますが、文字列があります"},
		{"null", types.Int, "null", "p: 整数が要りますが、nullがあります"},
		{"Dec に読めない文字列", types.Dec, `"1.5x"`, `p: "1.5x" は小数として読めません`},
		{"Dec の文字列に空白", types.Dec, `" 1"`, `p: " 1" は小数として読めません`},
		{"Dec に NaN", types.Dec, `"NaN"`, `p: "NaN" は小数として読めません`},
		{"Dec に Infinity", types.Dec, `"Infinity"`, `p: "Infinity" は小数として読めません`},
		{"Dec の整数部を省く", types.Dec, `".5"`, `p: ".5" は小数として読めません`},
		{"Dec の + の符号", types.Dec, `"+1"`, `p: "+1" は小数として読めません`},
		{"Dec の文字列の範囲を超える指数", types.Dec, `"1e999999"`, "p: 1e999999 は小数として扱える範囲を超えています"},
		{"Dec に真偽値", types.Dec, "true", "p: 小数（数値か文字列）が要りますが、真偽値があります"},
		{"Bool に数値", types.Bool, "1", "p: 真偽値が要りますが、数値があります"},
		{"Str に配列", types.Str, "[]", "p: 文字列が要りますが、配列があります"},
		{"リストにオブジェクト", &types.List{Elem: types.Int}, "{}", "p: 配列が要りますが、オブジェクトがあります"},
		{"リストの要素", &types.List{Elem: types.Int}, `[1, "x"]`, "p[1]: 整数が要りますが、文字列があります"},
		{"Str のキーのマップに配列", strMapT, "[]", "p: オブジェクトが要りますが、配列があります"},
		{"Str のキーの重なり", strMapT, `{"a": 1, "a": 2}`, `p: "a" が重なっています`},
		{"Str のキーのマップの値", strMapT, `{"a": "x"}`, `p["a"]: 整数が要りますが、文字列があります`},
		{"Int のキーのマップにオブジェクト", intMapT, "{}", "p: [キー, 値] の配列が要りますが、オブジェクトがあります"},
		{"組が1つしかない", intMapT, "[[1]]", "p[0]: [キー, 値] の2つを並べた配列が要ります"},
		{"組が3つある", intMapT, `[[1, "a", "b"]]`, "p[0]: [キー, 値] の2つを並べた配列が要ります"},
		{"組が配列でない", intMapT, "[1]", "p[0]: [キー, 値] の2つを並べた配列が要ります"},
		{"組のキーの型", intMapT, `[["1", "a"]]`, "p[0][0]: 整数が要りますが、文字列があります"},
		{"組の値の型", intMapT, "[[1, 2]]", "p[0][1]: 文字列が要りますが、数値があります"},
		{"組のキーの重なり", intMapT, `[[1, "a"], [2, "b"], [1, "c"]]`, "p[2]: キー 1 が重なっています"},
		{"フィールドが足りない", lineT, `{"product": "A", "qty": 1}`, "p: フィールド price がありません"},
		{"知らないフィールド", lineT, `{` + line + `, "size": "L"}`, "p: レコード型 Line にフィールド size はありません"},
		{"フィールドの重なり", lineT, `{` + line + `, "qty": 2}`, `p: "qty" が重なっています`},
		{"$tag が型名と違う", lineT, `{"$tag": "Invoice", ` + line + `}`, `p.$tag: $tag には "Line" を書きます`},
		{"$tag が文字列でない", lineT, `{"$tag": 1, ` + line + `}`, `p.$tag: $tag には "Line" を書きます`},
		{"無名レコードに $tag", anonT, `{"$tag": "X", "a": 1, "b": []}`, "p: レコード型 {a: Int, b: [Line]} にフィールド $tag はありません"},
		{"フィールドの値の型", lineT, `{"product": 1, "qty": 1, "price": 1}`, "p.product: 文字列が要りますが、数値があります"},
		{"入れ子の位置", anonT, `{"a": 1, "b": [{` + line + `}, {"product": "B", "qty": "2", "price": 1}]}`,
			"p.b[1].qty: 整数が要りますが、文字列があります"},
		{"レコードに配列", lineT, "[]", "p: オブジェクトが要りますが、配列があります"},
		{"ユニオンに $tag が無い", resultT, `{"total": 1, "customer": "c"}`, `p: ユニオン型 Invoice | Shortage の位置には "$tag" が要ります`},
		{"ユニオンの $tag が文字列でない", resultT, `{"$tag": true}`, "p.$tag: $tag には型名の文字列を書きます"},
		{"ユニオンにない $tag", resultT, `{"$tag": "Line"}`, "p.$tag: Line は、ユニオン型 Invoice | Shortage のどの型でもありません"},
		{"ユニオンの選んだ型のフィールド", resultT, `{"$tag": "Shortage", "products": [1]}`, "p.products[0]: 文字列が要りますが、数値があります"},
		{"ユニオンに配列", resultT, "[]", "p: オブジェクトが要りますが、配列があります"},
		{"ユニオンのフィールドの重なり", resultT, `{"$tag": "Shortage", "$tag": "Invoice"}`, `p: "$tag" が重なっています`},
		{"構文の誤り", &types.List{Elem: types.Int}, "[1,]", "p: JSON の構文の誤りです（4 バイト目）"},
		{"途中で終わる", strMapT, `{"a":`, "p: JSON が途中で終わっています"},
		{"空", types.Int, "", "p: JSON が途中で終わっています"},
		{"値のあとに余分なもの", types.Int, "1 2", "p: 値のあとに、余分なものがあります"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := FromJSON(tt.t, []byte(tt.in), "p")
			if err == nil {
				t.Fatalf("誤りになりません: %#v", v)
			}
			if err.Error() != tt.want {
				t.Errorf("got  %s\nwant %s", err, tt.want)
			}
		})
	}
}

func TestErrorWithoutPath(t *testing.T) {
	_, err := FromJSON(types.Int, []byte(`"x"`), "")
	if err == nil || err.Error() != "整数が要りますが、文字列があります" {
		t.Errorf("err = %v", err)
	}
}

func TestToJSONPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("ユニオンにない型のレコードで panic しません")
		}
	}()
	ToJSON(resultT, Record{Tag: "Line"})
}
