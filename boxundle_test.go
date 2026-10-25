package boxundle_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/amisonnet8/boxundle-mvp"
	"github.com/amisonnet8/boxundle-mvp/internal/diag"
)

// readExpected は、testdata/diag の期待する診断を読む。知らない項目があれば落とす（綴りの誤りで項目を見落とさないため）。
func readExpected(t *testing.T, path string) []boxundle.Diagnostic {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var ds []boxundle.Diagnostic
	if err := dec.Decode(&ds); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return ds
}

func diagText(t *testing.T, ds []boxundle.Diagnostic) string {
	t.Helper()
	var b bytes.Buffer
	if err := diag.WriteText(&b, ds); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// testdata/diag の .bx ごとに、同じ名前の .json の診断がそのまま返る。
func TestDiagFiles(t *testing.T) {
	inputs, err := filepath.Glob(filepath.Join("testdata", "diag", "*.bx"))
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, in := range inputs {
		name := filepath.Base(in)
		have[strings.TrimSuffix(name, ".bx")] = true
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(in)
			if err != nil {
				t.Fatal(err)
			}
			got := boxundle.CheckText(name, src)
			want := readExpected(t, strings.TrimSuffix(in, ".bx")+".json")
			if !reflect.DeepEqual(got, want) {
				t.Errorf("got:\n%s\nwant:\n%s", diagText(t, got), diagText(t, want))
			}
		})
	}
	// 検証のコード（BX001〜BX073）には、どれもそのコードの入力がある
	for _, code := range diag.AllCodes() {
		if !strings.HasPrefix(string(code), "BX-") && !have[string(code)] {
			t.Errorf("testdata/diag/%s.bx がありません", code)
		}
	}
}

// コードの名前の入力は、そのコードの診断だけを返す（ほかの誤りが混ざると、何を確かめた入力か分からなくなる）。
func TestDiagFilesOneCode(t *testing.T) {
	for _, code := range diag.AllCodes() {
		path := filepath.Join("testdata", "diag", string(code)+".bx")
		src, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue // 無いことは TestDiagFiles が知らせる。実行のコードには無い
		}
		if err != nil {
			t.Fatal(err)
		}
		ds := boxundle.CheckText(path, src)
		if len(ds) == 0 {
			t.Errorf("%s: 診断が返りません", path)
		}
		for _, d := range ds {
			if d.Code != code {
				t.Errorf("%s: %s が混ざっています\n%s", path, d.Code, diagText(t, ds))
			}
		}
	}
}

// docs の例は、診断なしで読み込める。
func TestLoadFileExamples(t *testing.T) {
	for _, name := range []string{"order.bx", "checkout.bx"} {
		m, err := boxundle.LoadFile(filepath.Join("docs", "examples", name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(m.Warnings) > 0 {
			t.Errorf("%s:\n%s", name, diagText(t, m.Warnings))
		}
	}
}

func TestLoadFile(t *testing.T) {
	t.Run("エラーがあれば CheckError で、警告も含めてすべての診断を持つ", func(t *testing.T) {
		path := filepath.Join("testdata", "diag", "multi.bx")
		m, err := boxundle.LoadFile(path)
		if m != nil {
			t.Error("エラーがあるのにモジュールが返りました")
		}
		var ce *boxundle.CheckError
		if !errors.As(err, &ce) {
			t.Fatalf("err = %v, want *CheckError", err)
		}
		var codes []diag.Code
		for _, d := range ce.Diagnostics {
			codes = append(codes, d.Code)
			if d.Span.File != path {
				t.Errorf("span.file = %q, want %q", d.Span.File, path)
			}
		}
		if want := []diag.Code{diag.BX002, diag.BX013, diag.BX020, diag.BX073, diag.BX071}; !reflect.DeepEqual(codes, want) {
			t.Errorf("codes = %v, want %v", codes, want)
		}
		if want := "検証で 4 件のエラーが見つかりました（最初: BX002: 型 Strr は定義されていません）"; err.Error() != want {
			t.Errorf("Error() = %q, want %q", err.Error(), want)
		}
	})
	t.Run("構文エラーも CheckError", func(t *testing.T) {
		_, err := boxundle.LoadFile(filepath.Join("testdata", "diag", "BX001.bx"))
		var ce *boxundle.CheckError
		if !errors.As(err, &ce) || len(ce.Diagnostics) != 1 || ce.Diagnostics[0].Code != diag.BX001 {
			t.Errorf("err = %v, want BX001 の CheckError", err)
		}
	})
	t.Run("警告だけなら読み込め、警告は Warnings にある", func(t *testing.T) {
		m, err := boxundle.LoadFile(filepath.Join("testdata", "diag", "BX073.bx"))
		if err != nil {
			t.Fatal(err)
		}
		if len(m.Warnings) != 1 || m.Warnings[0].Code != diag.BX073 {
			t.Errorf("Warnings:\n%s", diagText(t, m.Warnings))
		}
	})
	t.Run("ファイルを読めなければ、読んだときのエラー", func(t *testing.T) {
		_, err := boxundle.LoadFile(filepath.Join("testdata", "diag", "nothing.bx"))
		var ce *boxundle.CheckError
		if !errors.Is(err, fs.ErrNotExist) || errors.As(err, &ce) {
			t.Errorf("err = %v, want fs.ErrNotExist", err)
		}
	})
}

func TestCheckErrorWithoutErrors(t *testing.T) {
	e := &boxundle.CheckError{Diagnostics: []boxundle.Diagnostic{diag.New(diag.BX073, "w")}}
	if got, want := e.Error(), "検証でエラーが見つかりました"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
