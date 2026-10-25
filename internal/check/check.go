// Package check は、モジュールを検証する（language.md §5）。誤りは最初の1つで止めず、すべて返す。
package check

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/amisonnet8/boxundle-mvp/internal/diag"
	"github.com/amisonnet8/boxundle-mvp/internal/syntax"
	"github.com/amisonnet8/boxundle-mvp/internal/types"
)

// checker は、1回の検証の状態。
type checker struct {
	diags       []diag.Diagnostic
	defs        map[string]*typeDef // 登録した型定義（標準の箱の宣言とモジュール）
	order       []*typeDef          // 登録した型定義の、書かれた順（標準の箱の宣言が先）
	extra       []*typeDef          // 登録しなかった型定義（重複など）。右辺の誤りだけを確かめる
	env         *types.Env
	portTypes   map[*syntax.Port]types.Type
	nodeRefs    map[*syntax.NodeRef]*syntax.Node // ノードの参照が結び付いたノード
	dollarRefs  map[*syntax.DollarRef]binding    // $名前 の参照が結び付いたポート・束縛子
	nodeTypes   map[*syntax.Node]types.Type      // ノードの値の型（値を持たないノードは types.Invalid）
	binderTypes map[*syntax.Ident]types.Type     // 束縛子の型
	boxes       map[string]*syntax.Box           // 呼び出せる箱（標準の箱とモジュールの箱）
	boxList     []string                         // 呼び出せる箱の名前（候補に使う）
}

// Check は、モジュール f を、標準の箱の宣言 std とともに検証し、診断をソースの位置の順に返す。
func Check(f *syntax.File, std []*syntax.File) []diag.Diagnostic {
	return checkModule(f, std).sorted()
}

// checkModule は、検証を行い、その状態（解いた型の表・ポートの型）を返す。
func checkModule(f *syntax.File, std []*syntax.File) *checker {
	c := newChecker()
	c.externs(f)
	c.collectTypes(std, false)
	c.collectTypes([]*syntax.File{f}, true)
	c.boxNames(f)
	c.typeDefs()
	c.ports(std)
	c.ports([]*syntax.File{f})
	c.collectBoxes(std, f)
	c.scopes(f)
	c.typecheck(f)
	return c
}

// CheckStd は、標準の箱の宣言 std そのものを検証する。診断が出たら boxundle 自体の不具合。
func CheckStd(std []*syntax.File) []diag.Diagnostic {
	c := newChecker()
	c.collectTypes(std, false)
	c.typeDefs()
	c.ports(std)
	return c.sorted()
}

func newChecker() *checker {
	return &checker{
		defs:        map[string]*typeDef{},
		portTypes:   map[*syntax.Port]types.Type{},
		nodeRefs:    map[*syntax.NodeRef]*syntax.Node{},
		dollarRefs:  map[*syntax.DollarRef]binding{},
		nodeTypes:   map[*syntax.Node]types.Type{},
		binderTypes: map[*syntax.Ident]types.Type{},
		boxes:       map[string]*syntax.Box{},
	}
}

// report は、診断を1つ足す。戻り値は、足した診断にヒント・候補を書き足すときだけ使う
// （次に report を呼ぶまで有効。*diag.Diagnostic は error を満たすので、使わないときは _ = で捨てる）。
func (c *checker) report(code diag.Code, span diag.Span, box string, format string, args ...any) *diag.Diagnostic {
	d := diag.New(code, fmt.Sprintf(format, args...))
	d.Span, d.Box = span, box
	c.diags = append(c.diags, d)
	return &c.diags[len(c.diags)-1]
}

// sorted は、診断をソースの位置の順（同じ位置なら見つけた順）に並べて返す。
func (c *checker) sorted() []diag.Diagnostic {
	slices.SortStableFunc(c.diags, func(a, b diag.Diagnostic) int {
		return cmp.Or(
			cmp.Compare(a.Span.File, b.Span.File),
			cmp.Compare(a.Span.Start.Line, b.Span.Start.Line),
			cmp.Compare(a.Span.Start.Col, b.Span.Start.Col),
		)
	})
	return c.diags
}

// externs は、利用者のファイルの externs と extern box を BX070 にする。
func (c *checker) externs(f *syntax.File) {
	if f.Externs {
		d := c.report(diag.BX070, f.Name.Span, "", "標準の箱の宣言（externs）は、利用者のファイルには書けません")
		d.Hint = "ファイルは module 名前 で始めてください"
	}
	for _, decl := range f.Decls {
		if b, ok := decl.(*syntax.Box); ok && b.Extern {
			d := c.report(diag.BX070, b.Name.Span, b.Name.Name, "extern box は、利用者のファイルには書けません")
			d.Hint = "箱は box で定義し、body を書いてください"
		}
	}
}
