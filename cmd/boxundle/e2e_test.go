package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// buildBinary は、boxundle を、goreleaser と同じく -X main.version で版を埋めてビルドする。
func buildBinary(t *testing.T) string {
	t.Helper()
	name := "boxundle"
	if runtime.GOOS == "windows" {
		name += ".exe" // 付けないと、Windows はコマンドとして見つけない
	}
	bin := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("go", "build", "-ldflags", "-X main.version=v9.9.9", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

// runBinary は、dir でバイナリを動かし、終了コードと出力を返す。
func runBinary(t *testing.T, bin, dir string, args ...string) result {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("%v: %v", args, err)
		}
		code = exit.ExitCode()
	}
	return result{code, out.String(), errOut.String()}
}

func TestE2E(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	for name, body := range map[string]string{"ugly.bx": uglySrc, "bad.bx": badSrc} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	check(t, runBinary(t, bin, dir, "version"), result{0, "boxundle v9.9.9\n", ""})
	check(t, runBinary(t, bin, dir), result{2, "", help})
	check(t, runBinary(t, bin, dir, "help"), result{0, help, ""})
	check(t, runBinary(t, bin, dir, "frobnicate"),
		result{2, "", "boxundle: 未知のコマンド「frobnicate」です\n使い方: boxundle <コマンド> [引数...]（一覧は boxundle help）\n"})
	check(t, runBinary(t, bin, dir, "fmt", "nothing.bx"),
		result{2, "", "boxundle: ファイル「nothing.bx」を読めません（見つかりません）\n" + fmtUsage})
	check(t, runBinary(t, bin, dir, "fmt", "bad.bx"), result{1, "", badDiag})
	check(t, runBinary(t, bin, dir, "fmt", "ugly.bx", "--check"), result{1, "", "ugly.bx は正規形ではありません\n"})
	check(t, runBinary(t, bin, dir, "fmt", "ugly.bx"), result{0, niceSrc, ""})
	check(t, runBinary(t, bin, dir, "fmt", "--write", "ugly.bx"), result{0, "", ""})
	check(t, runBinary(t, bin, dir, "fmt", "--check", "ugly.bx"), result{0, "", ""})
}

// docs の例は正規形で書かれている（fmt --check が通り、fmt の出力が元と同じ）。
func TestE2EExamples(t *testing.T) {
	bin := buildBinary(t)
	examples := filepath.Join("..", "..", "docs", "examples")
	for _, name := range []string{"order.bx", "checkout.bx"} {
		src, err := os.ReadFile(filepath.Join(examples, name))
		if err != nil {
			t.Fatal(err)
		}
		check(t, runBinary(t, bin, examples, "fmt", "--check", name), result{0, "", ""})
		check(t, runBinary(t, bin, examples, "fmt", name), result{0, string(src), ""})
	}
}
