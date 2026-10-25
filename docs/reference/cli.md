> [docs](../README.md) / reference / cli

# boxundle コマンドリファレンス

> 版：v0.3（2026-10-25）

`boxundle` は Go で作られた単一のバイナリ。言語の定義は [language.md](language.md)、標準の箱は [std.md](std.md)。

---

## 1. コマンド

すべてのコマンドで、`--json` 指定時は結果も診断もJSONで標準出力に出す。終了コードは、成功0、検証・実行・テストの失敗1、使い方の誤り2、内部の不具合3。

| コマンド | 内容 |
|---|---|
| `boxundle check FILE... [--json]` | 検証。警告だけなら終了コード0 |
| `boxundle fmt FILE [--check] [--write]` | 正規形で出力。`--check` は差分があれば終了コード1 |
| `boxundle run FILE BOX --input IN.json [--allow PERMS] [--fs-root DIR] [--stubs STUBS.json] [--json]` | 実行して出力を表示 |
| `boxundle test PATH [--json]` | `.bx` のテストを実行。PATHはファイルかディレクトリ。各テストを2回実行し、結果が違えば `BX-TEST-002` |
| `boxundle show FILE [BOX] [--depth N] [--results]` | 人間向け表示。BOX省略時はファイル内の全箱。`--results` でテスト結果も表示 |
| `boxundle trace FILE BOX --input IN.json [--allow PERMS] [--fs-root DIR] [--stubs STUBS.json] [--nodes]` | 箱の呼び出しごとの入出力を木構造で表示。`--nodes` で全ノードの値も表示 |
| `boxundle patch FILE BOX NODEID (--replace SNIPPET \| --insert-after SNIPPET \| --delete) [--write] [--force]` | ノードIDで部分編集。SNIPPETはファイルパスか `-`（標準入力）。編集後に `check` し、エラーがあれば書き込まない（`--force` を除く）。`--write` なしなら結果を標準出力へ |
| `boxundle stats PATH [--json]` | §4 の集計 |
| `boxundle std [NAMESPACE] [--json]` | 標準の箱の一覧 |
| `boxundle version` | バージョン |
| `boxundle help` | コマンドの一覧（`-h`・`--help` も同じ） |

### 1.1 引数と使い方の誤り

- フラグは、FILE などの引数の前にも後ろにも書ける（`boxundle fmt --check order.bx` と `boxundle fmt order.bx --check` は同じ）
- 使い方の誤り（未知のコマンド・フラグ、引数の過不足、一緒に使えないフラグ、指定したファイルが無い・読めない）は、診断のコードを付けず、標準エラー出力に2行を出して終了コード2

```
$ boxundle fmt
boxundle: fmt には FILE が1つ要ります
使い方: boxundle fmt FILE [--check] [--write]
$ boxundle frobnicate
boxundle: 未知のコマンド「frobnicate」です
使い方: boxundle <コマンド> [引数...]（一覧は boxundle help）
$ boxundle fmt nothing.bx
boxundle: ファイル「nothing.bx」を読めません（見つかりません）
使い方: boxundle fmt FILE [--check] [--write]
```

- 引数なしの `boxundle` は、コマンドの一覧を標準エラー出力に出して終了コード2。`boxundle help`（`-h`・`--help`）は、同じ一覧を標準出力に出して終了コード0

### 1.2 内部の不具合

`boxundle` 自体の不具合（panic）は、最上位で捕まえて `BX-INTERNAL` の診断で報告し、終了コード3。本文は決まった文で、panic の値は報告の手がかりとしてヒントに添える。`--json` を付けたコマンドでは JSON で出す。

```
エラー BX-INTERNAL
  boxundle 内部の不具合です（コマンド fmt）
  ヒント: 不具合として報告してください。詳細: <panic の値>
```

### 1.3 fmt

- `boxundle fmt FILE` は、正規形（§2）を標準出力に出す。終了コード0
- `--write` は FILE を正規形で書き換え、何も出さない。終了コード0
- `--check` は何も書き換えない。正規形なら何も出さず終了コード0、違えば標準エラー出力に `FILE は正規形ではありません` の1行を出して終了コード1
- `--check` と `--write` は一緒に使えない（使い方の誤り）
- 構文エラーは `BX001` の診断を標準エラー出力に出し、終了コード1（`--write` でも書き換えない）

```
$ boxundle fmt order.bx --check
order.bx は正規形ではありません
```

### 1.4 version

標準出力に1行。リリースのバイナリと `go install …@v0.1.0` で入れたものは版を出す。手元でビルドしたものは Go が VCS から付ける版（`v0.0.0-日時-コミット`、コミットしていない変更があれば `+dirty`）を出し、版が分からなければ `(devel)` を出す。

```
$ boxundle version
boxundle v0.1.0
```

### 1.5 check

- `boxundle check FILE...` は、FILE を1つずつ検証し（language.md §5）、見つかった診断を、引数の順に、ファイルの中では位置の順にすべて出す
- 診断は、`--json` なしなら人間向けの形（language.md §6.1）で標準エラー出力に、`--json` なら全ファイルの診断を1つのJSONの配列で標準出力に出す
- 診断が無ければ、`--json` なしでは何も出さず、`--json` では `[]` を出す。終了コード0
- エラーが1つでもあれば終了コード1。警告（`BX073`）だけなら終了コード0
- 読めないファイルが1つでもあれば、どのファイルも検証せず、使い方の誤り（§1.1）

```
$ boxundle check checkout.bx
エラー BX071  箱 Checkout  ノード #2
  std.clock.now が使う権限 clock が、perm にありません
  位置: checkout.bx:122:13
  ヒント: 箱の perm には、本体で呼ぶ箱の権限をすべて宣言します
```

## 2. 正規形（fmt）

- インデントは2スペース、1ノード1行、ブロックの `{` は同じ行、`}` は単独行（中身の無いブロックも `{` と `}` を別の行に書く）
- 箱の中の順序：`in`（定義順）→ `out`（定義順）→ `perm` → `test` → `body`（標準の箱の宣言では `impl`）
- 先頭の `module` / `externs` の行と、箱・型定義の間は空行1つ。テストの間も空行1つ
- テストがあれば、最初のテストの前と最後のテストの後にも空行1つ。テストが無ければ、`perm` の次の行が `body`（`impl`）
- テストの中の順序：`stub` → `given` → `expect`
- `given` / `expect` は1ポート1行（`,` は行末。空なら `given {}`）。`stub` は1行で `stub std.clock.now { now: "…" }`（空なら `{}`）
- ポートは `in $order : Order`。`perm` の権限は `, ` で区切る（`perm clock, net`）
- 型は1行：`{product: Str, qty: Int}`、`{Str -> Int}`、`[Line]`、`Invoice | Shortage`
- 値は1行：`Line{product: "A", qty: 2}`、`{a: 1}`、`{"A" -> 5, "B" -> 1}`、`[1, 2]`、`{->}`、`([] : [Int])`。`call` の引数は `{order: $order}`（空なら `{}`）。束縛子は `as $e`、`as ($acc, $e)`
- `route` / `match` の分岐は1つずつ `true {` 〜 `}`、`Invoice $inv {` 〜 `}` と書き、分岐ノードの `}` は単独行
- 数は書かれたとおり（`1.50` を `1.5` にしない）
- 文字列は `"` `\` 改行・タブを `\"` `\\` `\n` `\t` に、そのほかの制御文字（U+0000〜U+001F、U+007F〜U+009F）と U+2028・U+2029 を `\u` と大文字の16進数4桁にする。それ以外の文字はそのまま書く
- ラベルはノードの末尾（ブロックを持つノードは `}` の後）
- IDの振り直しはしない
- `parse(print(ast)) == ast`（spanを除く）が常に成り立つこと

## 3. show の出力形式

```
箱 Checkout「注文を確定する」

受け取るもの
  order : Order = { customer: Str, lines: [Line] }
            Line = { product: Str, qty: Int, price: Int }
  stock : Stock = { Str -> Int }

返すもの
  message    : Str
  ordered_at : Str
  new_stock  : Stock

権限
  clock（現在時刻の取得）

使っている箱
  B7「注文を処理する」                 Boxundle・テスト3件
  std.clock.now「現在時刻を返す」       標準の箱（組み込み実装）・権限 clock

テスト
  ✓ 在庫が足りれば受け付け、受付時刻を返す
      固定    std.clock.now → now = "2026-10-25T09:00:00+09:00"
      与える  order = Order{customer: "c1", lines: [Line{product: "A", qty: 1, price: 100}]}
              stock = {"A" -> 1}
      期待    message = "注文を受け付けました"
              ordered_at = "2026-10-25T09:00:00+09:00"
              new_stock = {"A" -> 0}
```

- 本体（body）は一切表示しない
- 「使っている箱」では、Boxundleで書かれた箱と標準の箱を必ず区別して表示する
- `--depth N` で、`call` している箱を入れ子でN段まで同じ形式で表示する（標準の箱は1行のまま）
- テスト結果は `--results` 指定時のみ（✓ / ✗、失敗時は差分）。指定なしなら記号の代わりに `・`

---

## 4. stats

`boxundle stats PATH [--json]` は、PATH（ファイルまたはディレクトリ配下の `.bx` すべて）について次を数える。逆転実験の材料にする。

```json
{
  "files": 3,
  "boxes": 12,
  "nodes": 340,
  "max_depth": 4,
  "labels": 120,
  "ops": {"map": 21, "filter": 9, "call": 30},
  "calls": {"box": 14, "std_pure": 12, "std_effect": 4},
  "tests": 25,
  "bytes": 18234
}
```

- `ops` は使われた演算だけを、キー昇順で出す
- `max_depth` はノードIDの最大の深さ（`#4.1.2` なら3）

## 5. std

`boxundle std [NAMESPACE]` は、標準の箱を `show` と同じ人間向けの形式で一覧表示する。`--json` で宣言の内容をJSONで出す（実験でAIに標準の箱の一覧を渡すために使う）。

---
