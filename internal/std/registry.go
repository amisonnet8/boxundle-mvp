// Package std は、標準の箱（宣言と Go の実装）を持つ。
package std

import (
	"embed"
	"path"

	"github.com/amisonnet8/boxundle-mvp/internal/syntax"
)

//go:embed decl/*.bx
var declFS embed.FS

// DeclFile は、標準の箱の宣言のファイル1つ（名前空間1つ）。
type DeclFile struct {
	Name string // decl/text.bx
	Src  []byte
}

// DeclFiles は、組み込んだ宣言のファイルを、名前の順に返す。
func DeclFiles() ([]DeclFile, error) {
	entries, err := declFS.ReadDir("decl")
	if err != nil {
		return nil, err
	}
	files := make([]DeclFile, 0, len(entries))
	for _, e := range entries { // ReadDir は名前の順に返す
		name := path.Join("decl", e.Name())
		src, err := declFS.ReadFile(name)
		if err != nil {
			return nil, err
		}
		files = append(files, DeclFile{Name: name, Src: src})
	}
	return files, nil
}

// Decls は、組み込んだ宣言を構文解析して、名前の順に返す。
// 失敗は boxundle 自体の不具合（宣言はテストで構文解析できることを確かめている）。
func Decls() ([]*syntax.File, error) {
	files, err := DeclFiles()
	if err != nil {
		return nil, err
	}
	parsed := make([]*syntax.File, 0, len(files))
	for _, df := range files {
		f, err := syntax.Parse(df.Name, df.Src)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, f)
	}
	return parsed, nil
}
