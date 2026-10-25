package check

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/amisonnet8/boxundle-mvp/internal/diag"
	"github.com/amisonnet8/boxundle-mvp/internal/syntax"
	"github.com/amisonnet8/boxundle-mvp/internal/types"
)

// opShape は、演算の形。call は別に扱う。
type opShape struct {
	args, binders int
	block         bool // ブロックを取る（route・match の分岐は別）
}

// opShapes は、language.md §4.3 の演算の形の表。初期化のあとは変わらない。
var opShapes = map[string]opShape{
	"val": {args: 1},
	"add": {args: 2}, "sub": {args: 2}, "mul": {args: 2}, "div": {args: 2}, "idiv": {args: 2}, "mod": {args: 2},
	"neg": {args: 1}, "abs": {args: 1}, "min": {args: 2}, "max": {args: 2},
	"to_dec": {args: 1}, "floor": {args: 1}, "round": {args: 2}, "to_str": {args: 1},
	"eq": {args: 2}, "ne": {args: 2}, "lt": {args: 2}, "le": {args: 2}, "gt": {args: 2}, "ge": {args: 2},
	"and": {args: 2}, "or": {args: 2}, "not": {args: 1},
	"concat": {args: 2}, "len": {args: 1}, "empty": {args: 1}, "sum": {args: 1}, "reverse": {args: 1},
	"take": {args: 2}, "drop": {args: 2}, "distinct": {args: 1}, "range": {args: 1},
	"lookup": {args: 3}, "has": {args: 2}, "set": {args: 3}, "remove": {args: 2}, "keys": {args: 1}, "values": {args: 1},
	"with": {args: 2},
	"map":  {args: 1, binders: 1, block: true}, "flat_map": {args: 1, binders: 1, block: true},
	"filter": {args: 1, binders: 1, block: true}, "any": {args: 1, binders: 1, block: true},
	"all": {args: 1, binders: 1, block: true}, "sort_by": {args: 1, binders: 1, block: true},
	"group_by": {args: 1, binders: 1, block: true},
	"fold":     {args: 2, binders: 2, block: true},
	"route":    {args: 1}, "match": {args: 1},
}

// opNames は、候補にする演算名（call を含む）。
func opNames() []string {
	names := append(slices.Collect(maps.Keys(opShapes)), "call")
	slices.Sort(names)
	return names
}

// collectBoxes は、呼び出せる箱（標準の箱と、モジュールの箱）を登録する。箱の名前の重複は BX042（最初の箱を使う）。
// モジュールの箱の名前は std で始まらない（BX005）ので、標準の箱とは重ならない。
func (c *checker) collectBoxes(std []*syntax.File, f *syntax.File) {
	for _, file := range append(slices.Clip(std), f) {
		for _, decl := range file.Decls {
			b, ok := decl.(*syntax.Box)
			if !ok {
				continue
			}
			if first := c.boxes[b.Name.Name]; first != nil {
				d := c.report(diag.BX042, b.Name.Span, b.Name.Name, "箱 %s はすでに定義されています", b.Name.Name)
				d.Hint = "最初の定義は " + posText(first.Name.Span.Start) + " にあります"
				continue
			}
			c.boxes[b.Name.Name] = b
			c.boxList = append(c.boxList, b.Name.Name)
		}
	}
}

// typer は、1つの箱の本体の型検査の状態。
type typer struct {
	c      *checker
	box    *syntax.Box
	owners []*syntax.Node // 今いるノード（外から順）
}

// typecheck は、モジュールの箱ごとに、本体のノードの型を決めて確かめる。
func (c *checker) typecheck(f *syntax.File) {
	for _, decl := range f.Decls {
		if b, ok := decl.(*syntax.Box); ok && b.Body != nil {
			t := &typer{c: c, box: b}
			t.block(b.Body, nil, "")
		}
	}
}

func (t *typer) report(code diag.Code, span diag.Span, format string, args ...any) *diag.Diagnostic {
	d := t.c.report(code, span, t.box.Name.Name, format, args...)
	if len(t.owners) > 0 {
		d.Node = t.owners[len(t.owners)-1].ID.Name
	}
	return d
}

// mismatch は BX020 を出す。what は「add の1つ目の引数」のような、型が合わないものの名前。
func (t *typer) mismatch(span diag.Span, what, want, got string) {
	_ = t.report(diag.BX020, span, "%sの型が合いません（期待した型 %s、実際の型 %s）", what, want, got)
}

// invalid は、型が誤りを含むか（すでに診断を出した誤りから、診断を重ねないため）。
func invalid(typ types.Type) bool {
	switch typ := typ.(type) {
	case types.Invalid:
		return true
	case *types.List:
		return invalid(typ.Elem)
	case *types.Map:
		return invalid(typ.Key) || invalid(typ.Value)
	case *types.Record:
		return slices.ContainsFunc(typ.Fields, func(f types.Field) bool { return invalid(f.Type) })
	}
	return false
}

// assignable は、src の値を dst の位置に渡せるか。名前付きレコードは、それを含むユニオンの位置に渡せる（自動注入）。
func assignable(src, dst types.Type) bool {
	if invalid(src) || invalid(dst) || types.Equal(src, dst) {
		return true
	}
	u, isUnion := dst.(*types.Union)
	r, isRec := src.(*types.Record)
	return isUnion && isRec && r.Name != "" && u.Has(r.Name)
}

// expect は、e の型を、期待される型 want のもとで決め、want に渡せるかを確かめる。
func (t *typer) expect(e syntax.Expr, want types.Type, what string) types.Type {
	got := t.expr(e, want)
	if !assignable(got, want) {
		t.mismatch(e.Pos(), what, want.String(), got.String())
	}
	return got
}

// block は、ブロックの中を型検査し、ブロックの値の型（最後のノードの型。最後が emit なら types.Invalid）を返す。
// want があれば、最後のノードに期待される型として渡し、渡せるかを確かめる（what はその名前）。
func (t *typer) block(b *syntax.Block, want types.Type, what string) types.Type {
	var last types.Type = types.Invalid{}
	for i, st := range b.Stmts {
		switch st := st.(type) {
		case *syntax.Node:
			var w types.Type
			if i == len(b.Stmts)-1 {
				w = want
			}
			typ := t.node(st, w)
			t.c.nodeTypes[st] = typ
			last = typ
			if w != nil && !assignable(typ, w) {
				t.atNode(st, func() { t.mismatch(st.ID.Span, what, w.String(), typ.String()) })
			}
		case *syntax.Emit:
			t.emit(st)
			last = types.Invalid{}
		}
	}
	return last
}

// atNode は、n の中の診断として f を呼ぶ。
func (t *typer) atNode(n *syntax.Node, f func()) {
	t.owners = append(t.owners, n)
	f()
	t.owners = t.owners[:len(t.owners)-1]
}

// lastNode は、ブロックが値で終わるなら、その最後のノードを返す。
func lastNode(b *syntax.Block) *syntax.Node {
	if len(b.Stmts) == 0 {
		return nil
	}
	n, _ := b.Stmts[len(b.Stmts)-1].(*syntax.Node)
	return n
}

func (t *typer) emit(e *syntax.Emit) {
	for _, p := range t.box.Outs {
		if p.Name.Name == e.Port.Name {
			t.expect(e.Value, t.c.portTypes[p], "emit $"+p.Name.Name+" の値")
			return
		}
	}
	t.expr(e.Value, nil) // 対象の誤りは BX013 で知らせた
}

func (t *typer) node(n *syntax.Node, want types.Type) types.Type {
	t.owners = append(t.owners, n)
	defer func() { t.owners = t.owners[:len(t.owners)-1] }()
	name := n.Op.Name
	if name == "call" {
		return t.call(n)
	}
	shape, ok := opShapes[name]
	if !ok {
		d := t.report(diag.BX021, n.Op.Span, "演算 %s はありません", name)
		d.Suggestions = suggest(name, opNames())
		t.skip(n)
		return types.Invalid{}
	}
	if !t.shape(n, shape) {
		t.skip(n)
		return types.Invalid{}
	}
	if name == "with" {
		return t.with(n)
	}
	ts := make([]types.Type, len(n.Args))
	for i, a := range n.Args {
		var w types.Type
		if name == "val" {
			w = want
		}
		ts[i] = t.expr(a, w)
	}
	switch name {
	case "val":
		return ts[0]
	case "add", "sub", "mul", "min", "max":
		if t.argIs(n, ts, 0, "Int か Dec", isNum) && t.argTo(n, ts, 1, ts[0]) {
			return ts[0]
		}
	case "div":
		if t.argTo(n, ts, 0, types.Dec) && t.argTo(n, ts, 1, types.Dec) {
			return types.Dec
		}
	case "idiv", "mod":
		if t.argTo(n, ts, 0, types.Int) && t.argTo(n, ts, 1, types.Int) {
			return types.Int
		}
	case "neg", "abs":
		if t.argIs(n, ts, 0, "Int か Dec", isNum) {
			return ts[0]
		}
	case "to_dec":
		t.argTo(n, ts, 0, types.Int)
		return types.Dec
	case "floor":
		t.argTo(n, ts, 0, types.Dec)
		return types.Int
	case "round":
		t.argTo(n, ts, 0, types.Dec)
		t.argTo(n, ts, 1, types.Int)
		return types.Dec
	case "to_str":
		t.argIs(n, ts, 0, "Int・Dec・Bool のどれか", isBasic(types.Int, types.Dec, types.Bool))
		return types.Str
	case "eq", "ne":
		if !assignable(ts[1], ts[0]) && !assignable(ts[0], ts[1]) {
			t.mismatch(n.Args[1].Pos(), argWhat(n, 1), ts[0].String(), ts[1].String())
		}
		return types.Bool
	case "lt", "le", "gt", "ge":
		if t.argIs(n, ts, 0, "Int・Dec・Str のどれか", isBasic(types.Int, types.Dec, types.Str)) {
			t.argTo(n, ts, 1, ts[0])
		}
		return types.Bool
	case "and", "or":
		t.argTo(n, ts, 0, types.Bool)
		t.argTo(n, ts, 1, types.Bool)
		return types.Bool
	case "not":
		t.argTo(n, ts, 0, types.Bool)
		return types.Bool
	case "concat":
		if t.argIs(n, ts, 0, "Str かリスト", func(x types.Type) bool { return x == types.Str || isList(x) }) && t.argTo(n, ts, 1, ts[0]) {
			return ts[0]
		}
	case "len", "empty":
		t.argIs(n, ts, 0, "Str・リスト・マップのどれか", func(x types.Type) bool { return x == types.Str || isList(x) || isMap(x) })
		if name == "len" {
			return types.Int
		}
		return types.Bool
	case "sum":
		if t.argIs(n, ts, 0, "[Int] か [Dec]", func(x types.Type) bool { return isList(x) && isNum(x.(*types.List).Elem) }) {
			return ts[0].(*types.List).Elem
		}
	case "reverse", "distinct":
		if t.argIs(n, ts, 0, "リスト", isList) {
			return ts[0]
		}
	case "take", "drop":
		ok := t.argIs(n, ts, 0, "リスト", isList)
		if t.argTo(n, ts, 1, types.Int) && ok {
			return ts[0]
		}
	case "range":
		t.argTo(n, ts, 0, types.Int)
		return &types.List{Elem: types.Int}
	case "lookup", "has", "set", "remove", "keys", "values":
		return t.mapOp(n, ts)
	case "map", "flat_map", "filter", "any", "all", "sort_by", "group_by", "fold":
		return t.iterate(n, ts)
	case "route":
		return t.route(n, ts, want)
	case "match":
		return t.match(n, ts, want)
	}
	return types.Invalid{}
}

// shape は、引数・束縛子の数と、ブロックの有無を確かめる（BX021）。
func (t *typer) shape(n *syntax.Node, s opShape) bool {
	name := n.Op.Name
	ok := true
	if len(n.Args) != s.args {
		_ = t.report(diag.BX021, n.Op.Span, "%s は引数を%dつ取ります（書かれた数: %d）", name, s.args, len(n.Args))
		ok = false
	}
	if len(n.Binders) != s.binders {
		if s.binders == 0 {
			_ = t.report(diag.BX021, n.Op.Span, "%s は束縛子を取りません（書かれた数: %d）", name, len(n.Binders))
		} else {
			_ = t.report(diag.BX021, n.Op.Span, "%s は束縛子を%dつ取ります（書かれた数: %d）", name, s.binders, len(n.Binders))
		}
		ok = false
	}
	switch {
	case s.block && n.Block == nil:
		_ = t.report(diag.BX021, n.Op.Span, "%s はブロックを取ります（書かれていません）", name)
		ok = false
	case !s.block && n.Block != nil:
		_ = t.report(diag.BX021, n.Op.Span, "%s はブロックを取りません", name)
		ok = false
	case n.Block != nil && len(n.Block.Stmts) == 0:
		_ = t.report(diag.BX021, n.Op.Span, "%s のブロックが空です", name)
		ok = false
	}
	return ok
}

// skip は、形の誤ったノードの中も確かめる（束縛子の型は決めず、types.Invalid のまま）。
func (t *typer) skip(n *syntax.Node) {
	for _, a := range n.Args {
		t.expr(a, nil)
	}
	for _, a := range n.CallArgs {
		t.expr(a.Value, nil)
	}
	for _, b := range innerBlocks(n) {
		t.block(b, nil, "")
	}
}

// argWhat は、i 番目の引数の名前（「add の1つ目の引数」、引数が1つなら「neg の引数」）。
func argWhat(n *syntax.Node, i int) string {
	if opShapes[n.Op.Name].args == 1 {
		return n.Op.Name + " の引数"
	}
	return n.Op.Name + " の" + strconv.Itoa(i+1) + "つ目の引数"
}

// argIs は、i 番目の引数の型が ok を満たすかを確かめる。満たさなければ BX020（期待した型 desc）。
// 誤りを含む型なら、診断を出さずに false。
func (t *typer) argIs(n *syntax.Node, ts []types.Type, i int, desc string, ok func(types.Type) bool) bool {
	if invalid(ts[i]) {
		return false
	}
	if ok(ts[i]) {
		return true
	}
	t.mismatch(n.Args[i].Pos(), argWhat(n, i), desc, ts[i].String())
	return false
}

// argTo は、i 番目の引数を want の位置に渡せるかを確かめる。どちらかが誤りを含む型なら、診断を出さずに false。
func (t *typer) argTo(n *syntax.Node, ts []types.Type, i int, want types.Type) bool {
	if invalid(ts[i]) || invalid(want) {
		return false
	}
	if assignable(ts[i], want) {
		return true
	}
	t.mismatch(n.Args[i].Pos(), argWhat(n, i), want.String(), ts[i].String())
	return false
}

func isNum(x types.Type) bool { return x == types.Int || x == types.Dec }

func isBasic(bs ...types.Basic) func(types.Type) bool {
	return func(x types.Type) bool {
		b, ok := x.(types.Basic)
		return ok && slices.Contains(bs, b)
	}
}

func isList(x types.Type) bool { _, ok := x.(*types.List); return ok }

func isMap(x types.Type) bool { _, ok := x.(*types.Map); return ok }

// mapOp は、マップの演算（lookup・has・set・remove・keys・values）。
func (t *typer) mapOp(n *syntax.Node, ts []types.Type) types.Type {
	name := n.Op.Name
	if !t.argIs(n, ts, 0, "マップ", isMap) {
		if name == "has" {
			return types.Bool
		}
		return types.Invalid{}
	}
	m := ts[0].(*types.Map)
	switch name {
	case "lookup":
		k := t.argTo(n, ts, 1, m.Key)
		if t.argTo(n, ts, 2, m.Value) && k {
			return m.Value
		}
	case "has":
		t.argTo(n, ts, 1, m.Key)
		return types.Bool
	case "set":
		k := t.argTo(n, ts, 1, m.Key)
		if t.argTo(n, ts, 2, m.Value) && k {
			return m
		}
	case "remove":
		if t.argTo(n, ts, 1, m.Key) {
			return m
		}
	case "keys":
		return &types.List{Elem: m.Key}
	case "values":
		return &types.List{Elem: m.Value}
	}
	return types.Invalid{}
}

// iterate は、繰り返しの演算（map・flat_map・filter・any・all・sort_by・group_by・fold）。
func (t *typer) iterate(n *syntax.Node, ts []types.Type) types.Type {
	name := n.Op.Name
	var elem types.Type = types.Invalid{}
	isL := t.argIs(n, ts, 0, "リスト", isList)
	if isL {
		elem = ts[0].(*types.List).Elem
	}
	what := name + " のブロックの値"
	if name == "fold" {
		acc := ts[1]
		t.c.binderTypes[&n.Binders[0]] = acc
		t.c.binderTypes[&n.Binders[1]] = elem
		t.block(n.Block, acc, what)
		return acc
	}
	t.c.binderTypes[&n.Binders[0]] = elem
	switch name {
	case "filter", "any", "all":
		t.block(n.Block, types.Bool, what)
		if name != "filter" {
			return types.Bool
		}
		if isL {
			return ts[0]
		}
		return types.Invalid{}
	}
	v := t.block(n.Block, nil, "")
	check := func(desc string, ok func(types.Type) bool) bool {
		if invalid(v) {
			return false
		}
		if ok(v) {
			return true
		}
		last := lastNode(n.Block)
		t.atNode(last, func() { t.mismatch(last.ID.Span, what, desc, v.String()) })
		return false
	}
	switch name {
	case "map":
		return &types.List{Elem: v}
	case "flat_map":
		if check("リスト", isList) {
			return v
		}
	case "sort_by":
		if check("Int・Dec・Str のどれか", isBasic(types.Int, types.Dec, types.Str)) && isL {
			return ts[0]
		}
	case "group_by":
		if check("Int・Str・Bool のどれか", types.IsMapKey) && isL {
			return &types.Map{Key: v, Value: ts[0]}
		}
	}
	return types.Invalid{}
}

// route は、route の条件と分岐の形を確かめる（BX021）。
func (t *typer) route(n *syntax.Node, ts []types.Type, want types.Type) types.Type {
	t.argTo(n, ts, 0, types.Bool)
	seen := map[string]bool{}
	for _, br := range n.Branches {
		c := br.Cond.Name
		switch {
		case c != "true" && c != "false": // 型の名前の分岐（パーサは、そのときだけ束縛子を読む）
			_ = t.report(diag.BX021, br.Cond.Span, "route の分岐は true と false です")
		case seen[c]:
			_ = t.report(diag.BX021, br.Cond.Span, "route の %s の分岐が重なっています", c)
		}
		seen[c] = true
	}
	for _, c := range []string{"true", "false"} {
		if !seen[c] {
			_ = t.report(diag.BX021, n.Op.Span, "route に %s の分岐がありません", c)
		}
	}
	return t.branchValues(n, want)
}

// match は、match の値と分岐のタグを確かめ（BX020・BX021・BX023）、束縛子の型を決める。
func (t *typer) match(n *syntax.Node, ts []types.Type, want types.Type) types.Type {
	u, isUnion := ts[0].(*types.Union)
	if !invalid(ts[0]) && !isUnion {
		t.mismatch(n.Args[0].Pos(), argWhat(n, 0), "ユニオン型", ts[0].String())
	}
	seen := map[string]bool{}
	for _, br := range n.Branches {
		if br.Binder.Name == "" {
			_ = t.report(diag.BX021, br.Cond.Span, "match の分岐は「型の名前 $束縛子」です")
			continue
		}
		typ, _ := t.c.env.Lookup(br.Cond.Name)
		r, isRec := typ.(*types.Record)
		named := isRec && r.Name != ""
		var bt types.Type = types.Invalid{}
		switch {
		case !isUnion:
			if named {
				bt = r
			}
		case !named || !u.Has(r.Name):
			d := t.report(diag.BX023, br.Cond.Span, "ユニオン %s にタグ %s はありません", u, br.Cond.Name)
			d.Suggestions = suggest(br.Cond.Name, memberNames(u))
		case seen[r.Name]:
			_ = t.report(diag.BX023, br.Cond.Span, "タグ %s の分岐が重なっています", r.Name)
			bt = r
		default:
			seen[r.Name] = true
			bt = r
		}
		t.c.binderTypes[&br.Binder] = bt
	}
	if isUnion {
		var missing []string
		for _, m := range u.Members {
			if !seen[m.Name] {
				missing = append(missing, m.Name)
			}
		}
		if len(missing) > 0 {
			_ = t.report(diag.BX023, n.Op.Span, "match の分岐が足りません（%s）", strings.Join(missing, "・"))
		}
	}
	return t.branchValues(n, want)
}

func memberNames(u *types.Union) []string {
	names := make([]string, len(u.Members))
	for i, m := range u.Members {
		names[i] = m.Name
	}
	return names
}

// branchValues は、分岐のブロックを型検査し、分岐ノードの値の型を返す。
// 値の分岐（どの分岐も値で終わる）なら、分岐の値の型が一致すること（want があれば、want に渡せること）。
// 出口の分岐と混在は、値を持たない（types.Invalid。分岐の種類は emits.go が確かめる）。
func (t *typer) branchValues(n *syntax.Node, want types.Type) types.Type {
	what := n.Op.Name + " の分岐の値"
	vals := make([]types.Type, len(n.Branches))
	exit := false
	for i, br := range n.Branches {
		vals[i] = t.block(br.Block, want, what)
		if lastNode(br.Block) == nil {
			exit = true
		}
	}
	if exit || len(vals) == 0 {
		return types.Invalid{}
	}
	if want != nil {
		return want
	}
	first := vals[0]
	for i, v := range vals[1:] {
		if !assignable(v, first) {
			last := lastNode(n.Branches[i+1].Block)
			t.atNode(last, func() { t.mismatch(last.ID.Span, what, first.String(), v.String()) })
		}
	}
	return first
}

// with は、#n with r {f: v, …} を確かめる。
func (t *typer) with(n *syntax.Node) types.Type {
	r0 := t.expr(n.Args[0], nil)
	rec, isRec := r0.(*types.Record)
	if !invalid(r0) && !isRec {
		t.mismatch(n.Args[0].Pos(), argWhat(n, 0), "レコード型", r0.String())
	}
	lit, ok := n.Args[1].(*syntax.RecordLit)
	if !ok || lit.Tag.Name != "" {
		_ = t.report(diag.BX021, n.Args[1].Pos(), "with の2つ目の引数は {フィールド: 値, …} です")
		t.expr(n.Args[1], nil)
		lit = &syntax.RecordLit{}
	}
	for _, f := range lit.Fields {
		var ft types.Type
		if isRec {
			if ft, ok = rec.Field(f.Name.Name); !ok {
				_ = t.noField(f.Name.Span, rec, f.Name.Name)
			}
		}
		if ft == nil {
			t.expr(f.Value, nil)
			continue
		}
		t.expect(f.Value, ft, "フィールド "+f.Name.Name+" ")
	}
	if isRec {
		return rec
	}
	return types.Invalid{}
}

// call は、呼び出す箱と、入力ポートの過不足（BX040）・型（BX020）を確かめ、出力ポートのレコード型を返す。
func (t *typer) call(n *syntax.Node) types.Type {
	callee := n.Callee.Name
	box := t.c.boxes[callee]
	if box == nil {
		d := t.report(diag.BX040, n.Callee.Span, "箱 %s はありません", callee)
		d.Suggestions = suggest(callee, t.c.boxList)
		t.skip(n)
		return types.Invalid{}
	}
	var inNames []string
	for _, p := range box.Ins {
		inNames = append(inNames, p.Name.Name)
	}
	seen := map[string]bool{}
	for _, a := range n.CallArgs {
		name := a.Name.Name
		i := slices.Index(inNames, name)
		switch {
		case seen[name]:
			_ = t.report(diag.BX040, a.Name.Span, "ポート %s が重なっています", name)
			t.expr(a.Value, nil)
		case i < 0:
			d := t.report(diag.BX040, a.Name.Span, "%s に入力ポート %s はありません", callee, name)
			d.Suggestions = suggest(name, inNames)
			t.expr(a.Value, nil)
		default:
			t.expect(a.Value, t.c.portTypes[box.Ins[i]], callee+" のポート "+name+" ")
		}
		seen[name] = true
	}
	var missing []string
	for _, name := range inNames {
		if !seen[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		_ = t.report(diag.BX040, n.Callee.Span, "%s の入力ポート %s が渡されていません", callee, strings.Join(missing, "・"))
	}
	out := &types.Record{}
	for _, p := range box.Outs {
		out.Fields = append(out.Fields, types.Field{Name: p.Name.Name, Type: t.c.portTypes[p]})
	}
	return out
}

// expr は、値の式の型を決める。want は期待される型（無ければ nil）で、空の集まりの型と、構築の中の期待される型を決める。
// want に渡せるかは、呼び出し側（expect）が確かめる。
func (t *typer) expr(e syntax.Expr, want types.Type) types.Type {
	switch e := e.(type) {
	case *syntax.NodeRef:
		var typ types.Type = types.Invalid{}
		if n := t.c.nodeRefs[e]; n != nil {
			if nt, ok := t.c.nodeTypes[n]; ok {
				typ = nt
			}
		}
		return t.fields(typ, e.Fields, e.Span, e.ID+fieldsPath(e.Fields))
	case *syntax.DollarRef:
		var typ types.Type = types.Invalid{}
		if b, ok := t.c.dollarRefs[e]; ok {
			if b.port != nil {
				typ = t.c.portTypes[b.port]
			} else if bt, ok := t.c.binderTypes[b.binder]; ok {
				typ = bt
			}
		}
		return t.fields(typ, e.Fields, e.Span, "$"+e.Name+fieldsPath(e.Fields))
	case *syntax.IntLit:
		return types.Int
	case *syntax.DecLit:
		return types.Dec
	case *syntax.StrLit:
		return types.Str
	case *syntax.BoolLit:
		return types.Bool
	case *syntax.RecordLit:
		if e.Tag.Name != "" {
			return t.construct(e)
		}
		return t.anonRecord(e, want)
	case *syntax.ListLit:
		return t.list(e, want)
	case *syntax.MapLit:
		return t.mapLit(e, want)
	case *syntax.Annotated:
		typ := t.c.resolve(e.Type, t.box.Name.Name)
		t.expect(e.Value, typ, "型注釈の値")
		return typ
	}
	panic("check: 未知の値の式")
}

// fields は、参照のフィールドをたどる（BX022）。
func (t *typer) fields(typ types.Type, fields []string, span diag.Span, path string) types.Type {
	for _, f := range fields {
		if invalid(typ) {
			return types.Invalid{}
		}
		r, ok := typ.(*types.Record)
		if !ok {
			t.report(diag.BX022, span, "型 %s にはフィールドがありません", typ).Path = path
			return types.Invalid{}
		}
		ft, ok := r.Field(f)
		if !ok {
			t.noField(span, r, f).Path = path
			return types.Invalid{}
		}
		typ = ft
	}
	return typ
}

func (t *typer) noField(span diag.Span, r *types.Record, name string) *diag.Diagnostic {
	d := t.report(diag.BX022, span, "レコード型 %s にフィールド %s はありません", r, name)
	d.Hint = "フィールド名の綴りを確認してください"
	names := make([]string, len(r.Fields))
	for i, f := range r.Fields {
		names[i] = f.Name
	}
	d.Suggestions = suggest(name, names)
	return d
}

// construct は、名前付きレコードの構築 N{f: v, …} を確かめる。フィールドは過不足なく書く。
func (t *typer) construct(e *syntax.RecordLit) types.Type {
	tag := e.Tag.Name
	typ, ok := t.c.env.Lookup(tag)
	r, isRec := typ.(*types.Record)
	switch {
	case !ok:
		d := t.report(diag.BX002, e.Tag.Span, "型 %s は定義されていません", tag)
		d.Suggestions = suggest(tag, t.c.typeNames())
	case invalid(typ):
	case !isRec || r.Name == "":
		_ = t.report(diag.BX020, e.Span, "型 %s は名前付きレコード型ではないので、%s{…} とは書けません", tag, tag)
	default:
		seen := map[string]bool{}
		for _, f := range e.Fields {
			name := f.Name.Name
			ft, ok := r.Field(name)
			switch {
			case seen[name]:
				_ = t.report(diag.BX020, f.Name.Span, "フィールド %s が重なっています", name)
				t.expr(f.Value, nil)
			case !ok:
				_ = t.noField(f.Name.Span, r, name)
				t.expr(f.Value, nil)
			default:
				t.expect(f.Value, ft, "フィールド "+name+" ")
			}
			seen[name] = true
		}
		var missing []string
		for _, f := range r.Fields {
			if !seen[f.Name] {
				missing = append(missing, f.Name)
			}
		}
		if len(missing) > 0 {
			_ = t.report(diag.BX020, e.Span, "%s の構築にフィールド %s がありません", r.Name, strings.Join(missing, "・"))
		}
		return r
	}
	for _, f := range e.Fields {
		t.expr(f.Value, nil)
	}
	return types.Invalid{}
}

// anonRecord は、無名レコード {f: v, …} の型を決める。want が無名レコード型なら、各フィールドに期待される型を渡す。
func (t *typer) anonRecord(e *syntax.RecordLit, want types.Type) types.Type {
	wr, _ := want.(*types.Record)
	if wr != nil && wr.Name != "" {
		wr = nil
	}
	out := &types.Record{}
	for _, f := range e.Fields {
		name := f.Name.Name
		if _, dup := out.Field(name); dup {
			_ = t.report(diag.BX020, f.Name.Span, "フィールド %s が重なっています", name)
			t.expr(f.Value, nil)
			continue
		}
		var ft types.Type
		if wr != nil {
			if fw, ok := wr.Field(name); ok {
				t.expect(f.Value, fw, "フィールド "+name+" ")
				ft = fw
			}
		}
		if ft == nil {
			ft = t.expr(f.Value, nil)
		}
		out.Fields = append(out.Fields, types.Field{Name: name, Type: ft})
	}
	return out
}

// empty は、空のリスト・空のマップの型を、期待される型から決める。決まらなければ BX024。
func (t *typer) empty(span diag.Span, want types.Type, isKind func(types.Type) bool, what, hint string) types.Type {
	switch {
	case want == nil:
		d := t.report(diag.BX024, span, "空の%sの型が決まりません", what)
		d.Hint = hint
	case invalid(want):
	case isKind(want):
		return want
	default:
		t.mismatch(span, "空の"+what, want.String(), what)
	}
	return types.Invalid{}
}

func (t *typer) list(e *syntax.ListLit, want types.Type) types.Type {
	if len(e.Elems) == 0 {
		return t.empty(e.Span, want, isList, "リスト", "([] : [Int]) のように型注釈を付けてください")
	}
	if l, ok := want.(*types.List); ok {
		for _, x := range e.Elems {
			t.expect(x, l.Elem, "リストの要素")
		}
		return l
	}
	first := t.expr(e.Elems[0], nil)
	for _, x := range e.Elems[1:] {
		t.expect(x, first, "リストの要素")
	}
	return &types.List{Elem: first}
}

func (t *typer) mapLit(e *syntax.MapLit, want types.Type) types.Type {
	if len(e.Entries) == 0 {
		return t.empty(e.Span, want, isMap, "マップ", "({->} : {Str -> Int}) のように型注釈を付けてください")
	}
	if m, ok := want.(*types.Map); ok {
		for _, en := range e.Entries {
			t.expect(en.Key, m.Key, "マップのキー")
			t.expect(en.Value, m.Value, "マップの値")
		}
		return m
	}
	key := t.expr(e.Entries[0].Key, nil)
	if !invalid(key) && !types.IsMapKey(key) {
		t.mismatch(e.Entries[0].Key.Pos(), "マップのキー", "Int・Str・Bool のどれか", key.String())
		key = types.Invalid{}
	}
	val := t.expr(e.Entries[0].Value, nil)
	for _, en := range e.Entries[1:] {
		t.expect(en.Key, key, "マップのキー")
		t.expect(en.Value, val, "マップの値")
	}
	return &types.Map{Key: key, Value: val}
}
