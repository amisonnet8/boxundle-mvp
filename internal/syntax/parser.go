package syntax

import (
	"fmt"
	"strings"

	"github.com/amisonnet8/boxundle-mvp/internal/diag"
)

// ops は language.md §4.3 の演算名。演算名はキーワードではないが、ノードの引数の並びで、
// 次のノードの始まりを見分けるのに使う（argStarts）。
var ops = map[string]bool{
	"val": true,
	"add": true, "sub": true, "mul": true, "div": true, "idiv": true, "mod": true,
	"neg": true, "abs": true, "min": true, "max": true,
	"to_dec": true, "floor": true, "round": true, "to_str": true,
	"eq": true, "ne": true, "lt": true, "le": true, "gt": true, "ge": true,
	"and": true, "or": true, "not": true,
	"concat": true, "len": true, "empty": true, "sum": true, "reverse": true,
	"take": true, "drop": true, "distinct": true, "range": true,
	"lookup": true, "has": true, "set": true, "remove": true, "keys": true, "values": true,
	"with": true,
	"map":  true, "flat_map": true, "filter": true, "any": true, "all": true,
	"fold": true, "sort_by": true, "group_by": true,
	"route": true, "match": true,
	"call": true,
}

// IsOp は、s が language.md §4.3 の演算名（call を含む）かを返す。
func IsOp(s string) bool { return ops[s] }

// Parse は src を構文解析する。モジュール（module）と標準の箱の宣言（externs）の両方を読む。
// 誤りは最初の1つを BX001 の diag.Diagnostic として返す。
func Parse(file string, src []byte) (*File, error) {
	toks, err := Lex(file, src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	return p.file()
}

type parser struct {
	toks []Token
	pos  int
	box  string // 今読んでいる箱の名前（誤りの box）
	node string // 今読んでいるノードのID（誤りの node）
}

// peek は今の位置から n 個先のトークン。終わりより先は EOF。
func (p *parser) peek(n int) Token {
	if i := p.pos + n; i < len(p.toks) {
		return p.toks[i]
	}
	return p.toks[len(p.toks)-1]
}

func (p *parser) next() Token {
	t := p.peek(0)
	if t.Kind != EOF {
		p.pos++
	}
	return t
}

func isKw(t Token, s string) bool { return t.Kind == Keyword && t.Text == s }

// spanFrom は start から、最後に読んだトークンまでの範囲。start を読んだ後で呼ぶ。
func (p *parser) spanFrom(start Token) diag.Span {
	end := p.toks[p.pos-1]
	return diag.Span{File: start.Span.File, Start: start.Span.Start, End: end.Span.End}
}

func (p *parser) fail(span diag.Span, format string, args ...any) error {
	d := diag.New(diag.BX001, fmt.Sprintf(format, args...))
	d.Span = span
	d.Box = p.box
	d.Node = p.node
	return d
}

// unexpected は、今のトークンの所に want が要る、という誤り。
func (p *parser) unexpected(want string) error {
	t := p.peek(0)
	return p.fail(t.Span, "ここには%sが要りますが、%sがあります", want, describe(t))
}

// describe は、誤りのメッセージに出すトークンの説明。
func describe(t Token) string {
	switch {
	case t.Kind == EOF:
		return "ファイルの終わり"
	case t.Kind.isPunct():
		return "「" + t.Text + "」"
	default:
		return t.Kind.String() + "「" + t.Text + "」"
	}
}

func (p *parser) expect(k Kind) (Token, error) {
	if p.peek(0).Kind != k {
		return Token{}, p.unexpected("「" + k.String() + "」")
	}
	return p.next(), nil
}

func (p *parser) expectKw(s string) (Token, error) {
	if !isKw(p.peek(0), s) {
		return Token{}, p.unexpected("「" + s + "」")
	}
	return p.next(), nil
}

// qname は修飾名（. を含んでよい名前）を読む。
func (p *parser) qname(what string) (Ident, error) {
	if p.peek(0).Kind != Name {
		return Ident{}, p.unexpected(what)
	}
	t := p.next()
	return Ident{Name: t.Text, Span: t.Span}, nil
}

// simpleName は . を含まない名前を読む。
func (p *parser) simpleName(what string) (Ident, error) {
	id, err := p.qname(what)
	if err != nil {
		return Ident{}, err
	}
	if strings.Contains(id.Name, ".") {
		return Ident{}, p.fail(id.Span, "%sに . は使えません", what)
	}
	return id, nil
}

// dollarName は $名前（フィールドなし）を読み、$ を除いた名前を返す。
func (p *parser) dollarName(what string) (Ident, error) {
	if p.peek(0).Kind != Dollar {
		return Ident{}, p.unexpected(what)
	}
	t := p.next()
	if len(t.Fields) > 0 {
		return Ident{}, p.fail(t.Span, "%sにフィールドは付けられません", what)
	}
	return Ident{Name: strings.TrimPrefix(t.Text, "$"), Span: t.Span}, nil
}

func (p *parser) label() *LabelLit {
	if p.peek(0).Kind != Label {
		return nil
	}
	t := p.next()
	return &LabelLit{Value: t.Value, Span: t.Span}
}

func (p *parser) file() (*File, error) {
	start := p.peek(0)
	f := &File{}
	var err error
	switch {
	case isKw(start, "module"):
		p.next()
		f.Name, err = p.simpleName("モジュールの名前")
	case isKw(start, "externs"):
		p.next()
		f.Externs = true
		f.Name, err = p.qname("名前空間")
	default:
		return nil, p.unexpected("「module」か「externs」")
	}
	if err != nil {
		return nil, err
	}
	for p.peek(0).Kind != EOF {
		var d Decl
		t := p.peek(0)
		switch {
		case isKw(t, "type"):
			d, err = p.typeDef(f.Externs)
		case isKw(t, "box") && f.Externs:
			return nil, p.fail(t.Span, "標準の箱の宣言（externs）には、extern box だけが書けます")
		case isKw(t, "box"):
			d, err = p.boxDecl(false)
		case isKw(t, "extern"):
			d, err = p.boxDecl(true)
		default:
			return nil, p.unexpected("「type」か「box」")
		}
		if err != nil {
			return nil, err
		}
		f.Decls = append(f.Decls, d)
	}
	f.Span = p.spanFrom(start)
	return f, nil
}

func (p *parser) typeDef(qualified bool) (*TypeDef, error) {
	start := p.next() // type
	var name Ident
	var err error
	if qualified {
		name, err = p.qname("型の名前")
	} else {
		name, err = p.simpleName("型の名前")
	}
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(Equals); err != nil {
		return nil, err
	}
	t, err := p.typ()
	if err != nil {
		return nil, err
	}
	return &TypeDef{Name: name, Type: t, Span: p.spanFrom(start)}, nil
}

func (p *parser) boxDecl(extern bool) (*Box, error) {
	start := p.next() // box か extern
	b := &Box{Extern: extern}
	var err error
	if extern {
		if _, err := p.expectKw("box"); err != nil {
			return nil, err
		}
		b.Name, err = p.qname("箱の名前")
	} else {
		b.Name, err = p.simpleName("箱の名前")
	}
	if err != nil {
		return nil, err
	}
	p.box = b.Name.Name
	defer func() { p.box = "" }()
	b.Label = p.label()
	if _, err := p.expect(LBrace); err != nil {
		return nil, err
	}
	for isKw(p.peek(0), "in") || isKw(p.peek(0), "out") {
		in := p.peek(0).Text == "in"
		port, err := p.port()
		if err != nil {
			return nil, err
		}
		if in {
			b.Ins = append(b.Ins, port)
		} else {
			b.Outs = append(b.Outs, port)
		}
	}
	if !isKw(p.peek(0), "perm") {
		return nil, p.unexpected("「in」「out」か「perm」")
	}
	if b.Perm, err = p.perm(); err != nil {
		return nil, err
	}
	for isKw(p.peek(0), "test") {
		t, err := p.test()
		if err != nil {
			return nil, err
		}
		b.Tests = append(b.Tests, t)
	}
	if extern {
		if !isKw(p.peek(0), "impl") {
			return nil, p.unexpected("「test」か「impl」")
		}
		if b.Impl, err = p.impl(); err != nil {
			return nil, err
		}
	} else {
		if !isKw(p.peek(0), "body") {
			return nil, p.unexpected("「test」か「body」")
		}
		p.next()
		if b.Body, err = p.block(); err != nil {
			return nil, err
		}
	}
	if _, err := p.expect(RBrace); err != nil {
		return nil, err
	}
	b.Span = p.spanFrom(start)
	return b, nil
}

func (p *parser) port() (*Port, error) {
	start := p.next() // in か out
	name, err := p.dollarName("ポート（$名前）")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(Colon); err != nil {
		return nil, err
	}
	t, err := p.typ()
	if err != nil {
		return nil, err
	}
	return &Port{Name: name, Type: t, Span: p.spanFrom(start)}, nil
}

func (p *parser) perm() (*Perm, error) {
	start := p.next() // perm
	if isKw(p.peek(0), "none") {
		p.next()
		return &Perm{None: true, Span: p.spanFrom(start)}, nil
	}
	pm := &Perm{}
	for {
		name, err := p.qname("「none」か権限の名前")
		if err != nil {
			return nil, err
		}
		pm.Names = append(pm.Names, name)
		if p.peek(0).Kind != Comma {
			break
		}
		p.next()
	}
	pm.Span = p.spanFrom(start)
	return pm, nil
}

func (p *parser) test() (*Test, error) {
	start := p.next() // test
	if p.peek(0).Kind != String {
		return nil, p.unexpected("テストの名前（文字列）")
	}
	nt := p.next()
	t := &Test{Name: Ident{Name: nt.Value, Span: nt.Span}}
	if _, err := p.expect(LBrace); err != nil {
		return nil, err
	}
	for isKw(p.peek(0), "stub") {
		s, err := p.stub()
		if err != nil {
			return nil, err
		}
		t.Stubs = append(t.Stubs, s)
	}
	if !isKw(p.peek(0), "given") {
		return nil, p.unexpected("「stub」か「given」")
	}
	p.next()
	var err error
	if t.Given, err = p.testValues(); err != nil {
		return nil, err
	}
	if _, err := p.expectKw("expect"); err != nil {
		return nil, err
	}
	if t.Expect, err = p.testValues(); err != nil {
		return nil, err
	}
	if _, err := p.expect(RBrace); err != nil {
		return nil, err
	}
	t.Span = p.spanFrom(start)
	return t, nil
}

func (p *parser) stub() (*Stub, error) {
	start := p.next() // stub
	name, err := p.qname("スタブにする箱の名前")
	if err != nil {
		return nil, err
	}
	outs, err := p.testValues()
	if err != nil {
		return nil, err
	}
	return &Stub{Box: name, Outputs: outs, Span: p.spanFrom(start)}, nil
}

// testValues は、テストとスタブの値 { ポート名: 値, … } を読む。値に参照は書けない。
func (p *parser) testValues() (*RecordLit, error) {
	start := p.peek(0)
	fs, err := p.fieldArgs()
	if err != nil {
		return nil, err
	}
	r := &RecordLit{Fields: fs, Span: p.spanFrom(start)}
	if ref := findRef(r); ref != nil {
		return nil, p.fail(ref.Pos(), "テストの値には参照（#…・$…）は書けません。書けるのはリテラルと構築だけです")
	}
	return r, nil
}

// findRef は、e の中で最初に出てくる参照を返す。無ければ nil。
func findRef(e Expr) Expr {
	switch e := e.(type) {
	case *NodeRef, *DollarRef:
		return e
	case *RecordLit:
		for _, f := range e.Fields {
			if r := findRef(f.Value); r != nil {
				return r
			}
		}
	case *ListLit:
		for _, x := range e.Elems {
			if r := findRef(x); r != nil {
				return r
			}
		}
	case *MapLit:
		for _, m := range e.Entries {
			if r := findRef(m.Key); r != nil {
				return r
			}
			if r := findRef(m.Value); r != nil {
				return r
			}
		}
	case *Annotated:
		return findRef(e.Value)
	}
	return nil
}

func (p *parser) impl() (*Impl, error) {
	start := p.next() // impl
	if t := p.peek(0); t.Kind != Name || t.Text != "builtin" {
		return nil, p.unexpected("「builtin」")
	}
	p.next()
	if p.peek(0).Kind != String {
		return nil, p.unexpected("実装の登録名（文字列）")
	}
	t := p.next()
	return &Impl{Name: Ident{Name: t.Value, Span: t.Span}, Span: p.spanFrom(start)}, nil
}

// typ は型を読む。| でつないだものは UnionType。
func (p *parser) typ() (Type, error) {
	start := p.peek(0)
	first, err := p.typeAtom()
	if err != nil {
		return nil, err
	}
	if p.peek(0).Kind != Pipe {
		return first, nil
	}
	u := &UnionType{Members: []Type{first}}
	for p.peek(0).Kind == Pipe {
		p.next()
		m, err := p.typeAtom()
		if err != nil {
			return nil, err
		}
		u.Members = append(u.Members, m)
	}
	u.Span = p.spanFrom(start)
	return u, nil
}

func (p *parser) typeAtom() (Type, error) {
	start := p.peek(0)
	switch start.Kind {
	case Name:
		p.next()
		return &TypeName{Name: start.Text, Span: start.Span}, nil
	case LBrack:
		p.next()
		elem, err := p.typ()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(RBrack); err != nil {
			return nil, err
		}
		return &ListType{Elem: elem, Span: p.spanFrom(start)}, nil
	case LBrace:
		p.next()
		if p.peek(0).Kind == Name && p.peek(1).Kind == Colon {
			return p.recordType(start)
		}
		key, err := p.typ()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(Arrow); err != nil {
			return nil, err
		}
		val, err := p.typ()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(RBrace); err != nil {
			return nil, err
		}
		return &MapType{Key: key, Value: val, Span: p.spanFrom(start)}, nil
	default:
		return nil, p.unexpected("型")
	}
}

// recordType は {名前: 型, …} を、開きの { を読んだ後から読む。
func (p *parser) recordType(start Token) (*RecordType, error) {
	r := &RecordType{}
	for {
		fstart := p.peek(0)
		name, err := p.simpleName("フィールドの名前")
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(Colon); err != nil {
			return nil, err
		}
		t, err := p.typ()
		if err != nil {
			return nil, err
		}
		r.Fields = append(r.Fields, &FieldType{Name: name, Type: t, Span: p.spanFrom(fstart)})
		if p.peek(0).Kind != Comma {
			break
		}
		p.next()
	}
	if _, err := p.expect(RBrace); err != nil {
		return nil, err
	}
	r.Span = p.spanFrom(start)
	return r, nil
}

// block は { ノードか emit の並び } を読む。
func (p *parser) block() (*Block, error) {
	start, err := p.expect(LBrace)
	if err != nil {
		return nil, err
	}
	b := &Block{}
	for {
		t := p.peek(0)
		var s Stmt
		switch {
		case t.Kind == RBrace:
			p.next()
			b.Span = p.spanFrom(start)
			return b, nil
		case t.Kind == NodeID:
			s, err = p.nodeStmt()
		case isKw(t, "emit"):
			s, err = p.emit()
		default:
			return nil, p.unexpected("ノードか「emit」か「}」")
		}
		if err != nil {
			return nil, err
		}
		b.Stmts = append(b.Stmts, s)
	}
}

func (p *parser) emit() (*Emit, error) {
	start := p.next() // emit
	port, err := p.dollarName("出力ポート（$名前）")
	if err != nil {
		return nil, err
	}
	v, err := p.arg()
	if err != nil {
		return nil, err
	}
	return &Emit{Port: port, Value: v, Span: p.spanFrom(start)}, nil
}

func (p *parser) nodeStmt() (*Node, error) {
	start := p.next() // ノードID
	if len(start.Fields) > 0 {
		return nil, p.fail(start.Span, "ノードの始まりのIDにフィールドは付けられません")
	}
	outer := p.node
	p.node = start.Text
	defer func() { p.node = outer }()
	n := &Node{ID: Ident{Name: start.Text, Span: start.Span}}
	var err error
	if n.Op, err = p.simpleName("演算名"); err != nil {
		return nil, err
	}
	if n.Op.Name == "call" {
		if n.Callee, err = p.qname("呼び出す箱の名前"); err != nil {
			return nil, err
		}
		if n.CallArgs, err = p.fieldArgs(); err != nil {
			return nil, err
		}
		n.Label = p.label()
		n.Span = p.spanFrom(start)
		return n, nil
	}
	branching := n.Op.Name == "route" || n.Op.Name == "match"
	for p.argStarts(branching) {
		a, err := p.arg()
		if err != nil {
			return nil, err
		}
		n.Args = append(n.Args, a)
	}
	if isKw(p.peek(0), "as") {
		if n.Binders, err = p.binders(); err != nil {
			return nil, err
		}
	}
	if p.peek(0).Kind == LBrace {
		if branching {
			n.Branches, err = p.branches()
		} else {
			n.Block, err = p.block()
		}
		if err != nil {
			return nil, err
		}
	}
	n.Label = p.label()
	n.Span = p.spanFrom(start)
	return n, nil
}

// argStarts は、今のトークンからノードの引数が始まるかを返す。
//   - { は、route・match では分岐の始まり。ほかの演算では引数（レコード・マップ）
//   - フィールドの無いノードIDの直後に名前があり、その名前が演算名か、名前の直後が { でなければ、
//     そのノードIDは次のノードの始まり。そうでなければ、参照の後に名前付きレコードの構築が続く形
func (p *parser) argStarts(branching bool) bool {
	t := p.peek(0)
	switch t.Kind {
	case Dollar, Int, Dec, String, LBrack, LParen:
		return true
	case Keyword:
		return t.Text == "true" || t.Text == "false"
	case LBrace:
		return !branching
	case Name:
		return p.peek(1).Kind == LBrace
	case NodeID:
		if len(t.Fields) > 0 {
			return true
		}
		n := p.peek(1)
		if n.Kind != Name {
			return true
		}
		return !ops[n.Text] && p.peek(2).Kind == LBrace
	default:
		return false
	}
}

func (p *parser) binders() ([]Ident, error) {
	p.next() // as
	if p.peek(0).Kind != LParen {
		b, err := p.dollarName("束縛子（$名前）か「(」")
		if err != nil {
			return nil, err
		}
		return []Ident{b}, nil
	}
	p.next()
	a, err := p.dollarName("束縛子（$名前）")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(Comma); err != nil {
		return nil, err
	}
	b, err := p.dollarName("束縛子（$名前）")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(RParen); err != nil {
		return nil, err
	}
	return []Ident{a, b}, nil
}

// branches は route・match の { 分岐 … } を読む。
func (p *parser) branches() ([]*Branch, error) {
	p.next() // {
	var bs []*Branch
	for p.peek(0).Kind != RBrace {
		t := p.peek(0)
		br := &Branch{}
		switch {
		case isKw(t, "true") || isKw(t, "false"):
			p.next()
			br.Cond = Ident{Name: t.Text, Span: t.Span}
		case t.Kind == Name:
			p.next()
			br.Cond = Ident{Name: t.Text, Span: t.Span}
			var err error
			if br.Binder, err = p.dollarName("束縛子（$名前）"); err != nil {
				return nil, err
			}
		default:
			return nil, p.unexpected("分岐（「true」「false」か、型の名前と束縛子）か「}」")
		}
		var err error
		if br.Block, err = p.block(); err != nil {
			return nil, err
		}
		br.Span = p.spanFrom(t)
		bs = append(bs, br)
	}
	p.next() // }
	return bs, nil
}

// fieldArgs は { [名前: 値, …] } を読む（call の引数、名前付きレコードの構築、テストの値）。
func (p *parser) fieldArgs() ([]*FieldArg, error) {
	if _, err := p.expect(LBrace); err != nil {
		return nil, err
	}
	if p.peek(0).Kind == RBrace {
		p.next()
		return nil, nil
	}
	return p.fieldArgsRest()
}

// fieldArgsRest は、1つ以上の 名前: 値 と閉じの } を読む。
func (p *parser) fieldArgsRest() ([]*FieldArg, error) {
	var fs []*FieldArg
	for {
		start := p.peek(0)
		name, err := p.simpleName("フィールドの名前")
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(Colon); err != nil {
			return nil, err
		}
		v, err := p.arg()
		if err != nil {
			return nil, err
		}
		fs = append(fs, &FieldArg{Name: name, Value: v, Span: p.spanFrom(start)})
		if p.peek(0).Kind != Comma {
			break
		}
		p.next()
	}
	if _, err := p.expect(RBrace); err != nil {
		return nil, err
	}
	return fs, nil
}

// arg は値を1つ読む：参照・リテラル・構築・型注釈。
func (p *parser) arg() (Expr, error) {
	t := p.peek(0)
	switch t.Kind {
	case NodeID:
		p.next()
		return &NodeRef{ID: t.Text, Fields: t.Fields, Span: t.Span}, nil
	case Dollar:
		p.next()
		return &DollarRef{Name: strings.TrimPrefix(t.Text, "$"), Fields: t.Fields, Span: t.Span}, nil
	case Int:
		p.next()
		return &IntLit{Text: t.Text, Span: t.Span}, nil
	case Dec:
		p.next()
		return &DecLit{Text: t.Text, Span: t.Span}, nil
	case String:
		p.next()
		return &StrLit{Value: t.Value, Span: t.Span}, nil
	case Keyword:
		if t.Text == "true" || t.Text == "false" {
			p.next()
			return &BoolLit{Value: t.Text == "true", Span: t.Span}, nil
		}
	case Name:
		p.next()
		fs, err := p.fieldArgs()
		if err != nil {
			return nil, err
		}
		return &RecordLit{Tag: Ident{Name: t.Text, Span: t.Span}, Fields: fs, Span: p.spanFrom(t)}, nil
	case LBrack:
		return p.listLit()
	case LBrace:
		return p.braceLit()
	case LParen:
		return p.annotated()
	}
	return nil, p.unexpected("値")
}

func (p *parser) listLit() (*ListLit, error) {
	start := p.next() // [
	l := &ListLit{}
	if p.peek(0).Kind != RBrack {
		for {
			x, err := p.arg()
			if err != nil {
				return nil, err
			}
			l.Elems = append(l.Elems, x)
			if p.peek(0).Kind != Comma {
				break
			}
			p.next()
		}
	}
	if _, err := p.expect(RBrack); err != nil {
		return nil, err
	}
	l.Span = p.spanFrom(start)
	return l, nil
}

// braceLit は { で始まる値を読む：無名レコード {名前: 値, …}、マップ {k -> v, …}、空マップ {->}。
func (p *parser) braceLit() (Expr, error) {
	start := p.next() // {
	switch {
	case p.peek(0).Kind == RBrace:
		p.next()
		return nil, p.fail(p.spanFrom(start), "値の位置に {} は書けません。空のマップは {->} と書きます")
	case p.peek(0).Kind == Arrow:
		p.next()
		if _, err := p.expect(RBrace); err != nil {
			return nil, err
		}
		return &MapLit{Span: p.spanFrom(start)}, nil
	case p.peek(0).Kind == Name && p.peek(1).Kind == Colon:
		fs, err := p.fieldArgsRest()
		if err != nil {
			return nil, err
		}
		return &RecordLit{Fields: fs, Span: p.spanFrom(start)}, nil
	}
	m := &MapLit{}
	for {
		estart := p.peek(0)
		k, err := p.arg()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(Arrow); err != nil {
			return nil, err
		}
		v, err := p.arg()
		if err != nil {
			return nil, err
		}
		m.Entries = append(m.Entries, &MapEntry{Key: k, Value: v, Span: p.spanFrom(estart)})
		if p.peek(0).Kind != Comma {
			break
		}
		p.next()
	}
	if _, err := p.expect(RBrace); err != nil {
		return nil, err
	}
	m.Span = p.spanFrom(start)
	return m, nil
}

func (p *parser) annotated() (*Annotated, error) {
	start := p.next() // (
	v, err := p.arg()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(Colon); err != nil {
		return nil, err
	}
	t, err := p.typ()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(RParen); err != nil {
		return nil, err
	}
	return &Annotated{Value: v, Type: t, Span: p.spanFrom(start)}, nil
}
