package syntax

import "github.com/amisonnet8/boxundle-mvp/internal/diag"

// File は1つの .bx ファイル。モジュール（module）か、標準の箱の宣言（externs）。
type File struct {
	Externs bool   // externs で始まる（標準の箱の宣言）
	Name    Ident  // モジュールの名前、または名前空間（std.text）
	Decls   []Decl // 型定義（*TypeDef）と箱（*Box）。書かれた順
	Span    diag.Span
}

// Ident は、位置つきの名前。
type Ident struct {
	Name string
	Span diag.Span
}

// LabelLit は @"…"。Value は解読した中身。
type LabelLit struct {
	Value string
	Span  diag.Span
}

// Decl は、ファイルの中の型定義（*TypeDef）か箱（*Box）。
type Decl interface {
	Pos() diag.Span
	decl()
}

// TypeDef は type 名前 = 型。
type TypeDef struct {
	Name Ident // externs では修飾名（std.http.Response）
	Type Type
	Span diag.Span
}

// Box は box（利用者の箱）か extern box（標準の箱の宣言）。
// ポートは in と out を分けて、それぞれ書かれた順に持つ（正規形は in → out の順に並べるため）。
type Box struct {
	Extern bool  // extern box
	Name   Ident // extern box では修飾名
	Label  *LabelLit
	Ins    []*Port
	Outs   []*Port
	Perm   *Perm
	Tests  []*Test
	Body   *Block // box だけ
	Impl   *Impl  // extern box だけ
	Span   diag.Span
}

// Port は in $名前 : 型、または out $名前 : 型。
type Port struct {
	Name Ident // $ を除いた名前
	Type Type
	Span diag.Span
}

// Perm は perm none、または perm 権限名, …。
type Perm struct {
	None  bool
	Names []Ident
	Span  diag.Span
}

// Test は test "名前" { stub … given … expect … }。
type Test struct {
	Name   Ident // 解読した文字列
	Stubs  []*Stub
	Given  *RecordLit
	Expect *RecordLit
	Span   diag.Span
}

// Stub は stub 箱の名前 { 出力ポート名: 値, … }。
type Stub struct {
	Box     Ident
	Outputs *RecordLit
	Span    diag.Span
}

// Impl は impl builtin "登録名"。
type Impl struct {
	Name Ident // 解読した登録名
	Span diag.Span
}

// Type は型の式。
type Type interface {
	Pos() diag.Span
	typ()
}

// TypeName は Int・Dec・Bool・Str や、定義した型の名前（修飾名も）。
type TypeName struct {
	Name string
	Span diag.Span
}

// ListType は [T]。
type ListType struct {
	Elem Type
	Span diag.Span
}

// MapType は {K -> V}。
type MapType struct {
	Key, Value Type
	Span       diag.Span
}

// RecordType は {名前: 型, …}。
type RecordType struct {
	Fields []*FieldType
	Span   diag.Span
}

// FieldType は、レコード型の1つのフィールド。
type FieldType struct {
	Name Ident
	Type Type
	Span diag.Span
}

// UnionType は A | B | …（2つ以上）。
type UnionType struct {
	Members []Type
	Span    diag.Span
}

// Block は { … }。値を持つか、出力を確定させるノードの並び。
type Block struct {
	Stmts []Stmt
	Span  diag.Span
}

// Stmt はブロックの中の1つ。*Node か *Emit。
type Stmt interface {
	Pos() diag.Span
	stmt()
}

// Node は本体の中の1つの処理。
// call のときは Callee と CallArgs を持ち、Args・Binders・Block・Branches は持たない。
type Node struct {
	ID       Ident // #4.1
	Op       Ident // 演算名（未知の名前も、そのまま持つ）
	Args     []Expr
	Callee   Ident // call の呼び出し先
	CallArgs []*FieldArg
	Binders  []Ident // as $e は1つ、as ($acc, $e) は2つ。$ を除いた名前
	Block    *Block
	Branches []*Branch // route・match の分岐
	Label    *LabelLit
	Span     diag.Span
}

// Branch は route・match の分岐の1つ。
// route は Cond が true か false で Binder を持たない。match は Cond が型の名前で Binder を持つ。
type Branch struct {
	Cond   Ident
	Binder Ident // $ を除いた名前。無ければ Name が空
	Block  *Block
	Span   diag.Span
}

// Emit は emit $出力ポート 値。
type Emit struct {
	Port  Ident // $ を除いた名前
	Value Expr
	Span  diag.Span
}

// Expr はノードの引数・値。
type Expr interface {
	Pos() diag.Span
	expr()
}

// NodeRef は #4.1 や #3.result。
type NodeRef struct {
	ID     string // #4.1
	Fields []string
	Span   diag.Span
}

// DollarRef は $order や $e.product（ポートか束縛子）。
type DollarRef struct {
	Name   string // $ を除いた名前
	Fields []string
	Span   diag.Span
}

// IntLit は整数。Text は書かれたとおり（-12）。
type IntLit struct {
	Text string
	Span diag.Span
}

// DecLit は小数。Text は書かれたとおり（1.50）。
type DecLit struct {
	Text string
	Span diag.Span
}

// StrLit は文字列。Value は解読した中身。
type StrLit struct {
	Value string
	Span  diag.Span
}

// BoolLit は true・false。
type BoolLit struct {
	Value bool
	Span  diag.Span
}

// RecordLit はレコードの構築。名前付きなら Tag を持ち、無名なら Tag.Name が空。
// テストの given・expect、スタブの出力も、Tag の無いこの形で持つ（キーはポート名）。
type RecordLit struct {
	Tag    Ident
	Fields []*FieldArg
	Span   diag.Span
}

// FieldArg は 名前: 値。
type FieldArg struct {
	Name  Ident
	Value Expr
	Span  diag.Span
}

// ListLit は [a, b, …]。
type ListLit struct {
	Elems []Expr
	Span  diag.Span
}

// MapLit は {k -> v, …}。空マップ {->} は Entries が無い。
type MapLit struct {
	Entries []*MapEntry
	Span    diag.Span
}

// MapEntry は k -> v。
type MapEntry struct {
	Key, Value Expr
	Span       diag.Span
}

// Annotated は (値 : 型)。
type Annotated struct {
	Value Expr
	Type  Type
	Span  diag.Span
}

func (d *TypeDef) Pos() diag.Span { return d.Span }
func (b *Box) Pos() diag.Span     { return b.Span }

func (t *TypeName) Pos() diag.Span   { return t.Span }
func (t *ListType) Pos() diag.Span   { return t.Span }
func (t *MapType) Pos() diag.Span    { return t.Span }
func (t *RecordType) Pos() diag.Span { return t.Span }
func (t *UnionType) Pos() diag.Span  { return t.Span }

func (n *Node) Pos() diag.Span { return n.Span }
func (e *Emit) Pos() diag.Span { return e.Span }

func (e *NodeRef) Pos() diag.Span   { return e.Span }
func (e *DollarRef) Pos() diag.Span { return e.Span }
func (e *IntLit) Pos() diag.Span    { return e.Span }
func (e *DecLit) Pos() diag.Span    { return e.Span }
func (e *StrLit) Pos() diag.Span    { return e.Span }
func (e *BoolLit) Pos() diag.Span   { return e.Span }
func (e *RecordLit) Pos() diag.Span { return e.Span }
func (e *ListLit) Pos() diag.Span   { return e.Span }
func (e *MapLit) Pos() diag.Span    { return e.Span }
func (e *Annotated) Pos() diag.Span { return e.Span }

func (*TypeDef) decl() {}
func (*Box) decl()     {}

func (*TypeName) typ()   {}
func (*ListType) typ()   {}
func (*MapType) typ()    {}
func (*RecordType) typ() {}
func (*UnionType) typ()  {}

func (*Node) stmt() {}
func (*Emit) stmt() {}

func (*NodeRef) expr()   {}
func (*DollarRef) expr() {}
func (*IntLit) expr()    {}
func (*DecLit) expr()    {}
func (*StrLit) expr()    {}
func (*BoolLit) expr()   {}
func (*RecordLit) expr() {}
func (*ListLit) expr()   {}
func (*MapLit) expr()    {}
func (*Annotated) expr() {}
