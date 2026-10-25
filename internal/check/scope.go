package check

import (
	"slices"
	"strings"

	"github.com/amisonnet8/boxundle-mvp/internal/diag"
	"github.com/amisonnet8/boxundle-mvp/internal/syntax"
)

// binding は、$名前 の参照が結び付いた定義。ポートか束縛子のどちらか一方を持つ。
type binding struct {
	port   *syntax.Port
	binder *syntax.Ident
}

// frame は、1つのブロックで見えるようになった名前。
type frame struct {
	nodes   map[string]*syntax.Node // ブロックの中で、今の位置より前にあるノード
	binders map[string]*syntax.Ident
}

// scope は、1つの箱の本体をたどるときの状態。
type scope struct {
	c      *checker
	box    *syntax.Box
	all    map[string][]*syntax.Node // 箱の中のすべてのノード（IDごと、書かれた順）
	ports  map[string]*syntax.Port   // 名前ごとの最初のポート
	frames []*frame                  // 外のブロックから順
	owners []*syntax.Node            // 今いるノード（外から順）
}

const (
	hintID     = "本体の直下のノードのIDは #数、ブロックの中のノードのIDは、ブロックを持つノードのIDに .数 を1つ足した形です"
	hintRef    = "参照できるのは、同じブロックか祖先のブロックで、前にあるノードだけです"
	hintShadow = "スコープの中で、ポート名と束縛子名は重ねられません"
)

// scopes は、モジュールの箱ごとに、ノードIDの規則（BX010・BX011）とスコープ（BX012〜BX015）を確かめる。
func (c *checker) scopes(f *syntax.File) {
	for _, decl := range f.Decls {
		if b, ok := decl.(*syntax.Box); ok && b.Body != nil {
			s := &scope{c: c, box: b, all: map[string][]*syntax.Node{}, ports: map[string]*syntax.Port{}}
			s.portNames()
			s.ids(b.Body, nil)
			s.block(b.Body, nil)
		}
	}
}

func (s *scope) report(code diag.Code, span diag.Span, path string, format string, args ...any) *diag.Diagnostic {
	d := s.c.report(code, span, s.box.Name.Name, format, args...)
	if len(s.owners) > 0 {
		d.Node = s.owners[len(s.owners)-1].ID.Name
	}
	d.Path = path
	return d
}

// portNames は、ポート名の重なり（BX014）を確かめる。
func (s *scope) portNames() {
	for _, p := range slices.Concat(s.box.Ins, s.box.Outs) {
		name := p.Name.Name
		if first := s.ports[name]; first != nil {
			d := s.report(diag.BX014, p.Name.Span, "$"+name, "ポート $%s が重なっています", name)
			d.Hint = "最初のポートは " + posText(first.Name.Span.Start) + " にあります"
			continue
		}
		s.ports[name] = p
	}
}

// ids は、ブロックの中のノードのIDを確かめる（BX010・BX011）。parent は、ブロックを持つノードのIDの数の並び（本体なら nil）。
func (s *scope) ids(b *syntax.Block, parent []string) {
	for _, st := range b.Stmts {
		n, ok := st.(*syntax.Node)
		if !ok {
			continue
		}
		id := n.ID.Name
		if first := s.all[id]; len(first) > 0 {
			d := s.c.report(diag.BX010, n.ID.Span, s.box.Name.Name, "ノードID %s が重なっています", id)
			d.Node = id
			d.Hint = "最初のノードは " + posText(first[0].ID.Span.Start) + " にあります"
		}
		s.all[id] = append(s.all[id], n)
		segs := strings.Split(strings.TrimPrefix(id, "#"), ".")
		if len(segs) != len(parent)+1 || !slices.Equal(segs[:len(parent)], parent) {
			var d *diag.Diagnostic
			if parent == nil {
				d = s.c.report(diag.BX011, n.ID.Span, s.box.Name.Name, "ノードID %s は、本体の直下のノードの形（#数）ではありません", id)
			} else {
				pid := "#" + strings.Join(parent, ".")
				d = s.c.report(diag.BX011, n.ID.Span, s.box.Name.Name, "ノードID %s は、%s のブロックの中のノードの形（%s.数）ではありません", id, pid, pid)
			}
			d.Node = id
			d.Hint = hintID
		}
		for _, inner := range innerBlocks(n) {
			s.ids(inner, segs)
		}
	}
}

// innerBlocks は、ノードが持つブロック（route・match では各分岐のブロック）を返す。
func innerBlocks(n *syntax.Node) []*syntax.Block {
	if n.Block != nil {
		return []*syntax.Block{n.Block}
	}
	blocks := make([]*syntax.Block, len(n.Branches))
	for i, br := range n.Branches {
		blocks[i] = br.Block
	}
	return blocks
}

// block は、ブロックの中の参照を確かめる。binders は、このブロックで見えるようになる束縛子。
func (s *scope) block(b *syntax.Block, binders []*syntax.Ident) {
	fr := &frame{nodes: map[string]*syntax.Node{}, binders: map[string]*syntax.Ident{}}
	for _, id := range binders {
		s.bind(fr, id)
	}
	s.frames = append(s.frames, fr)
	for _, st := range b.Stmts {
		switch st := st.(type) {
		case *syntax.Node:
			s.node(st)
			if fr.nodes[st.ID.Name] == nil {
				fr.nodes[st.ID.Name] = st
			}
		case *syntax.Emit:
			s.emitPort(st.Port)
			s.expr(st.Value)
		}
	}
	s.frames = s.frames[:len(s.frames)-1]
}

func (s *scope) node(n *syntax.Node) {
	s.owners = append(s.owners, n)
	for _, a := range n.Args {
		s.expr(a)
	}
	for _, a := range n.CallArgs {
		s.expr(a.Value)
	}
	var binders []*syntax.Ident
	for i := range n.Binders {
		binders = append(binders, &n.Binders[i])
	}
	if n.Block != nil {
		s.block(n.Block, binders)
	}
	for _, br := range n.Branches {
		bs := binders
		if br.Binder.Name != "" {
			bs = append(slices.Clip(bs), &br.Binder)
		}
		s.block(br.Block, bs)
	}
	s.owners = s.owners[:len(s.owners)-1]
}

// bind は、束縛子を fr に足す。同じブロック・外のブロックの束縛子やポートと重なれば BX014。
func (s *scope) bind(fr *frame, id *syntax.Ident) {
	name := id.Name
	path := "$" + name
	switch {
	case fr.binders[name] != nil:
		s.report(diag.BX014, id.Span, path, "束縛子 $%s が重なっています", name).Hint = hintShadow
	case s.ports[name] != nil:
		s.report(diag.BX014, id.Span, path, "束縛子 $%s は、ポート $%s と同じ名前です", name, name).Hint = hintShadow
	case s.lookupBinder(name) != nil:
		s.report(diag.BX014, id.Span, path, "束縛子 $%s は、外側の束縛子 $%s と同じ名前です", name, name).Hint = hintShadow
	}
	fr.binders[name] = id
}

func (s *scope) lookupBinder(name string) *syntax.Ident {
	for _, fr := range slices.Backward(s.frames) {
		if id := fr.binders[name]; id != nil {
			return id
		}
	}
	return nil
}

func (s *scope) lookupNode(id string) *syntax.Node {
	for _, fr := range slices.Backward(s.frames) {
		if n := fr.nodes[id]; n != nil {
			return n
		}
	}
	return nil
}

func (s *scope) expr(e syntax.Expr) {
	switch e := e.(type) {
	case *syntax.NodeRef:
		s.nodeRef(e)
	case *syntax.DollarRef:
		s.dollarRef(e)
	case *syntax.RecordLit:
		for _, f := range e.Fields {
			s.expr(f.Value)
		}
	case *syntax.ListLit:
		for _, x := range e.Elems {
			s.expr(x)
		}
	case *syntax.MapLit:
		for _, m := range e.Entries {
			s.expr(m.Key)
			s.expr(m.Value)
		}
	case *syntax.Annotated:
		s.expr(e.Value)
	}
}

// nodeRef は、ノードの参照を、見えているノードに結び付ける。見えなければ BX012。
func (s *scope) nodeRef(r *syntax.NodeRef) {
	if n := s.lookupNode(r.ID); n != nil {
		s.c.nodeRefs[r] = n
		return
	}
	path := r.ID + fieldsPath(r.Fields)
	defs := s.all[r.ID]
	if len(defs) == 0 {
		_ = s.report(diag.BX012, r.Span, path, "ノード %s はありません", r.ID)
		return
	}
	var d *diag.Diagnostic
	switch {
	case slices.ContainsFunc(defs, func(n *syntax.Node) bool { return slices.Contains(s.owners, n) }):
		d = s.report(diag.BX012, r.Span, path, "ノード %s は、この参照を含むノードなので参照できません", r.ID)
	case before(r.Span.Start, defs[0].Span.Start):
		d = s.report(diag.BX012, r.Span, path, "ノード %s は、この参照より後ろにあります", r.ID)
	default:
		d = s.report(diag.BX012, r.Span, path, "ノード %s は、ここから見えないブロックの中にあります", r.ID)
	}
	d.Hint = hintRef
}

// dollarRef は、$名前 の参照を、束縛子か入力ポートに結び付ける。出力ポートなら BX015、無ければ BX013。
func (s *scope) dollarRef(r *syntax.DollarRef) {
	path := "$" + r.Name + fieldsPath(r.Fields)
	if id := s.lookupBinder(r.Name); id != nil {
		s.c.dollarRefs[r] = binding{binder: id}
		return
	}
	if p := s.ports[r.Name]; p != nil {
		if slices.Contains(s.box.Outs, p) {
			d := s.report(diag.BX015, r.Span, path, "出力ポート $%s は参照できません", r.Name)
			d.Hint = "出力ポートは emit で値を確定させるだけで、値は読めません"
			return
		}
		s.c.dollarRefs[r] = binding{port: p}
		return
	}
	var names []string
	for _, p := range s.box.Ins {
		names = append(names, p.Name.Name)
	}
	for _, fr := range s.frames {
		for name := range fr.binders {
			names = append(names, name)
		}
	}
	d := s.report(diag.BX013, r.Span, path, "ポート・束縛子 $%s はありません", r.Name)
	d.Suggestions = suggest(r.Name, names)
}

// emitPort は、emit の対象が出力ポートであることを確かめる（BX013）。
func (s *scope) emitPort(port syntax.Ident) {
	name := port.Name
	isName := func(p *syntax.Port) bool { return p.Name.Name == name }
	if slices.ContainsFunc(s.box.Outs, isName) {
		return
	}
	d := s.report(diag.BX013, port.Span, "$"+name, "出力ポート $%s はありません", name)
	if slices.ContainsFunc(s.box.Ins, isName) {
		d.Hint = "$" + name + " は入力ポートです。emit できるのは出力ポートだけです"
		return
	}
	var names []string
	for _, p := range s.box.Outs {
		names = append(names, p.Name.Name)
	}
	d.Suggestions = suggest(name, names)
}

func fieldsPath(fields []string) string {
	if len(fields) == 0 {
		return ""
	}
	return "." + strings.Join(fields, ".")
}

// before は、a が b より前にあるか。
func before(a, b diag.Pos) bool {
	return a.Line < b.Line || (a.Line == b.Line && a.Col < b.Col)
}
