# Boxundle ドキュメント

> **Boxundle** = **Box** + **Bundle**（箱を束ねる）
>
> 外部APIを使う感覚で、自分のシステム全体を扱えるようにする。人間が理解するのは「箱」の入出力・テスト・権限だけ。箱の中身はAIが書き、機械が検証する。

## design/ — なぜ・どう作るか

| 文書 | 内容 |
|---|---|
| [plan.md](design/plan.md) | 計画書。構想・原則・アーキテクチャ・逆転実験の計画・ロードマップ |
| [mvp.md](design/mvp.md) | MVPの実装計画。実装担当AIへの指示、内部設計、ディレクトリ構成、マイルストーンと受け入れ基準 |

## reference/ — 何であるか（仕様の正）

| 文書 | 内容 |
|---|---|
| [language.md](reference/language.md) | 言語リファレンス。字句・文法・意味・検証・エラーコード・値の受け渡し |
| [std.md](reference/std.md) | 標準の箱リファレンス。組み込みの部品の一覧と権限 |
| [cli.md](reference/cli.md) | `boxundle` コマンドリファレンス。各コマンド、正規形、`show` の表示形式、`stats` |

## examples/ — 動く例

| ファイル | 内容 |
|---|---|
| [order.bx](examples/order.bx) | 注文を処理する箱（B7）。分岐・繰り返し・テストの基本 |
| [checkout.bx](examples/checkout.bx) | 入れ子の箱・標準の箱・権限・スタブの例（order.bx の内容を含む） |
| [order_inputs.json](examples/order_inputs.json) | `boxundle run` に渡す B7 の入力 |
| [order_expected.json](examples/order_expected.json) | 上の入力に対する期待される出力 |

例のファイルは、`boxundle` 自体のテストでも使う。

## 読む順番

- 構想を知りたい：`design/plan.md`
- 実装を頼む・実装する：`design/mvp.md` → `reference/` → `examples/`
- Boxundleを書く（AI）：`reference/language.md` → `reference/std.md` → `examples/`
