// Package value は、Boxundle の値（language.md §4.1）を表す。値は不変として扱い、演算は常に新しい値を返す。
package value

import (
	"cmp"
	"fmt"
	"math/big"
	"slices"

	"github.com/cockroachdb/apd/v3"
)

// Value は値。Int・Dec・Bool・Str・List・Map・Record のどれか。
type Value interface{ isValue() }

// Int は任意精度整数。V は書き換えない。
type Int struct{ V *big.Int }

// Dec は10進小数。演算は DecContext で行う。V は書き換えない。
type Dec struct{ V apd.Decimal }

// Bool は真偽値。
type Bool bool

// Str は文字列。常に正しい UTF-8。
type Str string

// List はリスト。
type List []Value

// Map はマップ。Keys は常に昇順で重ならない。Vals は Keys と同じ順。
type Map struct {
	Keys []Value
	Vals []Value
}

// Record はレコード。Tag は名前付きレコードの型名で、無名なら ""。Names はフィールド名（型の順）。
type Record struct {
	Tag    string
	Names  []string
	Fields []Value
}

func (Int) isValue()    {}
func (Dec) isValue()    {}
func (Bool) isValue()   {}
func (Str) isValue()    {}
func (List) isValue()   {}
func (Map) isValue()    {}
func (Record) isValue() {}

// NewInt は、x の Int を作る。
func NewInt(x int64) Int { return Int{V: big.NewInt(x)} }

// DecContext は、Dec の演算の文脈（有効桁28、HALF_EVEN）を返す。呼ぶたびに新しい文脈を返す。
func DecContext() *apd.Context {
	return &apd.Context{
		Precision:   28,
		MaxExponent: apd.MaxExponent,
		MinExponent: apd.MinExponent,
		Traps:       apd.DefaultTraps,
		Rounding:    apd.RoundHalfEven,
	}
}

// Field は、名前 name のフィールドの値を返す。
func (r Record) Field(name string) (Value, bool) {
	if i := slices.Index(r.Names, name); i >= 0 {
		return r.Fields[i], true
	}
	return nil, false
}

// NewMap は、キーと値の組からマップを作る。キーが重なれば、後ろの組を使う。
func NewMap(keys, vals []Value) Map {
	idx := make([]int, len(keys))
	for i := range idx {
		idx[i] = i
	}
	slices.SortStableFunc(idx, func(a, b int) int { return Compare(keys[a], keys[b]) })
	var m Map
	for _, i := range idx {
		if n := len(m.Keys); n > 0 && Compare(m.Keys[n-1], keys[i]) == 0 {
			m.Vals[n-1] = vals[i] // 同じキーの中では、元の順が後ろのものが最後に来る
			continue
		}
		m.Keys = append(m.Keys, keys[i])
		m.Vals = append(m.Vals, vals[i])
	}
	return m
}

// find は、キー k の位置と、あるかを返す。無ければ、入れるべき位置。
func (m Map) find(k Value) (int, bool) {
	return slices.BinarySearchFunc(m.Keys, k, Compare)
}

// Get は、キー k の値を返す。
func (m Map) Get(k Value) (Value, bool) {
	if i, ok := m.find(k); ok {
		return m.Vals[i], true
	}
	return nil, false
}

// Set は、キー k の値を v にした新しいマップを返す。m は変わらない。
func (m Map) Set(k, v Value) Map {
	i, ok := m.find(k)
	if ok {
		vals := slices.Clone(m.Vals)
		vals[i] = v
		return Map{Keys: m.Keys, Vals: vals}
	}
	return Map{Keys: slices.Insert(slices.Clone(m.Keys), i, k), Vals: slices.Insert(slices.Clone(m.Vals), i, v)}
}

// Remove は、キー k を除いた新しいマップを返す。k が無ければ m をそのまま返す。m は変わらない。
func (m Map) Remove(k Value) Map {
	i, ok := m.find(k)
	if !ok {
		return m
	}
	return Map{Keys: slices.Delete(slices.Clone(m.Keys), i, i+1), Vals: slices.Delete(slices.Clone(m.Vals), i, i+1)}
}

// Equal は、2つの値が等しいかを返す。構造的に比べ、Dec は数値として比べる（1.0 と 1.00 は等しい）。
// レコードはタグと、名前ごとのフィールドで比べる。
func Equal(a, b Value) bool {
	switch a := a.(type) {
	case Int:
		b, ok := b.(Int)
		return ok && a.V.Cmp(b.V) == 0
	case Dec:
		b, ok := b.(Dec)
		return ok && a.V.Cmp(&b.V) == 0
	case Bool:
		return a == b
	case Str:
		return a == b
	case List:
		b, ok := b.(List)
		return ok && slices.EqualFunc(a, b, Equal)
	case Map:
		b, ok := b.(Map)
		return ok && slices.EqualFunc(a.Keys, b.Keys, Equal) && slices.EqualFunc(a.Vals, b.Vals, Equal)
	case Record:
		b, ok := b.(Record)
		if !ok || a.Tag != b.Tag || len(a.Names) != len(b.Names) {
			return false
		}
		for i, name := range a.Names {
			bv, ok := b.Field(name)
			if !ok || !Equal(a.Fields[i], bv) {
				return false
			}
		}
		return true
	}
	panic(fmt.Sprintf("value.Equal: 知らない値 %T", a))
}

// Compare は、順序を持つ値（Int・Dec・Str・Bool）を比べ、a < b なら負、等しければ0、a > b なら正を返す。
// Str はコードポイント列の辞書順（正しい UTF-8 のバイト列の順と同じ）、Bool は false < true。
// 型の違う値や、順序を持たない値は、検証が通さないので panic する。
func Compare(a, b Value) int {
	switch a := a.(type) {
	case Int:
		if b, ok := b.(Int); ok {
			return a.V.Cmp(b.V)
		}
	case Dec:
		if b, ok := b.(Dec); ok {
			return a.V.Cmp(&b.V)
		}
	case Str:
		if b, ok := b.(Str); ok {
			return cmp.Compare(a, b)
		}
	case Bool:
		if b, ok := b.(Bool); ok {
			switch {
			case a == b:
				return 0
			case bool(a):
				return 1
			}
			return -1
		}
	}
	panic(fmt.Sprintf("value.Compare: %T と %T は比べられません", a, b))
}
