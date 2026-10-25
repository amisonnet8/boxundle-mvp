// Package types は、Boxundle の型（language.md §4.1）を表す。
// 型定義の別名は解いたあとの形で持つ。タグを持つのは名前付きレコード型だけ。
package types

import (
	"slices"
	"strings"
)

// Type は型。Basic・*List・*Map・*Record・*Union・Invalid のどれか。
type Type interface {
	String() string
	isType()
}

// Basic は組み込みの型。
type Basic int

const (
	Int Basic = iota + 1
	Dec
	Bool
	Str
)

var basicNames = map[Basic]string{Int: "Int", Dec: "Dec", Bool: "Bool", Str: "Str"}

// BasicByName は、組み込みの型名から型を引く。
func BasicByName(name string) (Basic, bool) {
	for b, n := range basicNames {
		if n == name {
			return b, true
		}
	}
	return 0, false
}

// BasicNames は、組み込みの型名を返す（Int Dec Bool Str の順）。
func BasicNames() []string { return []string{"Int", "Dec", "Bool", "Str"} }

func (b Basic) String() string { return basicNames[b] }

// List は [Elem]。
type List struct{ Elem Type }

func (l *List) String() string { return "[" + l.Elem.String() + "]" }

// Map は {Key -> Value}。
type Map struct{ Key, Value Type }

func (m *Map) String() string { return "{" + m.Key.String() + " -> " + m.Value.String() + "}" }

// Field は、レコード型の1つのフィールド。
type Field struct {
	Name string
	Type Type
}

// Record はレコード型。Name は名前付きレコード型のタグで、無名なら ""。Fields は書かれた順。
type Record struct {
	Name   string
	Fields []Field
}

func (r *Record) String() string {
	if r.Name != "" {
		return r.Name
	}
	parts := make([]string, len(r.Fields))
	for i, f := range r.Fields {
		parts[i] = f.Name + ": " + f.Type.String()
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// Field は、名前 name のフィールドの型を返す。
func (r *Record) Field(name string) (Type, bool) {
	for _, f := range r.Fields {
		if f.Name == name {
			return f.Type, true
		}
	}
	return nil, false
}

// Union は、名前付きレコード型の和。Members は書かれた順で、重なりは無い。
type Union struct{ Members []*Record }

func (u *Union) String() string {
	parts := make([]string, len(u.Members))
	for i, m := range u.Members {
		parts[i] = m.Name
	}
	return strings.Join(parts, " | ")
}

// Has は、タグ name の構成要素があるかを返す。
func (u *Union) Has(name string) bool {
	return slices.ContainsFunc(u.Members, func(m *Record) bool { return m.Name == name })
}

// Invalid は、誤りのある型の代わり。すでに診断を出した誤りから、診断を重ねて出さないために使う。
type Invalid struct{}

func (Invalid) String() string { return "?" }

func (Basic) isType()   {}
func (*List) isType()   {}
func (*Map) isType()    {}
func (*Record) isType() {}
func (*Union) isType()  {}
func (Invalid) isType() {}

// IsMapKey は、t がマップのキーに使える型（Int・Str・Bool）かを返す。
func IsMapKey(t Type) bool {
	return t == Int || t == Str || t == Bool
}

// Equal は、2つの型が同じかを返す。名前付きレコード型はタグで、無名レコード型はフィールドの集まり
// （順序は問わない）で、ユニオンは構成要素のタグの集まりで比べる。
func Equal(a, b Type) bool {
	switch a := a.(type) {
	case Basic:
		return a == b
	case *List:
		b, ok := b.(*List)
		return ok && Equal(a.Elem, b.Elem)
	case *Map:
		b, ok := b.(*Map)
		return ok && Equal(a.Key, b.Key) && Equal(a.Value, b.Value)
	case *Record:
		b, ok := b.(*Record)
		if !ok || a.Name != b.Name {
			return false
		}
		if a.Name != "" {
			return true
		}
		if len(a.Fields) != len(b.Fields) {
			return false
		}
		for _, f := range a.Fields {
			bt, ok := b.Field(f.Name)
			if !ok || !Equal(f.Type, bt) {
				return false
			}
		}
		return true
	case *Union:
		b, ok := b.(*Union)
		if !ok || len(a.Members) != len(b.Members) {
			return false
		}
		for _, m := range a.Members {
			if !b.Has(m.Name) {
				return false
			}
		}
		return true
	case Invalid:
		_, ok := b.(Invalid)
		return ok
	}
	return false
}
