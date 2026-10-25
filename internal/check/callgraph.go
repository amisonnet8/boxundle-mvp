package check

import (
	"slices"
	"strings"

	"github.com/amisonnet8/boxundle-mvp/internal/diag"
	"github.com/amisonnet8/boxundle-mvp/internal/syntax"
)

// permNames は、権限名の一覧（language.md §4.6）。
var permNames = []string{"net", "fs.read", "fs.write", "clock", "random", "env"}

// callNodes は、ブロックの中（入れ子のブロックも含む）の call のノードを、書かれた順に返す。
func callNodes(b *syntax.Block) []*syntax.Node {
	var calls []*syntax.Node
	for _, st := range b.Stmts {
		n, ok := st.(*syntax.Node)
		if !ok {
			continue
		}
		if n.Op.Name == "call" {
			calls = append(calls, n)
		}
		for _, inner := range innerBlocks(n) {
			calls = append(calls, callNodes(inner)...)
		}
	}
	return calls
}

// modBoxes は、モジュールの本体を持つ箱を、書かれた順に返す。
func modBoxes(f *syntax.File) []*syntax.Box {
	var boxes []*syntax.Box
	for _, decl := range f.Decls {
		if b, ok := decl.(*syntax.Box); ok && b.Body != nil {
			boxes = append(boxes, b)
		}
	}
	return boxes
}

// callGraph は、モジュールの箱の呼び出しの循環（BX041）、スタブの対象（BX061）、権限の伝播（BX071・BX073）を確かめる。
func (c *checker) callGraph(f *syntax.File) {
	boxes := modBoxes(f)
	calls := map[*syntax.Box][]*syntax.Node{}
	for _, b := range boxes {
		calls[b] = callNodes(b.Body)
	}
	c.callCycles(boxes, calls)
	for _, b := range boxes {
		c.stubTargets(b, calls)
		c.permUse(b, calls[b])
	}
}

// callCycles は、箱の呼び出しをたどり、循環ごとに BX041 を1つ、循環の最初の箱から出る call に出す。
func (c *checker) callCycles(boxes []*syntax.Box, calls map[*syntax.Box][]*syntax.Node) {
	const (
		unvisited = iota
		visiting
		visited
	)
	type frame struct {
		box *syntax.Box
		via *syntax.Node // 次の箱へ進んだ call
	}
	state := map[*syntax.Box]int{}
	var stack []*frame
	var visit func(b *syntax.Box)
	visit = func(b *syntax.Box) {
		state[b] = visiting
		fr := &frame{box: b}
		stack = append(stack, fr)
		for _, n := range calls[b] {
			next := c.boxes[n.Callee.Name]
			if next == nil || next.Body == nil {
				continue
			}
			fr.via = n
			switch state[next] {
			case visiting:
				i := slices.IndexFunc(stack, func(x *frame) bool { return x.box == next })
				var names []string
				for _, x := range stack[i:] {
					names = append(names, x.box.Name.Name)
				}
				names = append(names, next.Name.Name)
				via := stack[i].via
				d := c.report(diag.BX041, via.Callee.Span, next.Name.Name, "箱の呼び出しが循環しています: %s", strings.Join(names, " → "))
				d.Node = via.ID.Name
				d.Hint = "箱は、自分自身を（ほかの箱を通しても）呼び出せません"
			case unvisited:
				visit(next)
			}
		}
		stack = stack[:len(stack)-1]
		state[b] = visited
	}
	for _, b := range boxes {
		if state[b] == unvisited {
			visit(b)
		}
	}
}

// stubTargets は、箱のテストのスタブの対象が、その箱から（入れ子を含めて）呼ばれるかを確かめる（BX061）。
// 対象の箱が無いときは、BX040 で知らせた。
func (c *checker) stubTargets(b *syntax.Box, calls map[*syntax.Box][]*syntax.Node) {
	reached := map[string]bool{}
	var walk func(x *syntax.Box)
	walk = func(x *syntax.Box) {
		for _, n := range calls[x] {
			name := n.Callee.Name
			if reached[name] {
				continue
			}
			reached[name] = true
			if next := c.boxes[name]; next != nil {
				walk(next)
			}
		}
	}
	walk(b)
	for _, test := range b.Tests {
		for _, s := range test.Stubs {
			name := s.Box.Name
			if c.boxes[name] == nil || reached[name] {
				continue
			}
			d := c.report(diag.BX061, s.Box.Span, b.Name.Name, "テスト「%s」のスタブの対象 %s は、箱 %s から呼ばれていません", test.Name.Name, name, b.Name.Name)
			d.Hint = "スタブは、その箱から（入れ子を含めて）呼ばれる箱にだけ書けます"
		}
	}
}

// perms は、箱の perm の権限名を確かめ（未知の名前と重なりは BX072）、正しい権限名を書かれた順に返す。
func (c *checker) perms(b *syntax.Box) []syntax.Ident {
	if b.Perm == nil {
		return nil
	}
	var ok []syntax.Ident
	for _, id := range b.Perm.Names {
		i := slices.IndexFunc(ok, func(x syntax.Ident) bool { return x.Name == id.Name })
		switch {
		case !slices.Contains(permNames, id.Name):
			d := c.report(diag.BX072, id.Span, b.Name.Name, "権限 %s はありません", id.Name)
			d.Suggestions = suggest(id.Name, permNames)
		case i >= 0:
			d := c.report(diag.BX072, id.Span, b.Name.Name, "権限 %s が重なっています", id.Name)
			d.Hint = "最初の " + id.Name + " は " + posText(ok[i].Span.Start) + " にあります"
		default:
			ok = append(ok, id)
		}
	}
	return ok
}

// stdPerms は、標準の箱の宣言の権限名を確かめる（BX072）。
func (c *checker) stdPerms(std []*syntax.File) {
	for _, f := range std {
		for _, decl := range f.Decls {
			if b, ok := decl.(*syntax.Box); ok {
				c.perms(b)
			}
		}
	}
}

// permUse は、箱の perm が、本体で呼ぶ箱の権限をすべて宣言しているか（不足は BX071）、
// 使っていない権限が無いか（警告 BX073）を確かめる。呼ぶ箱の権限は、その箱が perm に宣言したもの。
func (c *checker) permUse(b *syntax.Box, calls []*syntax.Node) {
	declared := c.perms(b)
	has := func(name string) bool {
		return slices.ContainsFunc(declared, func(x syntax.Ident) bool { return x.Name == name })
	}
	used := map[string]bool{}
	for _, n := range calls {
		callee := c.boxes[n.Callee.Name]
		if callee == nil || callee.Perm == nil {
			continue
		}
		var missing []string
		for _, id := range callee.Perm.Names {
			name := id.Name
			if !slices.Contains(permNames, name) || used[name] {
				continue
			}
			used[name] = true
			if !has(name) {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			d := c.report(diag.BX071, n.Callee.Span, b.Name.Name, "%s が使う権限 %s が、perm にありません", n.Callee.Name, strings.Join(missing, "・"))
			d.Node = n.ID.Name
			d.Hint = "箱の perm には、本体で呼ぶ箱の権限をすべて宣言します"
		}
	}
	for _, id := range declared {
		if !used[id.Name] {
			d := c.report(diag.BX073, id.Span, b.Name.Name, "宣言しているが使っていない権限 %s があります", id.Name)
			d.Hint = "本体で呼ぶ箱は、どれもこの権限を使いません"
		}
	}
}
