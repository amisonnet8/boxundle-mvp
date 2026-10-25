package syntax

import (
	"fmt"
	"strings"
)

// Print は f を正規形のテキストにする（cli.md §2）。改行は LF。
func Print(f *File) []byte {
	p := &printer{}
	p.file(f)
	return []byte(p.b.String())
}

type printer struct {
	b      strings.Builder
	indent int
}

// line は、今の字下げで s を1行書く。s が空なら空行。
func (p *printer) line(s string) {
	if s != "" {
		p.b.WriteString(strings.Repeat("  ", p.indent))
		p.b.WriteString(s)
	}
	p.b.WriteByte('\n')
}

func (p *printer) file(f *File) {
	if f.Externs {
		p.line("externs " + f.Name.Name)
	} else {
		p.line("module " + f.Name.Name)
	}
	for _, d := range f.Decls {
		p.line("")
		switch d := d.(type) {
		case *TypeDef:
			p.line("type " + d.Name.Name + " = " + typeText(d.Type))
		case *Box:
			p.box(d)
		}
	}
}

func (p *printer) box(b *Box) {
	head := "box " + b.Name.Name
	if b.Extern {
		head = "extern " + head
	}
	p.line(head + labelText(b.Label) + " {")
	p.indent++
	for _, port := range b.Ins {
		p.line("in $" + port.Name.Name + " : " + typeText(port.Type))
	}
	for _, port := range b.Outs {
		p.line("out $" + port.Name.Name + " : " + typeText(port.Type))
	}
	p.line(permText(b.Perm))
	for _, t := range b.Tests {
		p.line("")
		p.test(t)
	}
	if len(b.Tests) > 0 {
		p.line("")
	}
	if b.Body != nil {
		p.block("body ", b.Body, "")
	}
	if b.Impl != nil {
		p.line("impl builtin " + quote(b.Impl.Name.Name))
	}
	p.indent--
	p.line("}")
}

func permText(pm *Perm) string {
	if pm.None {
		return "perm none"
	}
	names := make([]string, len(pm.Names))
	for i, n := range pm.Names {
		names[i] = n.Name
	}
	return "perm " + strings.Join(names, ", ")
}

func (p *printer) test(t *Test) {
	p.line("test " + quote(t.Name.Name) + " {")
	p.indent++
	for _, s := range t.Stubs {
		p.line("stub " + s.Box.Name + " " + stubText(s.Outputs))
	}
	p.testValues("given", t.Given)
	p.testValues("expect", t.Expect)
	p.indent--
	p.line("}")
}

// testValues は given・expect を、1ポート1行で書く（q 50b1ae0687）。
func (p *printer) testValues(kw string, r *RecordLit) {
	if len(r.Fields) == 0 {
		p.line(kw + " {}")
		return
	}
	p.line(kw + " {")
	p.indent++
	for i, f := range r.Fields {
		s := f.Name.Name + ": " + exprText(f.Value)
		if i < len(r.Fields)-1 {
			s += ","
		}
		p.line(s)
	}
	p.indent--
	p.line("}")
}

// stubText はスタブの出力を1行で書く：{ now: "…" }。
func stubText(r *RecordLit) string {
	if len(r.Fields) == 0 {
		return "{}"
	}
	return "{ " + fieldsText(r.Fields) + " }"
}

// block は、prefix に続けて { を書き、中身を1段下げて書き、} に suffix を続ける。
func (p *printer) block(prefix string, b *Block, suffix string) {
	p.line(prefix + "{")
	p.indent++
	for _, s := range b.Stmts {
		switch s := s.(type) {
		case *Node:
			p.node(s)
		case *Emit:
			p.line("emit $" + s.Port.Name + " " + exprText(s.Value))
		}
	}
	p.indent--
	p.line("}" + suffix)
}

func (p *printer) node(n *Node) {
	head := n.ID.Name + " " + n.Op.Name
	if n.Op.Name == "call" {
		head += " " + n.Callee.Name + " {" + fieldsText(n.CallArgs) + "}"
	}
	for _, a := range n.Args {
		head += " " + exprText(a)
	}
	switch len(n.Binders) {
	case 1:
		head += " as $" + n.Binders[0].Name
	case 2:
		head += " as ($" + n.Binders[0].Name + ", $" + n.Binders[1].Name + ")"
	}
	label := labelText(n.Label)
	switch {
	case n.Block != nil:
		p.block(head+" ", n.Block, label)
	case n.Branches != nil:
		p.line(head + " {")
		p.indent++
		for _, br := range n.Branches {
			cond := br.Cond.Name
			if br.Binder.Name != "" {
				cond += " $" + br.Binder.Name
			}
			p.block(cond+" ", br.Block, "")
		}
		p.indent--
		p.line("}" + label)
	default:
		p.line(head + label)
	}
}

func labelText(l *LabelLit) string {
	if l == nil {
		return ""
	}
	return " @" + quote(l.Value)
}

func typeText(t Type) string {
	switch t := t.(type) {
	case *TypeName:
		return t.Name
	case *ListType:
		return "[" + typeText(t.Elem) + "]"
	case *MapType:
		return "{" + typeText(t.Key) + " -> " + typeText(t.Value) + "}"
	case *RecordType:
		fs := make([]string, len(t.Fields))
		for i, f := range t.Fields {
			fs[i] = f.Name.Name + ": " + typeText(f.Type)
		}
		return "{" + strings.Join(fs, ", ") + "}"
	case *UnionType:
		ms := make([]string, len(t.Members))
		for i, m := range t.Members {
			ms[i] = typeText(m)
		}
		return strings.Join(ms, " | ")
	}
	panic(fmt.Sprintf("syntax: 未知の型の式 %T", t))
}

func fieldsText(fs []*FieldArg) string {
	ss := make([]string, len(fs))
	for i, f := range fs {
		ss[i] = f.Name.Name + ": " + exprText(f.Value)
	}
	return strings.Join(ss, ", ")
}

func pathText(fields []string) string {
	s := ""
	for _, f := range fields {
		s += "." + f
	}
	return s
}

func exprText(e Expr) string {
	switch e := e.(type) {
	case *NodeRef:
		return e.ID + pathText(e.Fields)
	case *DollarRef:
		return "$" + e.Name + pathText(e.Fields)
	case *IntLit:
		return e.Text
	case *DecLit:
		return e.Text
	case *StrLit:
		return quote(e.Value)
	case *BoolLit:
		return fmt.Sprint(e.Value)
	case *RecordLit:
		return e.Tag.Name + "{" + fieldsText(e.Fields) + "}"
	case *ListLit:
		es := make([]string, len(e.Elems))
		for i, x := range e.Elems {
			es[i] = exprText(x)
		}
		return "[" + strings.Join(es, ", ") + "]"
	case *MapLit:
		if len(e.Entries) == 0 {
			return "{->}"
		}
		es := make([]string, len(e.Entries))
		for i, m := range e.Entries {
			es[i] = exprText(m.Key) + " -> " + exprText(m.Value)
		}
		return "{" + strings.Join(es, ", ") + "}"
	case *Annotated:
		return "(" + exprText(e.Value) + " : " + typeText(e.Type) + ")"
	}
	panic(fmt.Sprintf("syntax: 未知の値の式 %T", e))
}

// quote は文字列を正規形の "…" にする。" と \ と改行・タブは \" \\ \n \t、
// そのほかの制御文字（U+0000〜U+001F・U+007F〜U+009F）と U+2028・U+2029 は、
// 16進数4桁（大文字）のエスケープにする。それ以外の文字はそのまま。
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || (0x7F <= r && r <= 0x9F) || r == 0x2028 || r == 0x2029:
			fmt.Fprintf(&b, "\\"+"u%04X", r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
