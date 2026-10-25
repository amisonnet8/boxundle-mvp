package std

import (
	"fmt"
	"maps"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/amisonnet8/boxundle-mvp/internal/syntax"
)

// parseDecls は、組み込んだ宣言をすべて構文解析する。
func parseDecls(t *testing.T) map[string]*syntax.File {
	t.Helper()
	files, err := DeclFiles()
	if err != nil {
		t.Fatal(err)
	}
	parsed := map[string]*syntax.File{}
	for _, df := range files {
		f, err := syntax.Parse(df.Name, df.Src)
		if err != nil {
			t.Fatalf("%s: %v", df.Name, err)
		}
		parsed[df.Name] = f
	}
	return parsed
}

// boxes は、宣言の箱を名前の順に返す。
func boxes(t *testing.T, parsed map[string]*syntax.File) []*syntax.Box {
	t.Helper()
	var all []*syntax.Box
	for _, name := range slices.Sorted(maps.Keys(parsed)) {
		for _, d := range parsed[name].Decls {
			b, ok := d.(*syntax.Box)
			if !ok {
				t.Fatalf("%s: 箱でない宣言 %T", name, d)
			}
			all = append(all, b)
		}
	}
	return all
}

func TestDeclFiles(t *testing.T) {
	files, err := DeclFiles()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range files {
		names = append(names, f.Name)
	}
	// docs/design/mvp.md §4.2 の decl/ の一覧（名前の順）
	want := []string{
		"decl/clock.bx", "decl/date.bx", "decl/env.bx", "decl/fs.bx", "decl/hash.bx",
		"decl/http.bx", "decl/math.bx", "decl/random.bx", "decl/re.bx", "decl/text.bx",
	}
	if !slices.Equal(names, want) {
		t.Errorf("names = %q, want %q", names, want)
	}
}

// Decls は、DeclFiles と同じ順に、構文解析した宣言を返す。
func TestDecls(t *testing.T) {
	files, err := DeclFiles()
	if err != nil {
		t.Fatal(err)
	}
	decls, err := Decls()
	if err != nil {
		t.Fatal(err)
	}
	if len(decls) != len(files) {
		t.Fatalf("Decls() は %d 件、DeclFiles() は %d 件", len(decls), len(files))
	}
	for i, f := range decls {
		ns := "std." + strings.TrimSuffix(path.Base(files[i].Name), ".bx")
		if !f.Externs || f.Name.Name != ns || f.Span.File != files[i].Name {
			t.Errorf("Decls()[%d] = externs %v %s（%s）, want %s（%s）", i, f.Externs, f.Name.Name, f.Span.File, ns, files[i].Name)
		}
	}
}

// 宣言のファイルが構文解析でき、名前空間・箱の名前・登録名がそろい、正規形で書かれている。
func TestDeclFilesParse(t *testing.T) {
	files, err := DeclFiles()
	if err != nil {
		t.Fatal(err)
	}
	for _, df := range files {
		f, err := syntax.Parse(df.Name, df.Src)
		if err != nil {
			t.Errorf("%s: %v", df.Name, err)
			continue
		}
		ns := "std." + strings.TrimSuffix(path.Base(df.Name), ".bx")
		if !f.Externs || f.Name.Name != ns {
			t.Errorf("%s: externs = %v, 名前空間 = %q, want externs %s", df.Name, f.Externs, f.Name.Name, ns)
		}
		if len(f.Decls) == 0 {
			t.Errorf("%s: 箱が無い", df.Name)
		}
		for _, d := range f.Decls {
			b, ok := d.(*syntax.Box)
			if !ok {
				t.Errorf("%s: 箱でない宣言 %T", df.Name, d)
				continue
			}
			if !b.Extern || !strings.HasPrefix(b.Name.Name, ns+".") {
				t.Errorf("%s: %s は %s の extern box でない", df.Name, b.Name.Name, ns)
			}
			if b.Impl == nil || "std."+b.Impl.Name.Name != b.Name.Name {
				t.Errorf("%s: %s の登録名が、std. を除いた箱の名前でない", df.Name, b.Name.Name)
			}
			if b.Label == nil {
				t.Errorf("%s: %s にラベルが無い", df.Name, b.Name.Name)
			}
		}
		if got := syntax.Print(f); string(got) != string(df.Src) {
			t.Errorf("%s: 正規形でない（fmt の結果）:\n%s", df.Name, got)
		}
	}
}

// 作用のない箱には、テストが1つ以上ある。テストの given・expect は、入力・出力のポートをすべて、宣言の順に書く。
// 作用のある箱は、テストではスタブが必須なので、宣言にテストを書かない。
func TestDeclTests(t *testing.T) {
	for _, b := range boxes(t, parseDecls(t)) {
		pure := b.Perm != nil && b.Perm.None
		if pure && len(b.Tests) == 0 {
			t.Errorf("%s: 作用のない箱にテストが無い", b.Name.Name)
		}
		if !pure && len(b.Tests) > 0 {
			t.Errorf("%s: 作用のある箱にテストがある", b.Name.Name)
		}
		for _, tc := range b.Tests {
			if got, want := fieldNames(tc.Given), portNames(b.Ins); !slices.Equal(got, want) {
				t.Errorf("%s「%s」: given = %q, want %q", b.Name.Name, tc.Name.Name, got, want)
			}
			if got, want := fieldNames(tc.Expect), portNames(b.Outs); !slices.Equal(got, want) {
				t.Errorf("%s「%s」: expect = %q, want %q", b.Name.Name, tc.Name.Name, got, want)
			}
			if len(tc.Stubs) > 0 {
				t.Errorf("%s「%s」: スタブがある", b.Name.Name, tc.Name.Name)
			}
		}
	}
}

func fieldNames(r *syntax.RecordLit) []string {
	var names []string
	for _, f := range r.Fields {
		names = append(names, f.Name.Name)
	}
	return names
}

func portNames(ps []*syntax.Port) []string {
	var names []string
	for _, p := range ps {
		names = append(names, p.Name.Name)
	}
	return names
}

// 宣言の箱と、std.md §3〜§8 の表の箱が、名前・入出力・権限まで一致する。
func TestDeclsMatchStdDoc(t *testing.T) {
	doc := stdDocBoxes(t)
	got := map[string]string{}
	for _, b := range boxes(t, parseDecls(t)) {
		got[b.Name.Name] = signature(b)
	}
	for _, name := range slices.Sorted(maps.Keys(doc)) {
		if got[name] != doc[name] {
			t.Errorf("%s:\n  宣言   %q\n  std.md %q", name, got[name], doc[name])
		}
	}
	for _, name := range slices.Sorted(maps.Keys(got)) {
		if _, ok := doc[name]; !ok {
			t.Errorf("%s: std.md に無い箱が宣言されている", name)
		}
	}
}

// 宣言の名前空間と権限が、std.md §1 の表と一致する。
func TestDeclNamespacesMatchStdDoc(t *testing.T) {
	src, err := os.ReadFile("../../docs/reference/std.md")
	if err != nil {
		t.Fatal(err)
	}
	row := regexp.MustCompile("^\\| `(std\\.[a-z]+)` \\| ([^|]+) \\|")
	doc := map[string]string{}
	for line := range strings.Lines(string(src)) {
		if m := row.FindStringSubmatch(line); m != nil {
			doc[m[1]] = strings.TrimSpace(m[2])
		}
	}
	got := map[string]string{}
	for _, b := range boxes(t, parseDecls(t)) {
		ns := b.Name.Name[:strings.LastIndex(b.Name.Name, ".")]
		perm := permText(b.Perm)
		if prev, ok := got[ns]; ok && prev != perm {
			perm = prev + " / " + perm
		}
		got[ns] = perm
	}
	if fmt.Sprint(got) != fmt.Sprint(doc) {
		t.Errorf("名前空間と権限:\n  宣言   %v\n  std.md %v", got, doc)
	}
}

// signature は、箱の入出力と権限を std.md の表と比べられる形にする。
func signature(b *syntax.Box) string {
	return "in " + portsText(b.Ins) + " | out " + portsText(b.Outs) + " | perm " + permText(b.Perm)
}

func portsText(ps []*syntax.Port) string {
	if len(ps) == 0 {
		return "なし"
	}
	var parts []string
	for _, p := range ps {
		parts = append(parts, p.Name.Name+": "+typeText(p.Type))
	}
	return strings.Join(parts, ", ")
}

func permText(p *syntax.Perm) string {
	if p == nil {
		return "?"
	}
	if p.None {
		return "none"
	}
	var names []string
	for _, n := range p.Names {
		names = append(names, n.Name)
	}
	return strings.Join(names, ", ")
}

// typeText は、標準の箱が使う型（名前とリスト）を書く。
func typeText(ty syntax.Type) string {
	switch ty := ty.(type) {
	case *syntax.TypeName:
		return ty.Name
	case *syntax.ListType:
		return "[" + typeText(ty.Elem) + "]"
	}
	return fmt.Sprintf("%T", ty)
}

// stdDocBoxes は、std.md §3〜§8 の表から、箱の名前ごとの入出力と権限を読む。
// §3〜§7 の表は「箱・入力・出力」（権限は none）、§8 の表は「箱・権限・入力・出力」。
func stdDocBoxes(t *testing.T) map[string]string {
	t.Helper()
	src, err := os.ReadFile("../../docs/reference/std.md")
	if err != nil {
		t.Fatal(err)
	}
	name := regexp.MustCompile("`(std\\.[a-z_]+\\.[a-z_0-9]+)`")
	effects := false
	boxes := map[string]string{}
	for line := range strings.Lines(string(src)) {
		if strings.HasPrefix(line, "## ") {
			effects = strings.Contains(line, "作用のある箱")
			continue
		}
		if !strings.HasPrefix(line, "| `std.") {
			continue
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		names := name.FindAllStringSubmatch(cells[0], -1)
		if names == nil {
			continue // §1 の名前空間の表
		}
		perm, in, out := "none", cells[1], cells[2]
		if effects {
			perm, in, out = cells[1], cells[2], cells[3]
		}
		sig := "in " + docPorts(in) + " | out " + docPorts(out) + " | perm " + perm
		for _, m := range names {
			boxes[m[1]] = sig
		}
	}
	if len(boxes) == 0 {
		t.Fatal("std.md から箱を読めない")
	}
	return boxes
}

// docPorts は、表のポートの欄から、全角の括弧の補足を除く。
func docPorts(cell string) string {
	if i := strings.Index(cell, "（"); i >= 0 {
		cell = cell[:i]
	}
	return strings.TrimSpace(cell)
}
