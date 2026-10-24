package diag

import (
	"bytes"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// language.md §6.1 の例。
func example() Diagnostic {
	d := New(BX022, "レコード型 Line にフィールド prodcut はありません")
	d.Box = "B7"
	d.Node = "#1.1"
	d.Path = "$e.prodcut"
	d.Span = Span{File: "order.bx", Start: Pos{Line: 14, Col: 28}, End: Pos{Line: 14, Col: 38}}
	d.Hint = "フィールド名の綴りを確認してください"
	d.Suggestions = []string{"product"}
	return d
}

func TestNewSeverity(t *testing.T) {
	for _, c := range AllCodes() {
		want := Error
		if c == BX073 {
			want = Warning
		}
		if got := New(c, "x").Severity; got != want {
			t.Errorf("New(%s).Severity = %q, want %q", c, got, want)
		}
	}
}

// コードの一覧は、仕様の正（language.md §6.2 の表）と同じ並びで一致すること。
func TestAllCodesMatchSpec(t *testing.T) {
	src, err := os.ReadFile("../../docs/reference/language.md")
	if err != nil {
		t.Fatal(err)
	}
	row := regexp.MustCompile(`^\| (BX[-A-Z0-9]+) \|`)
	var want []string
	in := false
	for line := range strings.SplitSeq(string(src), "\n") {
		if strings.HasPrefix(line, "### ") {
			in = line == "### 6.2 コード一覧"
			continue
		}
		if m := row.FindStringSubmatch(line); in && m != nil {
			want = append(want, m[1])
		}
	}
	var got []string
	for _, c := range AllCodes() {
		got = append(got, string(c))
	}
	if !slices.Equal(got, want) {
		t.Errorf("AllCodes() = %v\nwant (language.md §6.2) %v", got, want)
	}
}

func TestAllCodesIsACopy(t *testing.T) {
	a := AllCodes()
	a[0] = "changed"
	if AllCodes()[0] != BX001 {
		t.Error("AllCodes() が内部の一覧を返している")
	}
}

func TestError(t *testing.T) {
	var err error = New(BXRun001, "ゼロで割りました")
	if got, want := err.Error(), "BX-RUN-001: ゼロで割りました"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestHasError(t *testing.T) {
	warn := New(BX073, "w")
	tests := []struct {
		name string
		ds   []Diagnostic
		want bool
	}{
		{"空", nil, false},
		{"警告だけ", []Diagnostic{warn}, false},
		{"エラーを含む", []Diagnostic{warn, New(BX001, "e")}, true},
	}
	for _, tt := range tests {
		if got := HasError(tt.ds); got != tt.want {
			t.Errorf("%s: HasError = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestWriteJSON(t *testing.T) {
	// language.md §6.1 の例を jq '[.]' で整形したもの。
	want := `[
  {
    "code": "BX022",
    "severity": "error",
    "message": "レコード型 Line にフィールド prodcut はありません",
    "box": "B7",
    "node": "#1.1",
    "path": "$e.prodcut",
    "span": {
      "file": "order.bx",
      "start": {
        "line": 14,
        "col": 28
      },
      "end": {
        "line": 14,
        "col": 38
      }
    },
    "hint": "フィールド名の綴りを確認してください",
    "suggestions": [
      "product"
    ]
  }
]
`
	var b bytes.Buffer
	if err := WriteJSON(&b, []Diagnostic{example()}); err != nil {
		t.Fatal(err)
	}
	if b.String() != want {
		t.Errorf("WriteJSON =\n%s\nwant\n%s", b.String(), want)
	}
}

func TestWriteJSONOmitsAbsent(t *testing.T) {
	want := `[
  {
    "code": "BX-RUN-020",
    "severity": "error",
    "message": "a<b && c>d"
  }
]
`
	var b bytes.Buffer
	if err := WriteJSON(&b, []Diagnostic{New(BXRun020, "a<b && c>d")}); err != nil {
		t.Fatal(err)
	}
	if b.String() != want {
		t.Errorf("WriteJSON =\n%s\nwant\n%s", b.String(), want)
	}
}

func TestWriteJSONEmpty(t *testing.T) {
	for _, ds := range [][]Diagnostic{nil, {}} {
		var b bytes.Buffer
		if err := WriteJSON(&b, ds); err != nil {
			t.Fatal(err)
		}
		if got := b.String(); got != "[]\n" {
			t.Errorf("WriteJSON(%#v) = %q, want %q", ds, got, "[]\n")
		}
	}
}

func TestWriteText(t *testing.T) {
	run := New(BXRun020, "許可されていない権限が必要です: clock")
	run.Hint = "--allow clock で許可してください"
	// language.md §6.1 の人間向けの形の例。
	want := `エラー BX022  箱 B7  ノード #1.1  $e.prodcut
  レコード型 Line にフィールド prodcut はありません
  位置: order.bx:14:28
  ヒント: フィールド名の綴りを確認してください
  候補: product

エラー BX-RUN-020
  許可されていない権限が必要です: clock
  ヒント: --allow clock で許可してください
`
	var b bytes.Buffer
	if err := WriteText(&b, []Diagnostic{example(), run}); err != nil {
		t.Fatal(err)
	}
	if b.String() != want {
		t.Errorf("WriteText =\n%s\nwant\n%s", b.String(), want)
	}
}

func TestTextParts(t *testing.T) {
	warn := New(BX073, "宣言しているが使っていない権限 net があります")
	warn.Box = "Checkout"
	warn.Span = Span{File: "checkout.bx", Start: Pos{Line: 92, Col: 3}, End: Pos{Line: 92, Col: 13}}

	multi := New(BXTest001, "期待した値と違います\n- 100\n+ 250")
	multi.Box = "B7"
	multi.Suggestions = []string{"product", "products"}

	nofile := New(BX001, "閉じていない文字列です")
	nofile.Span = Span{Start: Pos{Line: 1, Col: 5}, End: Pos{Line: 1, Col: 6}}

	tests := []struct {
		name string
		d    Diagnostic
		want string
	}{
		{"警告・ノードなし", warn, "警告 BX073  箱 Checkout\n  宣言しているが使っていない権限 net があります\n  位置: checkout.bx:92:3\n"},
		{"複数行の message と複数の候補", multi, "エラー BX-TEST-001  箱 B7\n  期待した値と違います\n  - 100\n  + 250\n  候補: product, products\n"},
		{"ファイル名のない span", nofile, "エラー BX001\n  閉じていない文字列です\n  位置: 1:5\n"},
	}
	for _, tt := range tests {
		if got := Text(tt.d); got != tt.want {
			t.Errorf("%s: Text =\n%q\nwant\n%q", tt.name, got, tt.want)
		}
	}
}

func TestWriteTextEmpty(t *testing.T) {
	var b bytes.Buffer
	if err := WriteText(&b, nil); err != nil {
		t.Fatal(err)
	}
	if b.Len() != 0 {
		t.Errorf("WriteText(nil) = %q, want empty", b.String())
	}
}
