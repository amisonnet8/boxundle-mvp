// Package std は、標準の箱（宣言と Go の実装）を持つ。
package std

import (
	"embed"
	"path"
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
