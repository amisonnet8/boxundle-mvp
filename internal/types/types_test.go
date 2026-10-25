package types

import (
	"slices"
	"testing"
)

func TestEqual(t *testing.T) {
	a := &Record{Name: "A", Fields: []Field{{"x", Int}}}
	a2 := &Record{Name: "A"} // 同じタグなら同じ型（型の表でタグは1つの定義に決まる）
	b := &Record{Name: "B", Fields: []Field{{"x", Int}}}
	anonXY := &Record{Fields: []Field{{"x", Int}, {"y", Str}}}
	anonYX := &Record{Fields: []Field{{"y", Str}, {"x", Int}}}
	anonXYInt := &Record{Fields: []Field{{"x", Int}, {"y", Int}}}
	anonXZ := &Record{Fields: []Field{{"x", Int}, {"z", Str}}}
	anonX := &Record{Fields: []Field{{"x", Int}}}
	tests := []struct {
		name string
		a, b Type
		want bool
	}{
		{"同じ組み込み", Int, Int, true},
		{"違う組み込み", Int, Dec, false},
		{"組み込みとリスト", Int, &List{Int}, false},
		{"リスト", &List{Str}, &List{Str}, true},
		{"要素の違うリスト", &List{Str}, &List{Int}, false},
		{"マップ", &Map{Str, Int}, &Map{Str, Int}, true},
		{"キーの違うマップ", &Map{Str, Int}, &Map{Int, Int}, false},
		{"値の違うマップ", &Map{Str, Int}, &Map{Str, Dec}, false},
		{"マップとリスト", &Map{Str, Int}, &List{Int}, false},
		{"同じタグ", a, a2, true},
		{"違うタグ", a, b, false},
		{"名前付きと無名", a, anonX, false},
		{"無名はフィールドの順を問わない", anonXY, anonYX, true},
		{"無名でフィールドの型が違う", anonXY, anonXYInt, false},
		{"無名でフィールドの名前が違う", anonXY, anonXZ, false},
		{"無名でフィールドの数が違う", anonX, anonXY, false},
		{"無名でフィールドの数が違う（逆）", anonXY, anonX, false},
		{"ユニオンは構成要素の順を問わない", &Union{[]*Record{a, b}}, &Union{[]*Record{b, a}}, true},
		{"構成要素の違うユニオン", &Union{[]*Record{a, b}}, &Union{[]*Record{a, {Name: "C"}}}, false},
		{"構成要素の数の違うユニオン", &Union{[]*Record{a, b}}, &Union{[]*Record{a}}, false},
		{"ユニオンとレコード", &Union{[]*Record{a}}, a, false},
		{"Invalid どうし", Invalid{}, Invalid{}, true},
		{"Invalid とほか", Invalid{}, Int, false},
	}
	for _, tt := range tests {
		if got := Equal(tt.a, tt.b); got != tt.want {
			t.Errorf("%s: Equal(%v, %v) = %v, want %v", tt.name, tt.a, tt.b, got, tt.want)
		}
	}
}

func TestString(t *testing.T) {
	line := &Record{Name: "Line", Fields: []Field{{"qty", Int}}}
	tests := []struct {
		t    Type
		want string
	}{
		{Int, "Int"}, {Dec, "Dec"}, {Bool, "Bool"}, {Str, "Str"},
		{&List{line}, "[Line]"},
		{&Map{Str, Int}, "{Str -> Int}"},
		{&Record{Fields: []Field{{"product", Str}, {"lines", &List{line}}}}, "{product: Str, lines: [Line]}"},
		{&Union{[]*Record{{Name: "Invoice"}, {Name: "Shortage"}}}, "Invoice | Shortage"},
		{Invalid{}, "?"},
	}
	for _, tt := range tests {
		if got := tt.t.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}

func TestIsMapKey(t *testing.T) {
	for _, tt := range []struct {
		t    Type
		want bool
	}{
		{Int, true}, {Str, true}, {Bool, true}, {Dec, false},
		{&List{Int}, false}, {&Record{Name: "A"}, false}, {Invalid{}, false},
	} {
		if got := IsMapKey(tt.t); got != tt.want {
			t.Errorf("IsMapKey(%v) = %v, want %v", tt.t, got, tt.want)
		}
	}
}

func TestRecordFieldAndUnionHas(t *testing.T) {
	r := &Record{Name: "A", Fields: []Field{{"x", Int}, {"y", Str}}}
	if ft, ok := r.Field("y"); !ok || ft != Str {
		t.Errorf("Field(y) = %v, %v", ft, ok)
	}
	if _, ok := r.Field("z"); ok {
		t.Error("Field(z) が見つかった")
	}
	u := &Union{[]*Record{r, {Name: "B"}}}
	if !u.Has("B") || u.Has("C") {
		t.Errorf("Has(B) = %v, Has(C) = %v", u.Has("B"), u.Has("C"))
	}
}

func TestEnv(t *testing.T) {
	named := map[string]Type{"Stock": &Map{Str, Int}, "std.t.R": &Record{Name: "std.t.R"}}
	env := NewEnv(named)
	named["Later"] = Int // 作ったあとの元の表の変更は効かない
	for name, want := range map[string]Type{"Int": Int, "Dec": Dec, "Bool": Bool, "Str": Str, "Stock": named["Stock"], "std.t.R": named["std.t.R"]} {
		if got, ok := env.Lookup(name); !ok || got != want {
			t.Errorf("Lookup(%s) = %v, %v", name, got, ok)
		}
	}
	for _, name := range []string{"Later", "int", ""} {
		if got, ok := env.Lookup(name); ok {
			t.Errorf("Lookup(%q) = %v が見つかった", name, got)
		}
	}
	if got, want := env.Names(), []string{"Stock", "std.t.R"}; !slices.Equal(got, want) {
		t.Errorf("Names() = %q, want %q", got, want)
	}
	if got, want := BasicNames(), []string{"Int", "Dec", "Bool", "Str"}; !slices.Equal(got, want) {
		t.Errorf("BasicNames() = %q, want %q", got, want)
	}
}
