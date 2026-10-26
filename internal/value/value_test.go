package value

import (
	"math/big"
	"slices"
	"testing"

	"github.com/cockroachdb/apd/v3"
)

func dec(t *testing.T, s string) Dec {
	t.Helper()
	d, _, err := apd.NewFromString(s)
	if err != nil {
		t.Fatal(err)
	}
	return Dec{V: *d}
}

func bigInt(t *testing.T, s string) Int {
	t.Helper()
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		t.Fatal(s)
	}
	return Int{V: v}
}

func rec(tag string, kv ...any) Record {
	r := Record{Tag: tag}
	for i := 0; i < len(kv); i += 2 {
		r.Names = append(r.Names, kv[i].(string))
		r.Fields = append(r.Fields, kv[i+1].(Value))
	}
	return r
}

func TestEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b Value
		want bool
	}{
		{"大きな整数", bigInt(t, "123456789012345678901234567890"), bigInt(t, "123456789012345678901234567890"), true},
		{"違う整数", NewInt(1), NewInt(2), false},
		{"Dec は数値で比べる", dec(t, "1.0"), dec(t, "1.00"), true},
		{"違う Dec", dec(t, "1.0"), dec(t, "1.01"), false},
		{"Int と Dec は等しくない", NewInt(1), dec(t, "1"), false},
		{"Dec と Int は等しくない", dec(t, "1"), NewInt(1), false},
		{"Bool", Bool(true), Bool(true), true},
		{"違う Bool", Bool(true), Bool(false), false},
		{"Str", Str("あ"), Str("あ"), true},
		{"違う Str", Str("a"), Str("b"), false},
		{"Str と Bool", Str("true"), Bool(true), false},
		{"リストは順序込み", List{NewInt(1), NewInt(2)}, List{NewInt(2), NewInt(1)}, false},
		{"同じリスト", List{NewInt(1), dec(t, "2.0")}, List{NewInt(1), dec(t, "2")}, true},
		{"長さの違うリスト", List{NewInt(1)}, List{NewInt(1), NewInt(1)}, false},
		{"リストとマップ", List{}, Map{}, false},
		{"マップはキーと値", NewMap([]Value{Str("a")}, []Value{NewInt(1)}), NewMap([]Value{Str("a")}, []Value{NewInt(1)}), true},
		{"マップの値が違う", NewMap([]Value{Str("a")}, []Value{NewInt(1)}), NewMap([]Value{Str("a")}, []Value{NewInt(2)}), false},
		{"マップのキーが違う", NewMap([]Value{Str("a")}, []Value{NewInt(1)}), NewMap([]Value{Str("b")}, []Value{NewInt(1)}), false},
		{"マップとレコード", Map{}, Record{}, false},
		{"レコードはタグも比べる", rec("A", "x", NewInt(1)), rec("B", "x", NewInt(1)), false},
		{"同じ名前付きレコード", rec("A", "x", NewInt(1), "y", Str("s")), rec("A", "x", NewInt(1), "y", Str("s")), true},
		{"フィールドの値が違う", rec("A", "x", NewInt(1), "y", Str("s")), rec("A", "x", NewInt(1), "y", Str("t")), false},
		{"無名レコードはフィールドの名前で比べる", rec("", "x", NewInt(1), "y", NewInt(2)), rec("", "y", NewInt(2), "x", NewInt(1)), true},
		{"フィールドの名前が違う", rec("", "x", NewInt(1)), rec("", "y", NewInt(1)), false},
		{"フィールドの数が違う", rec("", "x", NewInt(1)), rec("", "x", NewInt(1), "y", NewInt(1)), false},
		{"レコードと Int", rec(""), NewInt(1), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Equal(tt.a, tt.b); got != tt.want {
				t.Errorf("Equal = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name string
		a, b Value
		want int
	}{
		{"Int", NewInt(-3), NewInt(2), -1},
		{"大きな Int", bigInt(t, "100000000000000000000000000001"), bigInt(t, "100000000000000000000000000000"), 1},
		{"等しい Int", NewInt(7), NewInt(7), 0},
		{"Dec は数値で比べる", dec(t, "1.10"), dec(t, "1.1"), 0},
		{"負の Dec", dec(t, "-0.5"), dec(t, "0.25"), -1},
		{"大きい Dec", dec(t, "2"), dec(t, "1.99"), 1},
		{"大文字は小文字より前", Str("Z"), Str("a"), -1},
		{"かなは ASCII より後ろ", Str("あ"), Str("a"), 1},
		{"コードポイントの順（UTF-16 の順なら逆になる）", Str("\xef\xbf\xbf"), Str("😀"), -1}, // U+FFFF と U+1F600
		{"接頭辞は前", Str("ab"), Str("abc"), -1},
		{"等しい Str", Str("x"), Str("x"), 0},
		{"false は true より前", Bool(false), Bool(true), -1},
		{"true は false より後ろ", Bool(true), Bool(false), 1},
		{"等しい Bool", Bool(true), Bool(true), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Compare(tt.a, tt.b); got != tt.want {
				t.Errorf("Compare = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestComparePanics(t *testing.T) {
	for _, pair := range [][2]Value{
		{NewInt(1), dec(t, "1")}, {dec(t, "1"), NewInt(1)}, {Str("a"), Bool(true)}, {Bool(true), Str("a")},
		{List{}, List{}},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("Compare(%T, %T) が panic しません", pair[0], pair[1])
				}
			}()
			Compare(pair[0], pair[1])
		}()
	}
}

func TestEqualPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("知らない値で panic しません")
		}
	}()
	Equal(nil, nil)
}

func keysOf(m Map) []Value { return m.Keys }

func TestNewMap(t *testing.T) {
	m := NewMap(
		[]Value{Str("b"), Str("a"), Str("c"), Str("a"), Str("b")},
		[]Value{NewInt(1), NewInt(2), NewInt(3), NewInt(4), NewInt(5)},
	)
	if want := []Value{Str("a"), Str("b"), Str("c")}; !slices.EqualFunc(keysOf(m), want, Equal) {
		t.Errorf("Keys = %v, want %v", m.Keys, want)
	}
	if want := []Value{NewInt(4), NewInt(5), NewInt(3)}; !slices.EqualFunc(m.Vals, want, Equal) {
		t.Errorf("Vals = %v, want %v（重なったキーは後ろの値）", m.Vals, want)
	}
	if e := NewMap(nil, nil); len(e.Keys) != 0 || len(e.Vals) != 0 {
		t.Errorf("空のマップ = %v", e)
	}
}

func TestMapGetSet(t *testing.T) {
	m := NewMap([]Value{NewInt(10), NewInt(30)}, []Value{Str("x"), Str("z")})
	for _, tt := range []struct {
		key  Value
		want Value
		ok   bool
	}{
		{NewInt(10), Str("x"), true}, {NewInt(30), Str("z"), true},
		{NewInt(5), nil, false}, {NewInt(20), nil, false}, {NewInt(40), nil, false},
	} {
		got, ok := m.Get(tt.key)
		if ok != tt.ok || (ok && !Equal(got, tt.want)) {
			t.Errorf("Get(%v) = %v, %v; want %v, %v", tt.key, got, ok, tt.want, tt.ok)
		}
	}

	added := m.Set(NewInt(20), Str("y"))
	if want := []Value{NewInt(10), NewInt(20), NewInt(30)}; !slices.EqualFunc(added.Keys, want, Equal) {
		t.Errorf("足したあとの Keys = %v, want %v", added.Keys, want)
	}
	if want := []Value{Str("x"), Str("y"), Str("z")}; !slices.EqualFunc(added.Vals, want, Equal) {
		t.Errorf("足したあとの Vals = %v, want %v", added.Vals, want)
	}
	replaced := m.Set(NewInt(30), Str("w"))
	if want := []Value{Str("x"), Str("w")}; !slices.EqualFunc(replaced.Vals, want, Equal) || len(replaced.Keys) != 2 {
		t.Errorf("置き換えたあと = %v, want 値 %v", replaced, want)
	}
	// 元のマップは変わらない
	if want := NewMap([]Value{NewInt(10), NewInt(30)}, []Value{Str("x"), Str("z")}); !Equal(m, want) {
		t.Errorf("Set が元のマップを変えました: %v", m)
	}
}

func TestRecordField(t *testing.T) {
	r := rec("A", "x", NewInt(1), "y", Str("s"))
	if v, ok := r.Field("y"); !ok || !Equal(v, Str("s")) {
		t.Errorf("Field(y) = %v, %v", v, ok)
	}
	if _, ok := r.Field("z"); ok {
		t.Error("Field(z) が見つかりました")
	}
}

// DecContext の期待値は、Python の decimal の既定の文脈（有効桁28、ROUND_HALF_EVEN）で求めた。
func TestDecContext(t *testing.T) {
	ctx := DecContext()
	var q apd.Decimal
	if _, err := ctx.Quo(&q, apd.New(1, 0), apd.New(3, 0)); err != nil {
		t.Fatal(err)
	}
	if got, want := q.Text('f'), "0.3333333333333333333333333333"; got != want {
		t.Errorf("1/3 = %s, want %s", got, want)
	}
	for in, want := range map[string]string{
		"1.0000000000000000000000000005": "1.000000000000000000000000000", // ちょうど半分で、前の桁が偶数なら切り捨てる
		"1.0000000000000000000000000015": "1.000000000000000000000000002", // 前の桁が奇数なら切り上げる
	} {
		var s apd.Decimal
		d := dec(t, in)
		if _, err := ctx.Add(&s, &d.V, apd.New(0, 0)); err != nil {
			t.Fatal(err)
		}
		if got := s.Text('f'); got != want {
			t.Errorf("%s + 0 = %s, want %s", in, got, want)
		}
	}
	if DecContext() == ctx {
		t.Error("DecContext が同じ文脈を返しました（呼び出し側が書き換えると、ほかに効く）")
	}
}
