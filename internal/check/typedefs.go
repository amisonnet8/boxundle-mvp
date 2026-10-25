package check

import (
	"slices"
	"strconv"
	"strings"

	"github.com/amisonnet8/boxundle-mvp/internal/diag"
	"github.com/amisonnet8/boxundle-mvp/internal/syntax"
	"github.com/amisonnet8/boxundle-mvp/internal/types"
)

// typeDef は、1つの型定義と、解いた型。
type typeDef struct {
	def    *syntax.TypeDef
	cyclic bool       // 循環に含まれる（解くと types.Invalid）
	done   bool       // 解いた
	typ    types.Type // 解いた型
}

// collectTypes は、files の型定義を登録する。module なら、名前の決まり（BX005・BX006）も確かめる。
func (c *checker) collectTypes(files []*syntax.File, module bool) {
	for _, f := range files {
		for _, decl := range f.Decls {
			td, ok := decl.(*syntax.TypeDef)
			if !ok {
				continue
			}
			name := td.Name.Name
			entry := &typeDef{def: td}
			switch {
			case slices.Contains(types.BasicNames(), name):
				_ = c.report(diag.BX006, td.Name.Span, "", "組み込みの型名 %s は定義できません", name)
				c.extra = append(c.extra, entry)
				continue
			case c.defs[name] != nil:
				first := c.defs[name].def.Name.Span.Start
				d := c.report(diag.BX006, td.Name.Span, "", "型 %s はすでに定義されています", name)
				d.Hint = "最初の定義は " + posText(first) + " にあります"
				c.extra = append(c.extra, entry)
				continue
			}
			if module && strings.HasPrefix(name, "std") {
				reportStdName(c, td.Name, "型", "")
			}
			c.defs[name] = entry
			c.order = append(c.order, entry)
		}
	}
}

// boxNames は、モジュールの箱の名前の決まり（BX005）を確かめる。
func (c *checker) boxNames(f *syntax.File) {
	for _, decl := range f.Decls {
		if b, ok := decl.(*syntax.Box); ok && !b.Extern && strings.HasPrefix(b.Name.Name, "std") {
			reportStdName(c, b.Name, "箱", b.Name.Name)
		}
	}
}

func reportStdName(c *checker, name syntax.Ident, what, box string) {
	d := c.report(diag.BX005, name.Span, box, "%sの名前 %s は std で始められません", what, name.Name)
	d.Hint = "std で始まる名前は、標準の箱のためにあります"
}

// typeDefs は、型定義の循環を見つけ（BX003）、すべての型定義を解いて、型の表を作る。
func (c *checker) typeDefs() {
	c.cycles()
	named := map[string]types.Type{}
	for _, td := range c.order {
		named[td.def.Name.Name] = c.resolveDef(td)
	}
	for _, td := range c.extra {
		c.resolveRHS(td.def)
	}
	c.env = types.NewEnv(named)
}

// cycles は、型定義の参照をたどり、循環ごとに BX003 を1つ出して、含まれる型定義に印を付ける。
func (c *checker) cycles() {
	const (
		unvisited = iota
		visiting
		visited
	)
	state := map[*typeDef]int{}
	var stack []*typeDef
	var visit func(td *typeDef)
	visit = func(td *typeDef) {
		state[td] = visiting
		stack = append(stack, td)
		for _, name := range refNames(td.def.Type) {
			next := c.defs[name]
			switch {
			case next == nil:
			case state[next] == visiting:
				i := slices.Index(stack, next)
				c.reportCycle(stack[i:])
			case state[next] == unvisited:
				visit(next)
			}
		}
		stack = stack[:len(stack)-1]
		state[td] = visited
	}
	for _, td := range c.order {
		if state[td] == unvisited {
			visit(td)
		}
	}
}

func (c *checker) reportCycle(cycle []*typeDef) {
	names := make([]string, 0, len(cycle)+1)
	for _, td := range cycle {
		names = append(names, td.def.Name.Name)
		td.cyclic = true
	}
	names = append(names, names[0])
	d := c.report(diag.BX003, cycle[0].def.Name.Span, "", "型定義が循環しています: %s", strings.Join(names, " → "))
	d.Hint = "再帰する型は定義できません"
}

// refNames は、型の式の中で参照している型名を、書かれた順に返す。
func refNames(t syntax.Type) []string {
	switch t := t.(type) {
	case *syntax.TypeName:
		return []string{t.Name}
	case *syntax.ListType:
		return refNames(t.Elem)
	case *syntax.MapType:
		return append(refNames(t.Key), refNames(t.Value)...)
	case *syntax.RecordType:
		var names []string
		for _, f := range t.Fields {
			names = append(names, refNames(f.Type)...)
		}
		return names
	case *syntax.UnionType:
		var names []string
		for _, m := range t.Members {
			names = append(names, refNames(m)...)
		}
		return names
	}
	return nil
}

// resolveDef は、型定義を解く（1度だけ。右辺の誤りの診断も1度だけ出る）。
func (c *checker) resolveDef(td *typeDef) types.Type {
	if !td.done {
		td.done = true
		td.typ = c.resolveRHS(td.def)
		if td.cyclic {
			td.typ = types.Invalid{}
		}
	}
	return td.typ
}

// resolveRHS は、型定義の右辺を解く。右辺がレコード型なら、型定義の名前をタグに持つ。
func (c *checker) resolveRHS(def *syntax.TypeDef) types.Type {
	if rt, ok := def.Type.(*syntax.RecordType); ok {
		return &types.Record{Name: def.Name.Name, Fields: c.fields(rt, "")}
	}
	return c.resolve(def.Type, "")
}

// resolve は、型の式を解く。誤りは診断を出して types.Invalid を返す。box は診断に添える箱の名前。
func (c *checker) resolve(t syntax.Type, box string) types.Type {
	switch t := t.(type) {
	case *syntax.TypeName:
		if b, ok := types.BasicByName(t.Name); ok {
			return b
		}
		if td := c.defs[t.Name]; td != nil {
			if td.cyclic {
				return types.Invalid{} // 循環は BX003 で知らせた
			}
			return c.resolveDef(td)
		}
		d := c.report(diag.BX002, t.Span, box, "型 %s は定義されていません", t.Name)
		d.Suggestions = suggest(t.Name, c.typeNames())
		return types.Invalid{}
	case *syntax.ListType:
		return &types.List{Elem: c.resolve(t.Elem, box)}
	case *syntax.MapType:
		key := c.resolve(t.Key, box)
		if _, bad := key.(types.Invalid); !bad && !types.IsMapKey(key) {
			d := c.report(diag.BX006, t.Key.Pos(), box, "マップのキーに %s は使えません", syntax.TypeText(t.Key))
			d.Hint = "マップのキーに使えるのは Int・Str・Bool です"
			key = types.Invalid{}
		}
		return &types.Map{Key: key, Value: c.resolve(t.Value, box)}
	case *syntax.RecordType:
		return &types.Record{Fields: c.fields(t, box)}
	case *syntax.UnionType:
		return c.union(t, box)
	}
	panic("check: 未知の型の式")
}

// fields は、レコード型のフィールドを解く。重なった名前は BX006 で、2つ目からを除く。
func (c *checker) fields(rt *syntax.RecordType, box string) []types.Field {
	var fs []types.Field
	for _, f := range rt.Fields {
		typ := c.resolve(f.Type, box)
		if slices.ContainsFunc(fs, func(x types.Field) bool { return x.Name == f.Name.Name }) {
			_ = c.report(diag.BX006, f.Name.Span, box, "フィールド %s が重なっています", f.Name.Name)
			continue
		}
		fs = append(fs, types.Field{Name: f.Name.Name, Type: typ})
	}
	return fs
}

// union は、ユニオン型を解く。構成要素は名前付きレコード型（BX004）で、重ならない（BX006）。
// 構成要素に誤りがあれば、ユニオン全体を types.Invalid にする。
func (c *checker) union(ut *syntax.UnionType, box string) types.Type {
	var members []*types.Record
	ok := true
	for _, m := range ut.Members {
		mt := c.resolve(m, box)
		if _, bad := mt.(types.Invalid); bad {
			ok = false
			continue
		}
		rec, isRec := mt.(*types.Record)
		if !isRec || rec.Name == "" {
			d := c.report(diag.BX004, m.Pos(), box, "ユニオンの構成要素 %s は名前付きレコード型ではありません", syntax.TypeText(m))
			d.Hint = "ユニオンに並べられるのは、type 名前 = {…} で定義したレコード型です"
			ok = false
			continue
		}
		if slices.Contains(members, rec) {
			_ = c.report(diag.BX006, m.Pos(), box, "ユニオンの構成要素 %s が重なっています（タグ %s）", syntax.TypeText(m), rec.Name)
			ok = false
			continue
		}
		members = append(members, rec)
	}
	if !ok {
		return types.Invalid{}
	}
	return &types.Union{Members: members}
}

// typeNames は、候補にする型名（組み込みと、登録した型定義）を返す。
func (c *checker) typeNames() []string {
	names := types.BasicNames()
	for _, td := range c.order {
		names = append(names, td.def.Name.Name)
	}
	return names
}

// ports は、files の箱のポートの型を解く。
func (c *checker) ports(files []*syntax.File) {
	for _, f := range files {
		for _, decl := range f.Decls {
			b, ok := decl.(*syntax.Box)
			if !ok {
				continue
			}
			for _, p := range slices.Concat(b.Ins, b.Outs) {
				c.portTypes[p] = c.resolve(p.Type, b.Name.Name)
			}
		}
	}
}

func posText(p diag.Pos) string {
	return strconv.Itoa(p.Line) + ":" + strconv.Itoa(p.Col)
}
