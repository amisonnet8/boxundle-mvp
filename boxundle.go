// Package boxundle は、Boxundle のモジュールを検証・実行する公開APIを持つ（docs/design/mvp.md §4.3）。
package boxundle

import (
	"errors"
	"fmt"
	"os"

	"github.com/amisonnet8/boxundle-mvp/internal/check"
	"github.com/amisonnet8/boxundle-mvp/internal/diag"
	"github.com/amisonnet8/boxundle-mvp/internal/std"
	"github.com/amisonnet8/boxundle-mvp/internal/syntax"
)

// Diagnostic は1つの診断（language.md §6.1）。
type Diagnostic = diag.Diagnostic

// CheckError は、検証でエラーが見つかったときに LoadFile が返すエラー。
type CheckError struct {
	Diagnostics []Diagnostic // 見つかった診断すべて（警告も含む）を、位置の順に
}

func (e *CheckError) Error() string {
	var errs []Diagnostic
	for _, d := range e.Diagnostics {
		if d.Severity == diag.Error {
			errs = append(errs, d)
		}
	}
	if len(errs) == 0 {
		return "検証でエラーが見つかりました" // LoadFile は作らないが、利用者が作ったときのため
	}
	return fmt.Sprintf("検証で %d 件のエラーが見つかりました（最初: %s）", len(errs), errs[0].Error())
}

// Module は、検証を通ったモジュール。
type Module struct {
	file *syntax.File
	// Warnings は、検証で見つかった警告（BX073）。
	Warnings []Diagnostic
}

// LoadFile は、path のモジュールを読んで検証する。エラーがあれば *CheckError を返す。
// ファイルを読めないときは、os.ReadFile のエラーをそのまま返す。
func LoadFile(path string) (*Module, error) {
	src, err := os.ReadFile(path) //nolint:gosec // 利用者が指定したファイルを読むのが LoadFile の仕事
	if err != nil {
		return nil, err
	}
	f, ds := checkText(path, src)
	if diag.HasError(ds) {
		return nil, &CheckError{Diagnostics: ds}
	}
	return &Module{file: f, Warnings: ds}, nil
}

// CheckText は、src を filename のモジュールとして検証し、診断を位置の順に返す。
// 構文エラーがあれば、その BX001 だけを返す（language.md §5）。
func CheckText(filename string, src []byte) []Diagnostic {
	_, ds := checkText(filename, src)
	return ds
}

// checkText は、構文解析と検証を行う。構文エラーのときは、構文木を返さない。
func checkText(filename string, src []byte) (*syntax.File, []Diagnostic) {
	f, err := syntax.Parse(filename, src)
	if err != nil {
		var d diag.Diagnostic
		if !errors.As(err, &d) {
			panic(err) // Parse は診断だけを返す
		}
		return nil, []Diagnostic{d}
	}
	decls, err := std.Decls()
	if err != nil {
		panic(err) // 組み込んだ宣言はテストで構文解析できることを確かめている
	}
	return f, check.Check(f, decls)
}
