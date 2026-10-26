// Package interp は、検証を通った箱の本体を評価する（language.md §4.3〜§4.5）。
// 評価はテキストの順に行う。参照は常に前方だけなので、それで正しく動く。
package interp

import (
	"errors"
	"fmt"
	"math/big"
	"slices"

	"github.com/cockroachdb/apd/v3"

	"github.com/amisonnet8/boxundle-mvp/internal/check"
	"github.com/amisonnet8/boxundle-mvp/internal/diag"
	"github.com/amisonnet8/boxundle-mvp/internal/syntax"
	"github.com/amisonnet8/boxundle-mvp/internal/types"
	"github.com/amisonnet8/boxundle-mvp/internal/value"
)

// Caller は、call の呼び出し先の箱 box を入力 in で実行し、出力ポートのレコード（フィールドは出力ポートの宣言の順）を返す。
type Caller func(box string, in map[string]value.Value) (value.Record, error)

// Interp は、1つのモジュールの箱を評価する。
type Interp struct {
	Info *check.Info // 検証で決まった型の情報（エラーの無い検証のもの）
	Call Caller      // call の実行。nil のまま call を評価すると panic する
}

// Eval は、箱 b の本体を、入力ポートの値 in で評価し、出力ポートの値を返す。
// 実行時の誤りは、箱とノードの位置つきの diag.Diagnostic（BX-RUN-…）で返す。呼び出し先が返した誤りは、そのまま返す。
// in は入力ポートを過不足なく持ち、値はポートの型に合っていること（呼ぶ側が確かめる）。
func (it *Interp) Eval(b *syntax.Box, in map[string]value.Value) (map[string]value.Value, error) {
	e := &evaluator{it: it, box: b, outs: map[string]value.Value{}}
	sc := &scope{}
	for _, p := range b.Ins {
		v, ok := in[p.Name.Name]
		if !ok {
			panic("interp: 入力ポート " + p.Name.Name + " の値がありません")
		}
		sc.add("$"+p.Name.Name, v)
	}
	if _, err := e.block(b.Body, sc); err != nil {
		return nil, err
	}
	return e.outs, nil
}

// evaluator は、1つの箱の本体の評価の状態。
type evaluator struct {
	it   *Interp
	box  *syntax.Box
	outs map[string]value.Value // emit で確定した出力ポートの値
}

// scope は、参照できる名前（ノードの ID「#4.1」、ポート・束縛子の「$名前」）と値。
// ブロックごとに1つ作り、外側のスコープを parent でたどる。
// 名前は箱全体で重ならない（検証が BX010・BX014 で確かめた）ので、最初に見つかったものが参照の先。
type scope struct {
	parent *scope
	names  []string
	vals   []value.Value
}

func (s *scope) add(name string, v value.Value) {
	s.names = append(s.names, name)
	s.vals = append(s.vals, v)
}

func (s *scope) get(name string) value.Value {
	for ; s != nil; s = s.parent {
		if i := slices.Index(s.names, name); i >= 0 {
			return s.vals[i]
		}
	}
	panic("interp: 参照 " + name + " の先がありません")
}

// block は、ブロックの文を順に評価し、ブロックの値（最後のノードの値。最後が emit か出口の分岐なら nil）を返す。
// ノードの値は sc に足す（sc はこのブロックのスコープ）。
func (e *evaluator) block(b *syntax.Block, sc *scope) (value.Value, error) {
	var last value.Value
	for _, st := range b.Stmts {
		switch st := st.(type) {
		case *syntax.Node:
			v, err := e.node(st, sc)
			if err != nil {
				return nil, err
			}
			sc.add(st.ID.Name, v)
			last = v
		case *syntax.Emit:
			e.outs[st.Port.Name] = e.expr(st.Value, sc)
			last = nil
		}
	}
	return last, nil
}

// node は、ノードを評価する。組み込み演算の誤りは、このノードの位置を付けた診断にする。
func (e *evaluator) node(n *syntax.Node, sc *scope) (value.Value, error) {
	v, err := e.op(n, sc)
	if oe, ok := errors.AsType[*opError](err); ok {
		d := diag.New(oe.code, oe.msg)
		d.Box, d.Node, d.Span = e.box.Name.Name, n.ID.Name, n.Span
		return nil, d
	}
	return v, err
}

func (e *evaluator) op(n *syntax.Node, sc *scope) (value.Value, error) {
	switch n.Op.Name {
	case "call":
		return e.call(n, sc)
	case "with":
		return e.with(n, sc), nil
	}
	args := make([]value.Value, len(n.Args))
	for i, a := range n.Args {
		args[i] = e.expr(a, sc)
	}
	switch n.Op.Name {
	case "map", "flat_map", "filter", "any", "all", "sort_by", "group_by", "fold":
		return e.iterate(n, sc, args)
	case "route":
		want := "false"
		if args[0].(value.Bool) {
			want = "true"
		}
		for _, br := range n.Branches {
			if br.Cond.Name == want {
				return e.block(br.Block, &scope{parent: sc})
			}
		}
		panic("interp: route に " + want + " の分岐がありません")
	case "match":
		r := args[0].(value.Record)
		for _, br := range n.Branches {
			typ, _ := e.it.Info.Env.Lookup(br.Cond.Name)
			if typ.(*types.Record).Name == r.Tag {
				inner := &scope{parent: sc}
				inner.add("$"+br.Binder.Name, r)
				return e.block(br.Block, inner)
			}
		}
		panic("interp: match にタグ " + r.Tag + " の分岐がありません")
	}
	return apply(n.Op.Name, args, e.it.Info.NodeTypes[n])
}

func (e *evaluator) call(n *syntax.Node, sc *scope) (value.Value, error) {
	in := make(map[string]value.Value, len(n.CallArgs))
	for _, a := range n.CallArgs {
		in[a.Name.Name] = e.expr(a.Value, sc)
	}
	if e.it.Call == nil {
		panic("interp: call の実行が与えられていません")
	}
	return e.it.Call(n.Callee.Name, in)
}

// with は、#n with r {f: v, …}。r の写しの、書かれたフィールドだけを置き換える。
func (e *evaluator) with(n *syntax.Node, sc *scope) value.Value {
	r := e.expr(n.Args[0], sc).(value.Record)
	fields := slices.Clone(r.Fields)
	for _, f := range n.Args[1].(*syntax.RecordLit).Fields {
		fields[slices.Index(r.Names, f.Name.Name)] = e.expr(f.Value, sc)
	}
	return value.Record{Tag: r.Tag, Names: r.Names, Fields: fields}
}

// iterate は、繰り返しの演算。どの演算も、すべての要素についてブロックを評価する（any・all も、答えが決まったあとで止めない。q 6b850b78e2）。
func (e *evaluator) iterate(n *syntax.Node, sc *scope, args []value.Value) (value.Value, error) {
	xs := args[0].(value.List)
	each := func(vs ...value.Value) (value.Value, error) {
		inner := &scope{parent: sc}
		for i, b := range n.Binders {
			inner.add("$"+b.Name, vs[i])
		}
		return e.block(n.Block, inner)
	}
	if n.Op.Name == "fold" {
		acc := args[1]
		for _, x := range xs {
			v, err := each(acc, x)
			if err != nil {
				return nil, err
			}
			acc = v
		}
		return acc, nil
	}
	vals := make([]value.Value, len(xs))
	for i, x := range xs {
		v, err := each(x)
		if err != nil {
			return nil, err
		}
		vals[i] = v
	}
	switch n.Op.Name {
	case "map":
		return value.List(vals), nil
	case "flat_map":
		out := value.List{}
		for _, v := range vals {
			out = append(out, v.(value.List)...)
		}
		return out, nil
	case "filter":
		out := value.List{}
		for i, v := range vals {
			if v.(value.Bool) {
				out = append(out, xs[i])
			}
		}
		return out, nil
	case "any":
		return value.Bool(slices.Contains(vals, value.Value(value.Bool(true)))), nil
	case "all":
		return value.Bool(!slices.Contains(vals, value.Value(value.Bool(false)))), nil
	case "sort_by":
		out := value.List{}
		for _, i := range sortedIndex(vals) {
			out = append(out, xs[i])
		}
		return out, nil
	case "group_by":
		var m value.Map
		for _, i := range sortedIndex(vals) {
			k := len(m.Keys) - 1
			if k < 0 || value.Compare(m.Keys[k], vals[i]) != 0 {
				m.Keys = append(m.Keys, vals[i])
				m.Vals = append(m.Vals, value.List{})
				k++
			}
			m.Vals[k] = append(m.Vals[k].(value.List), xs[i])
		}
		return m, nil
	}
	panic("interp: 知らない繰り返しの演算 " + n.Op.Name)
}

// sortedIndex は、keys を昇順に並べたときの添字を返す。等しいキーは元の順のまま（安定）。
func sortedIndex(keys []value.Value) []int {
	idx := make([]int, len(keys))
	for i := range idx {
		idx[i] = i
	}
	slices.SortStableFunc(idx, func(a, b int) int { return value.Compare(keys[a], keys[b]) })
	return idx
}

// expr は、値の式を評価する。値の式は、参照とリテラル・構築だけなので、誤りにならない。
func (e *evaluator) expr(x syntax.Expr, sc *scope) value.Value {
	switch x := x.(type) {
	case *syntax.NodeRef:
		return fields(sc.get(x.ID), x.Fields)
	case *syntax.DollarRef:
		return fields(sc.get("$"+x.Name), x.Fields)
	case *syntax.IntLit:
		v, ok := new(big.Int).SetString(x.Text, 10)
		if !ok {
			panic("interp: 整数 " + x.Text + " を読めません")
		}
		return value.Int{V: v}
	case *syntax.DecLit:
		d, _, err := apd.NewFromString(x.Text)
		if err != nil {
			panic(fmt.Sprintf("interp: 小数 %s を読めません: %v", x.Text, err))
		}
		return value.Dec{V: *d}
	case *syntax.StrLit:
		return value.Str(x.Value)
	case *syntax.BoolLit:
		return value.Bool(x.Value)
	case *syntax.RecordLit:
		return e.record(x, sc)
	case *syntax.ListLit:
		out := make(value.List, len(x.Elems))
		for i, el := range x.Elems {
			out[i] = e.expr(el, sc)
		}
		return out
	case *syntax.MapLit:
		keys := make([]value.Value, len(x.Entries))
		vals := make([]value.Value, len(x.Entries))
		for i, en := range x.Entries {
			keys[i] = e.expr(en.Key, sc)
			vals[i] = e.expr(en.Value, sc)
		}
		return value.NewMap(keys, vals)
	case *syntax.Annotated:
		return e.expr(x.Value, sc)
	}
	panic(fmt.Sprintf("interp: 知らない値の式 %T", x))
}

// record は、レコードの構築。名前付きなら、フィールドを型の定義の順に並べ、タグは別名を解いた型の名前にする。
func (e *evaluator) record(x *syntax.RecordLit, sc *scope) value.Record {
	if x.Tag.Name == "" {
		r := value.Record{Names: make([]string, len(x.Fields)), Fields: make([]value.Value, len(x.Fields))}
		for i, f := range x.Fields {
			r.Names[i] = f.Name.Name
			r.Fields[i] = e.expr(f.Value, sc)
		}
		return r
	}
	typ, _ := e.it.Info.Env.Lookup(x.Tag.Name)
	rt := typ.(*types.Record)
	r := value.Record{Tag: rt.Name, Names: make([]string, len(rt.Fields)), Fields: make([]value.Value, len(rt.Fields))}
	for i, ft := range rt.Fields {
		r.Names[i] = ft.Name
		j := slices.IndexFunc(x.Fields, func(f *syntax.FieldArg) bool { return f.Name.Name == ft.Name })
		r.Fields[i] = e.expr(x.Fields[j].Value, sc)
	}
	return r
}

// fields は、参照のフィールドをたどる。
func fields(v value.Value, names []string) value.Value {
	for _, name := range names {
		f, ok := v.(value.Record).Field(name)
		if !ok {
			panic("interp: フィールド " + name + " がありません")
		}
		v = f
	}
	return v
}
