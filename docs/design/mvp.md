> [docs](../README.md) / design / mvp

# Boxundle MVP 実装計画

> 対象：[計画書](plan.md)のフェーズ1（Boxundle MVP）
> 目的：この文書と [reference/](../reference/)、[docs/examples/](../examples/) を渡して、AIに実装を依頼できること
> 版：v0.4（2026-10-25）

### 改訂履歴

| 版 | 日付 | 内容 |
|---|---|---|
| v0.1 | 2026-10-24 | 初版 |
| v0.2 | 2026-10-25 | 外部の箱（extern）、標準の箱、権限の静的検査、プロジェクト設定とロック、テストのスタブ、集まりの組み込み演算の追加 |
| v0.3 | 2026-10-25 | **方針変更。** 実装言語をPythonからGoに変更。制限付きPythonからの変換（from-python）を削除。外部の箱は組み込みの標準の箱のみとし、プロジェクト独自の外部の箱・プロジェクト設定・ロックを削除。権限の実行時の強制を追加。エラーコードを整理 |
| v0.4 | 2026-10-25 | 資料を `docs/design`・`docs/reference`・`docs/examples` に分割。言語・標準の箱・コマンドの定義は reference に移し、この文書は実装の進め方だけを扱う。例は `docs/examples/` を正とする |

---

## 0. 実装担当AIへの指示

1. **マイルストーン順（§5）に実装する。** 各マイルストーンは受け入れ基準を満たしてから次へ進む
2. **テストを先に書く。** 各マイルストーンの受け入れ基準をまず `go test` のテストとして書き、それが通る実装を作る
3. **仕様を黙って変えない。** 仕様の矛盾・不足を見つけたら、最も単純で一貫した解釈を選んで実装し、mtqg に「何を・なぜ」決めたかを1項目ずつ記録する（人間に聞いたことは `mtqg q add`、自分で決めたことは `mtqg m add`。`.claude/rules/mtqg-usage.md`）
4. **依存は最小限に。** 許可する外部依存は §4.1 に列挙したものだけ
5. **エラーはすべて [language.md §6](../reference/language.md) の構造化形式で返す。** Goのエラーや panic の生メッセージをそのまま利用者に見せない。panic は内部の不具合としてのみ扱い、CLIの最上位で捕まえて `BX-INTERNAL` として報告する
6. **やらないこと（§6）には手を出さない**
7. **仕様の正は reference。** 言語・標準の箱・コマンドの定義は [reference/](../reference/) にある。実装中に仕様を決めたり直したりしたら、reference も同時に更新する
8. **Goのマップの走査順に依存しない。** 結果に影響する走査は、必ずキーを整列してから行う（[language.md §4.5](../reference/language.md)）
9. **大域的な可変状態を持たない。** 標準の箱の登録表など、初期化後に変わらないものは除く

---

## 1. MVPのゴール

次の一連の流れが、手元のマシンで動くこと。

1. 箱の形式（`.bx` ファイル）を書く
2. 標準の箱（文字列、正規表現、日付、ハッシュ、時刻、HTTPなど）を `call` で使う
3. `boxundle check` で構文・スコープ・型・出力の網羅性・権限を検証する
4. `boxundle test` で箱に書かれたテストを実行する（作用のある箱はスタブで置き換える）
5. `boxundle run` で実行する。作用は、実行時に許可した権限の範囲でだけ起きる
6. `boxundle show` で、人間向けの表示（入出力・権限・使っている箱・テスト）を見る
7. `boxundle trace` で、入れ子の箱の入出力を実際の値で確認する（箱を開けて中を覗く）
8. `boxundle patch` で、ノードIDを指定して箱の中身を部分的に差し替える
9. `boxundle stats` で、箱の形式で書いた量と標準の箱を使った量を測る（逆転実験の材料）

`boxundle` はGoで書いた単一のバイナリで、箱の本体はその中のインタプリタで実行する。Wasmは一切使わない。

---

## 2. 設計原則（実装上の帰結）

| 原則 | 実装上のルール |
|---|---|
| 行という概念を持たない | 改行・インデントは意味を持たない。パーサは空白をすべて同一視する。場所の指し示しはノードIDで行う |
| 変数を持たない | 値は「ノードの出力」として一度だけ定義され、書き換えられない |
| 名前は識別に使わない | ノードはIDで識別する。`@"..."` のラベルは意味の手がかりで、削除しても動作は変わらない |
| 関数を持たない | 箱の中に関数定義はない。他の箱の利用は `call` のみ |
| IF/WHILEを持たない | 分岐は `route` / `match`、繰り返しは `map` / `filter` / `fold` など有限の集まりへの操作のみ。必ず停止する |
| 状態を隠さない | 本体は純粋。状態は入力ポートで受け取り、出力ポートで返す。外への作用は作用のある標準の箱の中だけで起き、権限として外側に現れる |
| 前から順に検証できる | ノードは、テキスト上で自分より前に定義されたものしか参照できない |
| 書き方は一通り | `boxundle fmt` が唯一の正規形を出力する |
| コメント構文はない | 意味はラベルで持つ |
| 手法に依存しない | TDD・DDD・SDDなどの進め方は強制しない。部品（箱・型・テスト・権限・入れ子）だけを提供する |
| Boxundleだけで書く | 利用者（AI）が書くのは `.bx` だけ。他の言語のコードを置いたり呼んだりする手段は持たない |
| 汎用の機能は標準の箱で | 汎用の部品は `boxundle` に組み込む。標準の箱は「Boxundleで検証された中身ではない」ので、表示では区別する |

---

## 3. 内部の設計

### 3.1 Goでの値の表現

```go
package value

type Value interface{ isValue() }

type Int  struct{ V *big.Int }          // 不変として扱う
type Dec  struct{ V apd.Decimal }       // 演算は有効桁28・HALF_EVEN の apd.Context で行う
type Bool bool
type Str  string                        // 常に正しいUTF-8
type List []Value
type Map  struct {                      // キー昇順を常に保つ
    Keys []Value
    Vals []Value
}
type Record struct {
    Tag    string                       // 無名レコードは ""
    Names  []string                     // フィールド名（型定義の順）
    Fields []Value
}
```

- 値は不変。演算は常に新しい値を返す（`set` などで元のマップを書き換えない）
- 等値比較・順序比較・JSONとの相互変換は `value` パッケージにまとめる

### 3.2 標準の箱の組み込み方

各箱の仕様は [std.md](../reference/std.md)。

- 宣言は `internal/std/decl/*.bx` に置き、`go:embed` でバイナリに埋め込む
- 実装はGoの関数として `internal/std` に置き、登録名（`impl builtin "text.split"` の文字列）で引けるように登録表に登録する
- 実装の関数の形：

```go
type Builtin func(ctx *Context, in map[string]value.Value) (map[string]value.Value, error)
```

- `ctx` は許可された権限、`--fs-root`、HTTPクライアントなどを持つ。作用のある箱は呼ばれた最初に `ctx` で許可を確認する
- 実装が返した値は、宣言された出力の型で検査する（合わなければ `BX-INTERNAL`。`boxundle` 自体の不具合のため）
- 宣言と登録表の対応（宣言があるのに実装がない、実装があるのに宣言がない）は、`go test` で検査する
- 作用のない箱には、宣言の中に `test` を1つ以上書くこと。これも `go test` で全件実行する

### 3.3 標準の箱の実装例

`internal/std/text.go`（抜粋）

```go
func textSplit(ctx *Context, in map[string]value.Value) (map[string]value.Value, error) {
	s := string(in["s"].(value.Str))
	sep := string(in["sep"].(value.Str))
	if sep == "" {
		return nil, runErr("区切り文字が空です")
	}
	parts := strings.Split(s, sep)
	out := make(value.List, len(parts))
	for i, p := range parts {
		out[i] = value.Str(p)
	}
	return map[string]value.Value{"parts": out}, nil
}
```

`internal/std/clock.go`（抜粋）

```go
func clockNow(ctx *Context, in map[string]value.Value) (map[string]value.Value, error) {
	if err := ctx.Require("clock"); err != nil {
		return nil, err // BX-RUN-020
	}
	return map[string]value.Value{"now": value.Str(ctx.Now().Format(time.RFC3339))}, nil
}
```

---

## 4. 実装構成

### 4.1 技術選定

- Go 1.24 以上（`os.Root` を使うため）
- 外部依存：`github.com/cockroachdb/apd/v3`（`Dec` の実装）のみ
- それ以外は標準ライブラリだけで実装する（CLIは `flag`、パーサは手書き、テストは `testing`）
- `gofmt` 済みで、`go vet` を通すこと

### 4.2 ディレクトリ

```
boxundle/
  go.mod                       # module github.com/amisonnet8/boxundle-mvp
  README.md
  boxundle.go                  # 公開API
  cmd/boxundle/main.go         # CLI
  internal/
    diag/                      # 診断の型・コード・整形（人間向け／JSON）
    syntax/
      lexer.go
      parser.go                # テキスト → AST（span付き）。モジュールと標準の箱の宣言の両方
      ast.go
      printer.go               # AST → 正規形テキスト（fmt）
    types/                     # 型の表現・単一化・注入
    check/
      check.go                 # 入口（Check・CheckStd）、診断を位置の順に返す、BX070
      typedefs.go              # 型定義とポートの型の検証、型の表（types.Env）
      suggest.go               # 名前の誤りの候補（編集距離2以内）
      scope.go                 # ID規則・スコープ
      typecheck.go
      emits.go                 # 出力の網羅性・分岐の種類
      callgraph.go             # 循環検出
      perms.go                 # 権限の伝播
    value/                     # 値の表現・比較・JSON変換
    interp/
      eval.go                  # 本体・ブロック・分岐・繰り返しの評価
      ops.go                   # ブロックを取らない組み込み演算
      trace.go
    std/
      decl/                    # text.bx re.bx math.bx date.bx hash.bx clock.bx random.bx env.bx fs.bx http.bx
      registry.go              # 登録表・go:embed
      context.go               # 権限・fs-root・HTTPクライアント
      text.go re.go math.go date.go hash.go clock.go random.go env.go fs.go http.go
    view/                      # show の出力
    patch/
    testrun/                   # .bx のテスト実行（スタブ、2回実行）
    stats/
  testdata/
    diag/                      # エラーコードごとの最小の入力と、期待する診断（JSON）
  docs/
    README.md
    design/                    # 計画書、この実装計画
    reference/                 # 言語・標準の箱・コマンドのリファレンス（仕様の正）
    examples/                  # 例。テストでもこのファイルを使う
      order.bx
      order_inputs.json
      order_expected.json
      checkout.bx
```

### 4.3 公開API（Go）

```go
import boxundle "github.com/amisonnet8/boxundle-mvp"

m, err := boxundle.LoadFile("docs/examples/order.bx")
// 検証エラーがあれば err は *boxundle.CheckError（Diagnostics を持つ）

diags := boxundle.CheckText("order.bx", src)   // 診断のスライスを返す（エラーを返さない）

out, err := m.Run("B7", inputs, boxundle.RunOptions{})
// inputs: map[string]value.Value、または m.InputsFromJSON("B7", jsonBytes) で作る
// out:    map[string]value.Value。m.OutputsToJSON("B7", out) でJSONにする

out, err = m.Run("Checkout", inputs, boxundle.RunOptions{
    Allow:  []string{"clock"},
    FSRoot: ".",
    Stubs:  map[string]map[string]value.Value{ /* 箱名 → 出力 */ },
    Trace:  true,                               // true なら m.LastTrace() で取得
})

results := m.Test()                             // 各テストの合否と差分
```

## 5. マイルストーンと受け入れ基準

各マイルストーンの受け入れ基準は、そのまま `go test` のテストにすること。CLIの受け入れ基準は、ビルドしたバイナリを `os/exec` で呼ぶテストにする。

### M1：構文（字句・パーサ・AST・正規形）

- `docs/examples/order.bx`、`docs/examples/checkout.bx`、`internal/std/decl/*.bx` を構文解析できる
- 改行をすべてスペースに置き換えたテキストも、同じASTになる
- `fmt` の結果を再度解析すると、元と同じASTになる（spanを除く）
- `fmt` は冪等（2回かけても変わらない）
- 構文エラーは `BX001` で、spanを含む。不正なUTF-8も `BX001`

### M2：検証・型検査

- `docs/examples/order.bx` と `docs/examples/checkout.bx` が診断なしで通る
- [language.md §6.2](../reference/language.md) の BX002〜BX073 の各コードについて、それを起こす最小の入力を `testdata/diag/` に1つずつ用意し、そのコードが返ることを確認する
- 1つのファイルに複数のエラーがあるとき、すべて返る
- フィールド名の誤りで `suggestions` に正しい名前が入る。`std.text.splt` の呼び出しで `std.text.split` が候補に入る
- 権限の伝播：`checkout.bx` から `perm clock` を消すと `BX071` が返る

### M3：インタプリタ（組み込み演算）

- `docs/examples/order.bx` の B7 を `docs/examples/order_inputs.json` で実行し、`docs/examples/order_expected.json` と一致する
- [language.md §4.3](../reference/language.md) の全演算について単体テストがある（`sort_by` の安定性、`group_by` の順序、`range` の境界、`mod` の符号、`Dec` の丸めを含む）
- 選ばれなかった分岐は評価されない（ゼロ除算を置いた分岐が選ばれなければエラーにならない）
- マップの走査順がキー昇順であること（同じ入力で100回実行して出力が常に同じ）
- `run` のJSON入出力が [language.md §7.1](../reference/language.md) どおり（大きな整数・小数の精度が落ちないことを含む）

### M4：標準の箱と権限の実行時強制

- [std.md](../reference/std.md) の標準の箱がすべて宣言・実装・登録されている。宣言と登録表が一致する
- 作用のない標準の箱の宣言内テストがすべて通る
- 作用のある箱を `go test` で確認する（HTTPは `net/http/httptest`、ファイルは一時ディレクトリ、時刻・乱数・環境変数は `Context` に差し込める関数で）
- 実行時の権限：
  - `--allow` なしで `Checkout` を `run` すると、実行前に `BX-RUN-020`（不足は `clock`）
  - `--allow clock` なら実行できる
  - `std.fs.read_text` に `../x`、絶対パス、`--fs-root` の外を指すシンボリックリンクを渡すと `BX-RUN-021`
- 標準の箱の失敗（不正な正規表現、不正な日付など）が `BX-RUN-010` になり、原因を含む

### M5：テスト

- `boxundle test docs/examples/` で、order.bx と checkout.bx のテストがすべて通る
- 期待値を1箇所変えると `BX-TEST-001` で失敗し、差分（どのポートのどこが違うか）が表示される
- checkout.bx のテストからスタブを消すと `BX-TEST-003`
- スタブの対象が呼ばれていなければ `BX061`
- `docs/examples/order.bx` について、Goのテストでランダムな注文と在庫を1,000件生成し（`math/rand/v2`、種は固定）、在庫が足りる注文では `total` が数量×単価の総和に等しく、足りない注文では在庫が変わらないことを確認する

### M6：show・trace・std・stats

- `boxundle show docs/examples/checkout.bx Checkout` が [cli.md §3](../reference/cli.md) の形式で出力され、本体を含まない
- `--depth 1` で B7 の外側が入れ子で表示される
- 「使っている箱」で標準の箱が区別して表示される
- `trace` が、呼び出しごとの入出力を木構造で表示する（標準の箱の呼び出しも含む）
- `boxundle std text` が std.text の箱を一覧表示し、`--json` で宣言をJSONで出す
- `boxundle stats docs/examples/` の `calls` が、`box` / `std_pure` / `std_effect` を正しく数える

### M7：部分編集（patch）

- order.bx の `#4.3` を `--replace` で別の内容に差し替えられ、他のノードは一切変わらない（ASTで比較）
- `--insert-after #2` で挿入できる（挿入するノードのIDが既存と重複していればエラー）
- `--delete` で削除できる。削除したノードを参照しているノードがあれば `check` のエラーとなり、書き込まれない
- 差し替えた結果が検証エラーになる場合、ファイルは変更されず診断が返る
- 存在しないノードIDの指定はエラー

### M8：仕上げ

- README：Boxundleの考え方（[計画書](plan.md)の§1〜3の要約）、インストール（`go install`）、order.bx と checkout.bx を使った一通りの使い方
- `docs/reference/` が実装と一致している（コマンドの出力例、エラーコード、標準の箱の一覧）
- `docs/reference/language.md` と `std.md` から、**Boxundleを初めて見るAIに渡す言語仕様**（`docs/reference/prompt.md`）を作る。逆転実験のプロンプトにそのまま使うので、余計な背景説明を省き、例を多くする
- `docs/reference/std.md` の一覧が、`boxundle std` の出力と一致していることを `go test` で確認する
- CI（GitHub Actions）：`gofmt` の差分なし、`go vet`、`go test -race ./...`、Linux・macOS・Windows向けのビルド
- 実装中に決めたことが、すべて mtqg に記録されている

---

## 6. やらないこと（MVPの範囲外）

- **Wasmに関わることすべて**：Boxundle → Wasm の変換、Wasmモジュールの読み込み・実行
- プロジェクト独自の外部の箱（利用者が部品を足す仕組み）。将来Wasmで追加する
- 他の言語からの変換（from-python など）
- DBの標準の箱（SQLiteなど）
- 型の決まらないJSONの解析・生成の標準の箱（動的な型の扱いを決めてから追加する）
- 複数モジュールの読み込み（`import`）
- プロパティテストの言語への組み込み
- 箱の融合などの最適化、並列実行
- 形式検証（証明）
- 書きかけのテキストの途中検証（後のフェーズで `check --prefix` として追加予定。ただし文法と検証は「前方参照なし」を守っているので、将来追加できる構造にしておくこと）
- 逆転実験のハーネス（題材、共通の受け入れテスト、比較言語での実行と測定。別仕様とする）
- GUI、エディタ連携（LSP）

---
