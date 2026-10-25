package types

import (
	"maps"
	"slices"
)

// Env は、型定義の名前から、別名を解いた型を引く表。NewEnv で作ったあとは変わらない。
type Env struct {
	named map[string]Type
}

// NewEnv は、名前から型への表を写して Env を作る。
func NewEnv(named map[string]Type) *Env {
	return &Env{named: maps.Clone(named)}
}

// Lookup は、組み込みの型名か、定義した型名から型を引く。
func (e *Env) Lookup(name string) (Type, bool) {
	if b, ok := BasicByName(name); ok {
		return b, true
	}
	t, ok := e.named[name]
	return t, ok
}

// Names は、定義した型名を名前の順に返す（組み込みの型名は含まない）。
func (e *Env) Names() []string {
	return slices.Sorted(maps.Keys(e.named))
}
