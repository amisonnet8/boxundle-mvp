package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

// result は、1回の実行の終了コードと出力。
type result struct {
	code           int
	stdout, stderr string
}

func runArgs(args ...string) result {
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut)
	return result{code, out.String(), errOut.String()}
}

const help = `使い方: boxundle <コマンド> [引数...]

コマンド:
  fmt FILE [--check] [--write]  正規形で出力する
  help                          この一覧を出す
  version                       バージョンを出す
`

const fmtUsage = "使い方: boxundle fmt FILE [--check] [--write]\n"

// 正規形でないモジュールと、その正規形。
const (
	uglySrc = "module  m\ntype T=Int\n"
	niceSrc = "module m\n\ntype T = Int\n"
	badSrc  = "module m\n\nbox\n"
	badDiag = "エラー BX001\n  ここには箱の名前が要りますが、ファイルの終わりがあります\n  位置: bad.bx:4:1\n"
)

// inTempDir は、一時ディレクトリに files を置いて、そこへ移る。
func inTempDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	return dir
}

func check(t *testing.T, got, want result) {
	t.Helper()
	if got != want {
		t.Errorf("code = %d, want %d\nstdout:\n%s\nwant:\n%s\nstderr:\n%s\nwant:\n%s",
			got.code, want.code, got.stdout, want.stdout, got.stderr, want.stderr)
	}
}

func TestRunUsage(t *testing.T) {
	inTempDir(t, map[string]string{"nice.bx": niceSrc})
	if err := os.Mkdir("dir.bx", 0o755); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		args []string
		want result
	}{
		{"引数なし", nil, result{2, "", help}},
		{"help", []string{"help"}, result{0, help, ""}},
		{"-h", []string{"-h"}, result{0, help, ""}},
		{"--help", []string{"--help"}, result{0, help, ""}},
		{"未知のコマンド", []string{"frobnicate"},
			result{2, "", "boxundle: 未知のコマンド「frobnicate」です\n使い方: boxundle <コマンド> [引数...]（一覧は boxundle help）\n"}},
		{"FILE が無い", []string{"fmt"}, result{2, "", "boxundle: fmt には FILE が1つ要ります\n" + fmtUsage}},
		{"FILE が多い", []string{"fmt", "nice.bx", "nice.bx"}, result{2, "", "boxundle: fmt には FILE が1つ要ります\n" + fmtUsage}},
		{"未知のフラグ", []string{"fmt", "--x", "nice.bx"}, result{2, "", "boxundle: 未知のフラグ「--x」です\n" + fmtUsage}},
		{"1文字のフラグ", []string{"fmt", "nice.bx", "-c"}, result{2, "", "boxundle: 未知のフラグ「-c」です\n" + fmtUsage}},
		{"一緒に使えないフラグ", []string{"fmt", "--check", "--write", "nice.bx"},
			result{2, "", "boxundle: --check と --write は一緒に使えません\n" + fmtUsage}},
		{"ファイルが無い", []string{"fmt", "nothing.bx"},
			result{2, "", "boxundle: ファイル「nothing.bx」を読めません（見つかりません）\n" + fmtUsage}},
		{"- はフラグでなくファイル名", []string{"fmt", "-"},
			result{2, "", "boxundle: ファイル「-」を読めません（見つかりません）\n" + fmtUsage}},
		{"ディレクトリ", []string{"fmt", "dir.bx"},
			result{2, "", "boxundle: ファイル「dir.bx」を読めません（ディレクトリです）\n" + fmtUsage}},
		{"途中がファイル", []string{"fmt", "nice.bx/x.bx"},
			result{2, "", "boxundle: ファイル「nice.bx/x.bx」を読めません（" + notDirReason() + "）\n" + fmtUsage}},
		{"version に引数", []string{"version", "x"},
			result{2, "", "boxundle: version は引数を取りません\n使い方: boxundle version\n"}},
		{"version に未知のフラグ", []string{"version", "--json"},
			result{2, "", "boxundle: 未知のフラグ「--json」です\n使い方: boxundle version\n"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check(t, runArgs(tt.args...), tt.want)
		})
	}
}

// notDirReason は、途中がファイルのパスを読んだときの理由。Unix の ENOTDIR はどの理由にも当たらない。
func notDirReason() string {
	if runtime.GOOS == "windows" {
		return "見つかりません"
	}
	return "入出力の誤りです"
}

func TestRunPermission(t *testing.T) {
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("権限で読み書きを止められない")
	}
	inTempDir(t, map[string]string{"secret.bx": niceSrc, "ro.bx": uglySrc})
	if err := os.Chmod("secret.bx", 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod("ro.bx", 0o444); err != nil {
		t.Fatal(err)
	}
	check(t, runArgs("fmt", "secret.bx"),
		result{2, "", "boxundle: ファイル「secret.bx」を読めません（権限がありません）\n" + fmtUsage})
	check(t, runArgs("fmt", "--write", "ro.bx"),
		result{2, "", "boxundle: ファイル「ro.bx」に書けません（権限がありません）\n" + fmtUsage})
}

func TestFmt(t *testing.T) {
	inTempDir(t, map[string]string{"ugly.bx": uglySrc, "nice.bx": niceSrc, "bad.bx": badSrc})
	check(t, runArgs("fmt", "ugly.bx"), result{0, niceSrc, ""})
	check(t, runArgs("fmt", "nice.bx"), result{0, niceSrc, ""})
	check(t, runArgs("fmt", "ugly.bx", "--check"), result{1, "", "ugly.bx は正規形ではありません\n"})
	check(t, runArgs("fmt", "--check", "nice.bx"), result{0, "", ""})
	check(t, runArgs("fmt", "bad.bx"), result{1, "", badDiag})
	check(t, runArgs("fmt", "bad.bx", "--check"), result{1, "", badDiag})
	check(t, runArgs("fmt", "bad.bx", "--write"), result{1, "", badDiag})
	if got, _ := os.ReadFile("bad.bx"); string(got) != badSrc {
		t.Errorf("構文エラーのファイルを書き換えた: %q", got)
	}
	if got, _ := os.ReadFile("ugly.bx"); string(got) != uglySrc {
		t.Errorf("--write なしで書き換えた: %q", got)
	}
}

func TestFmtWrite(t *testing.T) {
	inTempDir(t, map[string]string{"ugly.bx": uglySrc, "nice.bx": niceSrc})
	old := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes("nice.bx", old, old); err != nil {
		t.Fatal(err)
	}
	check(t, runArgs("fmt", "--write", "ugly.bx"), result{0, "", ""})
	if got, _ := os.ReadFile("ugly.bx"); string(got) != niceSrc {
		t.Errorf("--write の結果 = %q, want %q", got, niceSrc)
	}
	check(t, runArgs("fmt", "nice.bx", "--write"), result{0, "", ""})
	info, err := os.Stat("nice.bx")
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(old) {
		t.Errorf("正規形のファイルを書き直した（更新時刻 %v）", info.ModTime())
	}
}

func TestRunInternal(t *testing.T) {
	cmds := map[string]command{
		"boom": {flags: []string{"--json"}, run: func(*call) int { panic("壊れた") }},
	}
	var out, errOut bytes.Buffer
	got := result{runWith(cmds, []string{"boom"}, &out, &errOut), out.String(), errOut.String()}
	check(t, got, result{3, "", "エラー BX-INTERNAL\n  boxundle 内部の不具合です（コマンド boom）\n  ヒント: 不具合として報告してください。詳細: 壊れた\n"})

	out.Reset()
	errOut.Reset()
	got = result{runWith(cmds, []string{"boom", "--json"}, &out, &errOut), out.String(), errOut.String()}
	check(t, got, result{3, `[
  {
    "code": "BX-INTERNAL",
    "severity": "error",
    "message": "boxundle 内部の不具合です（コマンド boom）",
    "hint": "不具合として報告してください。詳細: 壊れた"
  }
]
`, ""})
}

func TestVersionString(t *testing.T) {
	mod := func(v string) *debug.BuildInfo { return &debug.BuildInfo{Main: debug.Module{Version: v}} }
	tests := []struct {
		embedded string
		info     *debug.BuildInfo
		want     string
	}{
		{"v0.1.0", mod("v0.0.0-20261025074447-bb43f7f5f3ac+dirty"), "v0.1.0"},
		{"", mod("v0.2.0"), "v0.2.0"},
		{"", mod("(devel)"), "(devel)"},
		{"", mod(""), "(devel)"},
		{"", nil, "(devel)"},
	}
	for _, tt := range tests {
		if got := versionString(tt.embedded, tt.info); got != tt.want {
			t.Errorf("versionString(%q, %v) = %q, want %q", tt.embedded, tt.info, got, tt.want)
		}
	}
}
