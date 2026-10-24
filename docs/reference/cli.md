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

## 2. 正規形（fmt）

- インデントは2スペース、1ノード1行、ブロックの `{` は同じ行、`}` は単独行
- 箱の中の順序：`in`（定義順）→ `out`（定義順）→ `perm` → `test` → `body`
- 箱と型定義の間は空行1つ。テストの間も空行1つ
- テストの中の順序：`stub` → `given` → `expect`
- ラベルはノードの末尾
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
