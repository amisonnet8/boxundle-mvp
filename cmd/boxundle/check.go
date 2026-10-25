package main

import (
	"os"

	"github.com/amisonnet8/boxundle-mvp"
	"github.com/amisonnet8/boxundle-mvp/internal/diag"
)

// runCheck は boxundle check FILE... [--json]。警告だけなら終了コード0。
func runCheck(c *call) int {
	srcs := make([][]byte, len(c.args))
	for i, file := range c.args { // 読めないファイルがあれば、どれも検証しない
		src, err := os.ReadFile(file) //nolint:gosec // 利用者が指定したファイルを読むのが check の仕事
		if err != nil {
			return c.usageError("ファイル「%s」を読めません（%s）", file, fileReason(file, err))
		}
		srcs[i] = src
	}
	var ds []diag.Diagnostic
	for i, file := range c.args { // 診断は、引数の順、ファイルの中では位置の順
		ds = append(ds, boxundle.CheckText(file, srcs[i])...)
	}
	if c.flags["--json"] {
		_ = diag.WriteJSON(c.stdout, ds)
	} else {
		_ = diag.WriteText(c.stderr, ds)
	}
	if diag.HasError(ds) {
		return exitFailed
	}
	return exitOK
}
