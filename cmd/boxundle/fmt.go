package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/amisonnet8/boxundle-mvp/internal/diag"
	"github.com/amisonnet8/boxundle-mvp/internal/syntax"
)

// runFmt は boxundle fmt FILE [--check] [--write]。
func runFmt(c *call) int {
	check, write := c.flags["--check"], c.flags["--write"]
	if check && write {
		return c.usageError("--check と --write は一緒に使えません")
	}
	file := c.args[0]
	src, err := os.ReadFile(file) //nolint:gosec // 利用者が指定したファイルを読むのが fmt の仕事
	if err != nil {
		return c.usageError("ファイル「%s」を読めません（%s）", file, fileReason(file, err))
	}
	f, err := syntax.Parse(file, src)
	if err != nil {
		var d diag.Diagnostic
		if !errors.As(err, &d) {
			panic(err) // Parse は診断だけを返す
		}
		_ = diag.WriteText(c.stderr, []diag.Diagnostic{d})
		return exitFailed
	}
	out := syntax.Print(f)
	switch {
	case check:
		if !bytes.Equal(out, src) {
			_, _ = fmt.Fprintf(c.stderr, "%s は正規形ではありません\n", file)
			return exitFailed
		}
	case write:
		if bytes.Equal(out, src) {
			return exitOK // 変わらないなら書かない（更新時刻を変えない）
		}
		// 権限は新しく作るときだけ使われる。すでにあるファイルの権限は変わらない
		if err := os.WriteFile(file, out, 0o600); err != nil { //nolint:gosec // 利用者が指定し、いま読んだファイルを書き直すのが --write の仕事
			return c.usageError("ファイル「%s」に書けません（%s）", file, fileReason(file, err))
		}
	default:
		_, _ = c.stdout.Write(out)
	}
	return exitOK
}

// fileReason は、ファイルを読み書きできなかった理由を、Go のエラー文を出さずに言う。
func fileReason(file string, err error) string {
	if info, serr := os.Stat(file); serr == nil && info.IsDir() {
		return "ディレクトリです" // OS によって、権限の誤りなどほかの誤りで返るため、先に見る
	}
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "見つかりません"
	case errors.Is(err, fs.ErrPermission):
		return "権限がありません"
	}
	return "入出力の誤りです"
}
