# Boxundle MVP

> **Boxundle** = **Box** + **Bundle**（箱を束ねる）

外部APIを使う感覚で、自分のシステム全体を扱えるようにする。人間が理解するのは「箱」の入出力・テスト・権限だけ。箱の中身はAIが書き、機械が検証する。

このリポジトリは、箱（`.bx`）を検証・実行する Go の単一のバイナリ`boxundle`を作る。いまは実装の前の段階で、文書だけがある。

## 文書

- [docs/README.md](docs/README.md)：文書の一覧と読む順番
- 構想：[docs/design/plan.md](docs/design/plan.md)
- 実装計画：[docs/design/mvp.md](docs/design/mvp.md)
- 仕様：[docs/reference/](docs/reference/)
- 例：[docs/examples/](docs/examples/)

## 開発環境

devcontainer で開く。`.devcontainer/postCreate.sh`が、Go の道具と mtqg・qsoku・srwr・bcuz・crube などを入れる。ビルド・テストは`qsoku check`（`qsokufile`）。

インストールの方法と使い方は、実装ができてから書く（`docs/design/mvp.md`の M8）。

## ライセンス

[MIT](LICENSE)
