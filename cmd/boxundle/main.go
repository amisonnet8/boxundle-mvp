// Command boxundle は、Boxundle の箱を検証・実行する（docs/reference/cli.md）。
package main

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/amisonnet8/boxundle-mvp/internal/diag"
)

// 終了コード（cli.md §1）。
const (
	exitOK       = 0 // 成功
	exitFailed   = 1 // 検証・実行・テストの失敗
	exitUsage    = 2 // 使い方の誤り
	exitInternal = 3 // 内部の不具合
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// command は1つのコマンド。
type command struct {
	usage string   // 名前のあとの引数（使い方の行と一覧に出す）
	about string   // 一覧に出す説明
	flags []string // 受け付ける真偽のフラグ（--check など）
	nargs int      // フラグでない引数の数
	run   func(c *call) int
}

// call は、コマンドの1回の呼び出し。
type call struct {
	name   string
	cmd    command
	args   []string        // フラグでない引数
	flags  map[string]bool // 付けられたフラグ
	stdout io.Writer
	stderr io.Writer
}

// usageError は使い方の誤りを、標準エラー出力の2行（cli.md §1.1）で知らせる。
func (c *call) usageError(format string, a ...any) int {
	_, _ = fmt.Fprintf(c.stderr, "boxundle: "+format+"\n", a...)
	_, _ = fmt.Fprintf(c.stderr, "使い方: %s\n", usageLine(c.name, c.cmd))
	return exitUsage
}

func usageLine(name string, cmd command) string {
	return strings.TrimSpace("boxundle " + name + " " + cmd.usage)
}

// commands は、コマンドの表を返す。テストが別の表を渡せるように、大域の変数にしない。
func commands() map[string]command {
	return map[string]command{
		"fmt": {
			usage: "FILE [--check] [--write]", about: "正規形で出力する",
			flags: []string{"--check", "--write"}, nargs: 1, run: runFmt,
		},
		"version": {about: "バージョンを出す", run: runVersion},
	}
}

func run(args []string, stdout, stderr io.Writer) int {
	return runWith(commands(), args, stdout, stderr)
}

// runWith は、コマンドの表 cmds で args を実行し、終了コードを返す。
func runWith(cmds map[string]command, args []string, stdout, stderr io.Writer) (code int) {
	if len(args) == 0 {
		writeHelp(stderr, cmds)
		return exitUsage
	}
	name := args[0]
	if name == "help" || name == "-h" || name == "--help" {
		writeHelp(stdout, cmds)
		return exitOK
	}
	cmd, ok := cmds[name]
	if !ok {
		_, _ = fmt.Fprintf(stderr, "boxundle: 未知のコマンド「%s」です\n", name)
		_, _ = fmt.Fprintln(stderr, "使い方: boxundle <コマンド> [引数...]（一覧は boxundle help）")
		return exitUsage
	}
	defer func() {
		if r := recover(); r != nil {
			code = reportInternal(stdout, stderr, name, slices.Contains(args, "--json"), r)
		}
	}()
	c := &call{name: name, cmd: cmd, flags: map[string]bool{}, stdout: stdout, stderr: stderr}
	for _, a := range args[1:] {
		switch {
		case strings.HasPrefix(a, "-") && a != "-":
			if !slices.Contains(cmd.flags, a) {
				return c.usageError("未知のフラグ「%s」です", a)
			}
			c.flags[a] = true
		default:
			c.args = append(c.args, a)
		}
	}
	if len(c.args) != cmd.nargs {
		if cmd.nargs == 0 {
			return c.usageError("%s は引数を取りません", name)
		}
		return c.usageError("%s には FILE が%dつ要ります", name, cmd.nargs)
	}
	return cmd.run(c)
}

// reportInternal は、panic を BX-INTERNAL の診断で報告する（cli.md §1.2）。
func reportInternal(stdout, stderr io.Writer, name string, asJSON bool, r any) int {
	d := diag.New(diag.BXInternal, fmt.Sprintf("boxundle 内部の不具合です（コマンド %s）", name))
	d.Hint = fmt.Sprintf("不具合として報告してください。詳細: %v", r)
	if asJSON {
		_ = diag.WriteJSON(stdout, []diag.Diagnostic{d})
	} else {
		_ = diag.WriteText(stderr, []diag.Diagnostic{d})
	}
	return exitInternal
}

// writeHelp は、コマンドの一覧を名前の順に書く。
func writeHelp(w io.Writer, cmds map[string]command) {
	lines := [][2]string{{"help", "この一覧を出す"}}
	for name, cmd := range cmds {
		lines = append(lines, [2]string{strings.TrimPrefix(usageLine(name, cmd), "boxundle "), cmd.about})
	}
	slices.SortFunc(lines, func(a, b [2]string) int { return strings.Compare(a[0], b[0]) })
	width := 0
	for _, l := range lines {
		width = max(width, len(l[0]))
	}
	_, _ = fmt.Fprintln(w, "使い方: boxundle <コマンド> [引数...]")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "コマンド:")
	for _, l := range lines {
		_, _ = fmt.Fprintf(w, "  %-*s  %s\n", width, l[0], l[1])
	}
}
