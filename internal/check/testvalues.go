package check

import (
	"strings"

	"github.com/amisonnet8/boxundle-mvp/internal/diag"
	"github.com/amisonnet8/boxundle-mvp/internal/syntax"
)

// testValues は、箱のテストの given・expect とスタブの出力が、ポートを過不足なく持ち、ポートの型に合っているかを確かめる（BX060）。
// スタブの対象の箱が無ければ BX040。値の中の型の不一致も BX060 にする（typer.test）。
func (c *checker) testValues(files []*syntax.File) {
	for _, file := range files {
		for _, decl := range file.Decls {
			b, ok := decl.(*syntax.Box)
			if !ok {
				continue
			}
			t := &typer{c: c, box: b, test: true}
			for _, test := range b.Tests {
				t.testStubs(test)
				t.testPorts(test, "given ", test.Given, b.Ins, "入力")
				t.testPorts(test, "expect ", test.Expect, b.Outs, "出力")
			}
		}
	}
}

func (t *typer) testStubs(test *syntax.Test) {
	seen := map[string]*syntax.Stub{}
	for _, s := range test.Stubs {
		name := s.Box.Name
		callee := t.c.boxes[name]
		switch {
		case callee == nil:
			d := t.report(diag.BX040, s.Box.Span, "箱 %s はありません", name)
			d.Suggestions = suggest(name, t.c.boxList)
		case seen[name] != nil:
			d := t.report(diag.BX060, s.Box.Span, "テスト「%s」で、箱 %s のスタブが重なっています", test.Name.Name, name)
			d.Hint = "最初のスタブは " + posText(seen[name].Box.Span.Start) + " にあります"
		default:
			seen[name] = s
			t.testPorts(test, name+" のスタブ", s.Outputs, callee.Outs, "出力")
		}
	}
}

// testPorts は、lit のフィールドが ports を過不足なく持ち、それぞれポートの型に合うかを確かめる。
// where は「given 」「std.clock.now のスタブ」のような、lit の名前。kind は「入力」か「出力」。
func (t *typer) testPorts(test *syntax.Test, where string, lit *syntax.RecordLit, ports []*syntax.Port, kind string) {
	tn := test.Name.Name
	var names []string
	for _, p := range ports {
		names = append(names, p.Name.Name)
	}
	seen := map[string]bool{}
	for _, f := range lit.Fields {
		name := f.Name.Name
		i := indexPort(ports, name)
		switch {
		case seen[name]:
			_ = t.report(diag.BX060, f.Name.Span, "テスト「%s」の %sで、ポート %s が重なっています", tn, where, name)
		case i < 0:
			d := t.report(diag.BX060, f.Name.Span, "テスト「%s」の %sに、%sポート %s はありません", tn, where, kind, name)
			d.Suggestions = suggest(name, names)
		default:
			t.expect(f.Value, t.c.portTypes[ports[i]], "テスト「"+tn+"」の "+where+"の "+name+" ")
		}
		seen[name] = true
	}
	var missing []string
	for _, name := range names {
		if !seen[name] {
			missing = append(missing, name)
			seen[name] = true // ポート名の重なり（BX014）を、重ねて知らせない
		}
	}
	if len(missing) > 0 {
		_ = t.report(diag.BX060, lit.Span, "テスト「%s」の %sに、%sポート %s がありません", tn, where, kind, strings.Join(missing, "・"))
	}
}

// indexPort は、ports の中で名前が name の最初のポートの位置（無ければ -1）。
func indexPort(ports []*syntax.Port, name string) int {
	for i, p := range ports {
		if p.Name.Name == name {
			return i
		}
	}
	return -1
}
