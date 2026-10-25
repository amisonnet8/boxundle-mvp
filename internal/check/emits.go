package check

import (
	"strings"

	"github.com/amisonnet8/boxundle-mvp/internal/diag"
	"github.com/amisonnet8/boxundle-mvp/internal/syntax"
)

// branchKind は、route・match の分岐の種類（language.md §4.3）。
type branchKind int

const (
	notBranch    branchKind = iota // route・match でない
	valueBranch                    // どの分岐も値で終わる
	exitBranch                     // どの分岐も emit で終わる
	mixedBranch                    // 値で終わる分岐と emit で終わる分岐が混ざる（BX032）
	unsureBranch                   // 空の分岐、または種類の決まらない分岐で終わる分岐がある（形の誤りは型検査が知らせた）
)

const hintExit = "出口の分岐は、どの分岐も emit で終わる route・match です"

// flow は、1つの箱の本体の、分岐と出力をたどるときの状態。
type flow struct {
	c         *checker
	box       *syntax.Box
	outs      []*syntax.Port // 名前ごとの最初の出力ポート
	kinds     map[*syntax.Node]branchKind
	owners    []*syntax.Node  // 今いるノード（外から順）
	misplaced map[string]bool // 位置の誤った emit がある出力ポート（足りない emit を重ねて知らせない）
	unsure    bool            // 経路を決められない（出力の網羅を確かめない）
}

// flows は、モジュールの箱ごとに、分岐の種類（BX032）、emit と出口の分岐の位置（BX031）、出力の網羅（BX030）を確かめる。
func (c *checker) flows(f *syntax.File) {
	for _, decl := range f.Decls {
		b, ok := decl.(*syntax.Box)
		if !ok || b.Body == nil {
			continue
		}
		fl := &flow{c: c, box: b, kinds: map[*syntax.Node]branchKind{}, misplaced: map[string]bool{}}
		seen := map[string]bool{}
		for _, p := range b.Outs {
			if !seen[p.Name.Name] {
				seen[p.Name.Name] = true
				fl.outs = append(fl.outs, p)
			}
		}
		fl.block(b.Body, true)
		if !fl.unsure {
			fl.cover(b.Body, make([]*syntax.Emit, len(fl.outs)), nil, nil)
		}
	}
}

func (fl *flow) report(code diag.Code, span diag.Span, format string, args ...any) *diag.Diagnostic {
	d := fl.c.report(code, span, fl.box.Name.Name, format, args...)
	d.Node = fl.owners[len(fl.owners)-1].ID.Name // 位置の誤りは、必ずノードの中にある
	return d
}

// kind は、ノードの分岐の種類を決める。
func (fl *flow) kind(n *syntax.Node) branchKind {
	if k, ok := fl.kinds[n]; ok {
		return k
	}
	k := notBranch
	if n.Op.Name == "route" || n.Op.Name == "match" {
		k = unsureBranch
		values, exits := 0, 0
		for _, br := range n.Branches {
			switch fl.blockKind(br.Block) {
			case valueBranch:
				values++
			case exitBranch:
				exits++
			}
		}
		switch {
		case values+exits < len(n.Branches):
		case exits == 0:
			k = valueBranch
		case values == 0:
			k = exitBranch
		default:
			k = mixedBranch
		}
	}
	fl.kinds[n] = k
	return k
}

// blockKind は、分岐のブロックが値で終わるか（valueBranch）、emit で終わるか（exitBranch）を決める。
// 最後が出口の分岐なら emit で終わる。空のブロックと、最後が決められない分岐のものは unsureBranch。
func (fl *flow) blockKind(b *syntax.Block) branchKind {
	if len(b.Stmts) == 0 {
		return unsureBranch
	}
	n, ok := b.Stmts[len(b.Stmts)-1].(*syntax.Node)
	if !ok {
		return exitBranch
	}
	switch fl.kind(n) {
	case exitBranch:
		return exitBranch
	case mixedBranch, unsureBranch:
		return unsureBranch
	}
	return valueBranch
}

// block は、ブロックの中の emit と出口の分岐の位置を確かめる。exits は、ここに emit と出口の分岐を書けるか
// （本体と、emit で終わる分岐のブロック）。
func (fl *flow) block(b *syntax.Block, exits bool) {
	for i, st := range b.Stmts {
		switch st := st.(type) {
		case *syntax.Emit:
			if !exits {
				fl.report(diag.BX031, st.Span, "emit は、本体の直下か、出口の分岐のブロックの中にしか書けません").Hint = hintExit
				fl.misplaced[st.Port.Name] = true
			}
		case *syntax.Node:
			fl.node(st, exits, i == len(b.Stmts)-1)
		}
	}
}

func (fl *flow) node(n *syntax.Node, exits, last bool) {
	fl.owners = append(fl.owners, n)
	defer func() { fl.owners = fl.owners[:len(fl.owners)-1] }()
	switch fl.kind(n) {
	case exitBranch:
		switch {
		case !exits:
			_ = fl.report(diag.BX031, n.ID.Span, "%s はどの分岐も emit で終わるので、本体か出口の分岐のブロックの中にしか置けません", n.Op.Name)
		case !last:
			_ = fl.report(diag.BX031, n.ID.Span, "%s はどの分岐も emit で終わるので、ブロックの最後に置きます（後ろにノードも emit も書けません）", n.Op.Name)
			fl.unsure = true
		}
	case mixedBranch:
		var valueConds, exitConds []string
		for _, br := range n.Branches {
			if fl.blockKind(br.Block) == exitBranch {
				exitConds = append(exitConds, br.Cond.Name)
			} else {
				valueConds = append(valueConds, br.Cond.Name)
			}
		}
		d := fl.report(diag.BX032, n.Op.Span, "%s の分岐に、値で終わる分岐（%s）と emit で終わる分岐（%s）が混ざっています",
			n.Op.Name, strings.Join(valueConds, "・"), strings.Join(exitConds, "・"))
		d.Hint = "どの分岐も値で終わるか、どの分岐も emit で終わるように揃えてください"
		fl.unsure = true
	case unsureBranch:
		fl.unsure = true
	}
	if n.Block != nil {
		fl.block(n.Block, false)
	}
	for _, br := range n.Branches {
		fl.block(br.Block, fl.blockKind(br.Block) == exitBranch)
	}
}

// cover は、本体を出口の分岐ごとの経路に分けてたどり、どの経路でも出力ポートごとにちょうど1回 emit されるかを確かめる（BX030）。
// firsts は、出力ポートごとの、この経路で最初の emit。at と br は、今いる出口の分岐とその分岐（本体なら nil）。
// 出口の分岐はブロックの最後にしか無い（block が確かめた）ので、ブロックの終わりが経路の終わり。
func (fl *flow) cover(b *syntax.Block, firsts []*syntax.Emit, at *syntax.Node, br *syntax.Branch) {
	for _, st := range b.Stmts {
		switch st := st.(type) {
		case *syntax.Emit:
			i := indexPort(fl.outs, st.Port.Name)
			switch {
			case i < 0: // 対象の誤りは BX013 で知らせた
			case firsts[i] != nil:
				d := fl.atReport(at, st.Span, "出力ポート $%s は、この経路ですでに emit されています", st.Port.Name)
				d.Path = "$" + st.Port.Name
				d.Hint = "最初の emit は " + posText(firsts[i].Span.Start) + " にあります"
			default:
				firsts[i] = st
			}
		case *syntax.Node:
			if fl.kinds[st] == exitBranch {
				for _, inner := range st.Branches {
					fl.cover(inner.Block, append([]*syntax.Emit(nil), firsts...), st, inner)
				}
				return
			}
		}
	}
	for i, p := range fl.outs {
		name := p.Name.Name
		if firsts[i] != nil || fl.misplaced[name] {
			continue
		}
		var d *diag.Diagnostic
		if at == nil {
			d = fl.atReport(nil, p.Name.Span, "出力ポート $%s が emit されていません", name)
		} else {
			d = fl.atReport(at, br.Cond.Span, "%s の %s の分岐で、出力ポート $%s が emit されていません", at.Op.Name, br.Cond.Name, name)
		}
		d.Path = "$" + name
		d.Hint = "本体のどの経路でも、出力ポートごとにちょうど1回 emit します"
	}
}

// atReport は、出口の分岐 at の中の診断を出す（at が nil なら本体の直下）。
func (fl *flow) atReport(at *syntax.Node, span diag.Span, format string, args ...any) *diag.Diagnostic {
	d := fl.c.report(diag.BX030, span, fl.box.Name.Name, format, args...)
	if at != nil {
		d.Node = at.ID.Name
	}
	return d
}
