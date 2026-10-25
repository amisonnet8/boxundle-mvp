package main

import "testing"

// 警告だけのモジュールと、エラーのあるモジュール。
const (
	warnSrc  = "module m\n\nbox B @\"b\" {\n  out $r : Str\n  perm clock\n  body {\n    emit $r \"x\"\n  }\n}\n"
	warnDiag = "警告 BX073  箱 B\n  宣言しているが使っていない権限 clock があります\n  位置: warn.bx:5:8\n  ヒント: 本体で呼ぶ箱は、どれもこの権限を使いません\n"
	warnJSON = `  {
    "code": "BX073",
    "severity": "warning",
    "message": "宣言しているが使っていない権限 clock があります",
    "box": "B",
    "span": {
      "file": "warn.bx",
      "start": {
        "line": 5,
        "col": 8
      },
      "end": {
        "line": 5,
        "col": 13
      }
    },
    "hint": "本体で呼ぶ箱は、どれもこの権限を使いません"
  }`
	errSrc  = "module m\n\nbox B @\"b\" {\n  in $x : Int\n  out $r : Int\n  perm none\n  body {\n    emit $r \"x\"\n  }\n}\n"
	errDiag = "エラー BX020  箱 B\n  emit $r の値の型が合いません（期待した型 Int、実際の型 Str）\n  位置: err.bx:8:13\n"
	errJSON = `  {
    "code": "BX020",
    "severity": "error",
    "message": "emit $r の値の型が合いません（期待した型 Int、実際の型 Str）",
    "box": "B",
    "span": {
      "file": "err.bx",
      "start": {
        "line": 8,
        "col": 13
      },
      "end": {
        "line": 8,
        "col": 16
      }
    }
  }`
)

func TestCheck(t *testing.T) {
	inTempDir(t, map[string]string{"nice.bx": niceSrc, "warn.bx": warnSrc, "err.bx": errSrc, "bad.bx": badSrc})
	tests := []struct {
		name string
		args []string
		want result
	}{
		{"診断が無ければ何も出さない", []string{"check", "nice.bx"}, result{0, "", ""}},
		{"診断が無ければ --json は []", []string{"check", "--json", "nice.bx"}, result{0, "[]\n", ""}},
		{"警告だけなら終了コード0", []string{"check", "warn.bx"}, result{0, "", warnDiag}},
		{"警告だけなら --json でも終了コード0", []string{"check", "warn.bx", "--json"}, result{0, "[\n" + warnJSON + "\n]\n", ""}},
		{"エラーがあれば終了コード1", []string{"check", "err.bx"}, result{1, "", errDiag}},
		{"構文エラー", []string{"check", "bad.bx"}, result{1, "", badDiag}},
		{"複数のファイルの診断を、引数の順にすべて出す", []string{"check", "err.bx", "nice.bx", "warn.bx"},
			result{1, "", errDiag + "\n" + warnDiag}},
		{"複数のファイルの診断を、1つの JSON の配列で出す", []string{"check", "warn.bx", "err.bx", "--json"},
			result{1, "[\n" + warnJSON + ",\n" + errJSON + "\n]\n", ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			check(t, runArgs(tt.args...), tt.want)
		})
	}
}
