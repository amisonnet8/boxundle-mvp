// Package diag は、boxundle が利用者に返す診断（language.md §6）を表す。
// 構文・検証・実行・テストのどの段階のエラーも、この型で返す。
package diag

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Severity は診断の重さ。
type Severity string

const (
	Error   Severity = "error"
	Warning Severity = "warning"
)

// Code は診断のコード（language.md §6.2）。
type Code string

const (
	BX001      Code = "BX001"
	BX002      Code = "BX002"
	BX003      Code = "BX003"
	BX004      Code = "BX004"
	BX005      Code = "BX005"
	BX006      Code = "BX006"
	BX010      Code = "BX010"
	BX011      Code = "BX011"
	BX012      Code = "BX012"
	BX013      Code = "BX013"
	BX014      Code = "BX014"
	BX015      Code = "BX015"
	BX020      Code = "BX020"
	BX021      Code = "BX021"
	BX022      Code = "BX022"
	BX023      Code = "BX023"
	BX024      Code = "BX024"
	BX030      Code = "BX030"
	BX031      Code = "BX031"
	BX032      Code = "BX032"
	BX040      Code = "BX040"
	BX041      Code = "BX041"
	BX060      Code = "BX060"
	BX061      Code = "BX061"
	BX070      Code = "BX070"
	BX071      Code = "BX071"
	BX072      Code = "BX072"
	BX073      Code = "BX073"
	BXRun001   Code = "BX-RUN-001"
	BXRun002   Code = "BX-RUN-002"
	BXRun003   Code = "BX-RUN-003"
	BXRun010   Code = "BX-RUN-010"
	BXRun020   Code = "BX-RUN-020"
	BXRun021   Code = "BX-RUN-021"
	BXTest001  Code = "BX-TEST-001"
	BXTest002  Code = "BX-TEST-002"
	BXTest003  Code = "BX-TEST-003"
	BXInternal Code = "BX-INTERNAL"
)

// AllCodes は、すべてのコードを language.md §6.2 の表の順で返す。
// 返すのは毎回新しいスライス。
func AllCodes() []Code {
	return []Code{
		BX001, BX002, BX003, BX004, BX005, BX006,
		BX010, BX011, BX012, BX013, BX014, BX015,
		BX020, BX021, BX022, BX023, BX024,
		BX030, BX031, BX032,
		BX040, BX041,
		BX060, BX061,
		BX070, BX071, BX072, BX073,
		BXRun001, BXRun002, BXRun003, BXRun010, BXRun020, BXRun021,
		BXTest001, BXTest002, BXTest003,
		BXInternal,
	}
}

// Pos はソースの中の位置。Line・Col とも1始まりで、Col はコードポイント単位。
type Pos struct {
	Line int `json:"line"`
	Col  int `json:"col"`
}

// Span はソースの範囲。表示のための補助で、主たる位置は Box と Node。
type Span struct {
	File  string `json:"file"`
	Start Pos    `json:"start"`
	End   Pos    `json:"end"`
}

// Diagnostic は1つの診断。該当しない項目は空のままにする（JSON では省く）。
type Diagnostic struct {
	Code        Code     `json:"code"`
	Severity    Severity `json:"severity"`
	Message     string   `json:"message"`
	Box         string   `json:"box,omitempty"`
	Node        string   `json:"node,omitempty"`
	Path        string   `json:"path,omitempty"`
	Span        Span     `json:"span,omitzero"`
	Hint        string   `json:"hint,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
}

// New は、コードに応じた重さの診断を作る。警告は BX073 だけ。
func New(code Code, message string) Diagnostic {
	sev := Error
	if code == BX073 {
		sev = Warning
	}
	return Diagnostic{Code: code, Severity: sev, Message: message}
}

// Error は、診断を Go の error として扱えるようにする（利用者への表示には Text を使う）。
func (d Diagnostic) Error() string {
	return string(d.Code) + ": " + d.Message
}

// HasError は、ds にエラー（警告でないもの）があるかを返す。
func HasError(ds []Diagnostic) bool {
	for _, d := range ds {
		if d.Severity == Error {
			return true
		}
	}
	return false
}

// WriteJSON は、ds を JSON の配列として書く（--json）。診断が無ければ []。
func WriteJSON(w io.Writer, ds []Diagnostic) error {
	if ds == nil {
		ds = []Diagnostic{}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(ds)
}

// WriteText は、ds を人間向けの形で書く。診断の間は空行1つ。
func WriteText(w io.Writer, ds []Diagnostic) error {
	var b strings.Builder
	for i, d := range ds {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(Text(d))
	}
	_, err := io.WriteString(w, b.String())
	return err
}

var severityLabel = map[Severity]string{Error: "エラー", Warning: "警告"}

// Text は、1つの診断の人間向けの形（language.md §6.1）を返す。末尾は改行。
func Text(d Diagnostic) string {
	head := []string{severityLabel[d.Severity] + " " + string(d.Code)}
	if d.Box != "" {
		head = append(head, "箱 "+d.Box)
	}
	if d.Node != "" {
		head = append(head, "ノード "+d.Node)
	}
	if d.Path != "" {
		head = append(head, d.Path)
	}

	var b strings.Builder
	b.WriteString(strings.Join(head, "  ") + "\n")
	for line := range strings.SplitSeq(d.Message, "\n") {
		b.WriteString("  " + line + "\n")
	}
	if d.Span != (Span{}) {
		pos := fmt.Sprintf("%d:%d", d.Span.Start.Line, d.Span.Start.Col)
		if d.Span.File != "" {
			pos = d.Span.File + ":" + pos
		}
		b.WriteString("  位置: " + pos + "\n")
	}
	if d.Hint != "" {
		b.WriteString("  ヒント: " + d.Hint + "\n")
	}
	if len(d.Suggestions) > 0 {
		b.WriteString("  候補: " + strings.Join(d.Suggestions, ", ") + "\n")
	}
	return b.String()
}
