# ディレクトリ構成

新しいファイルを追加する際は、どこに置くかをここで確かめる。**構成を変えたら、この文書も同じ変更の中で直す。**

**このリポジトリの説明（`.crube/`）も保つ。** ファイル・ディレクトリを足す・動かす・消すときは、同じ変更の中で説明（日本語だけ）を直し、コミットの前に`env CRUBE_LANG=ja crube status --require all`を通す。やり方（MCPとCLI、`.crube/ignore`の保ち方）は`.claude/rules/crube-usage.md`

```
.
├── CLAUDE.md               ← プロジェクトルール（参照先の案内）
├── LICENSE                 （MIT）
├── README.md
├── qsokufile               ← ビルド・テストの近道（qsoku）
├── go.mod / go.sum
├── .gitattributes          （`* text=auto eol=lf`）
├── .gitignore
├── .golangci.yaml          ← lintの設定
├── .goreleaser.yaml        ← 配布物の作り方（goreleaser。`release.yml`と`qsoku snapshot`が使う。`.claude/rules/distribution.md`）
├── trivy.yaml              ← 脆弱性・ライセンス検査の設定
├── .mcp.json               ← mtqg・srwrとcrube（日本語の`crube-ja`）のMCPサーバーの配線
├── .srwrignore             ← srwrがテープに記録しないファイル（`.mtqg/`・`.bcuz/`など）
├── .bcuz/                  ← bcuzの記録（`history.jsonl`）。`postCreate.sh`が`bcuz --init`で作る。**コミットする**（`merge=union`）
├── .crube/                 ← このリポジトリのファイルの説明（crube。説明は日本語の`structure_ja.txt`だけ、`ignore`は`.gitignore`に合わせて手で保つ）。**コミットする**
├── .srwr/                  ← srwrの鍵・ロック・テープ（最初のhook・MCPの呼び出しでできる。**閉じた`tapes/*.gz`だけコミットする**）
├── boxundle.go             ← 公開API（`package boxundle`。テストは`boxundle_test.go`）
├── cmd/boxundle/           ← CLI
├── internal/               ← このモジュールの中だけで使うコード
├── testdata/diag/          ← エラーコードごとの最小の入力（`.bx`）と期待する診断（`.json`）
├── tools/licenses/         ← 配布物に入れる依存のライセンス文を集める道具（goreleaser が`go run`で動かす。実行ファイルには組み込まれない）
├── docs/                   ← 文書（design/：計画と実装の進め方、reference/：仕様の正、examples/：動く例。`.claude/rules/documentation.md`）
├── .devcontainer/          ← 開発環境（devcontainer.json、postCreate.sh）
├── .claude/                ← rules/（ルール）、skills/（commit-with-tape：テープを閉じてコミットする手順。mutation-check：壊して確かめる手順と、変異の道具`mutants/`（別の Go モジュール）・仕様の変異の一覧`spec.txt`）、hooks/（build.sh、require-bcuz.sh とそのテスト、crube-untracked.sh：srwr の new のあとに未説明を知らせる）、settings.json（人間が管理する）
├── .github/workflows/      ← CI（`ci.yml`）と、タグで動くリリース（`release.yml`。`ci.yml`を呼んでから Release を作って公開する）
└── .mtqg/                  ← mtqgの記録
```

## 配置の判断基準

**Go のコードの置き場所は、`docs/design/mvp.md` §4.2 の構成に従う。** 要点：

- **公開API**（`LoadFile`・`CheckText`・`Run`など。`docs/design/mvp.md` §4.3）は、ルートの`package boxundle`（`boxundle.go`）に置く。外からimportされるのはここだけ
- **CLI**は`cmd/boxundle/main.go`。引数を解いて公開APIか`internal/`を呼ぶだけにする
- **実装**は`internal/`の下に、役割ごとのパッケージで置く（`diag`・`syntax`・`types`・`check`・`value`・`interp`・`std`・`view`・`patch`・`testrun`・`stats`）。新しい役割が要るときも、§4.2 に無いパッケージを足す前に、どれかに入らないかを考える。足したら §4.2 と、この文書を直す
- **標準の箱の宣言**（`.bx`）は`internal/std/decl/`に置き、`go:embed`で組み込む
- **テストの入力**：エラーコードごとの最小の入力と期待する診断は`testdata/diag/`。`docs/examples/`の例はテストからそのまま読む（写しを作らない）

## 各ファイル・ディレクトリの補足

- **`cmd/boxundle/`**: CLI。`main.go`は、コマンドの表・引数の解析・使い方の誤り・終了コード・panic の`BX-INTERNAL`を受け持ち、コマンドごとの処理は`<コマンド>.go`（`check.go`・`fmt.go`・`version.go`）に置く。どれも引数を公開APIか`internal/`に渡すだけにする。コマンドの表は大域の変数にせず`commands()`が返す（テストが panic するコマンドを渡せるように）。e2e テスト（`e2e_test.go`）は、`-X main.version`を付けてビルドしたバイナリを`os/exec`で呼ぶ
- **`tools/licenses/`**: 配布物に同梱する依存のライセンス文を集める（`.claude/rules/distribution.md`）。`go run ./tools/licenses <出力先> <main パッケージ>`。`TestRun`は`./cmd/boxundle`に組み込まれるモジュールの一覧のテスト（依存を足す・外すと落ちる）
- **`.claude/skills/mutation-check/mutants/`**: 変異の道具。`go.mod`を持つ別のモジュールなので、`qsoku check`（`./...`）の対象に入らない。変えたら、そのディレクトリで整形・vet・テストを通す（Skill の「気をつけること」）
- **`internal/`**: Goの仕組みとして、モジュールの外からimportできない。外へ公開するパッケージは`internal/`に置かない
- **`go.sum`**: コミットする（改ざん検知・再現可能なビルドのため）
- **`.mtqg/`**: コミットされる。中のファイルは直接編集せず、すべて`mtqg`のコマンド経由で書く（`.claude/rules/mtqg-usage.md`）。`.mtqg/.local/`はコミットされない
- **`.bcuz/`**: bcuzが`.bcuz/history.jsonl`に追記する、コマンドの実行の記録（`.claude/rules/bcuz-usage.md`）。**コミットする**（`.gitignore`の`!/.bcuz/`）。行を足すだけなので、`.gitattributes`の`merge=union`で、マージしても両側の行が残る。コミットの前に`git add .bcuz/history.jsonl`する（`git commit`は`bcuz`を通さない）。秘密をコマンドに書かない（記録に残る）。中のファイルは直接編集しない。`.srwrignore`でテープにも載せない
- **`.claude/hooks/require-bcuz.sh`**: Bashツールのコマンドを、`bcuz`を通っていなければ実行前に断る`PreToolUse`フック。判定の表は`require-bcuz.test.sh`（`bash .claude/hooks/require-bcuz.test.sh`）。判定を変えたら、テストの表も同じ変更の中で直す
- **`.mcp.json`**: `mtqg mcp`と`srwr mcp`（MCPサーバー）の配線。`mtqg init --agent claude-code`・`srwr init`が書くものと同じ。その後ろに、crubeのMCPサーバー`crube-ja`（`crube mcp --allow-write`と`CRUBE_LANG=ja`）を手で足している（`.claude/rules/crube-usage.md`）
- **`.crube/`**: このリポジトリのファイルとディレクトリの説明（`.claude/rules/crube-usage.md`）。**コミットする**（`.gitignore`の`!/.crube/`）。コンテナを作ったあと（`.bcuz/`・`devcontainer-lock.json`・`.srwr/`ができたあと）に、`status --require all`が通る形にしてある。閉じたテープ（`*.tape.jsonl.gz`）は、説明を付けず`.crube/ignore`で外す（パターンのエントリにすると、最初のテープを閉じるまで何にも当たらず`orphan`になり、無ければ新しいテープが`untracked`になるため）。`ignore`は`.gitignore`を手で写したもの（crube は`.gitignore`を読まない）
- **`.srwr/`**: 最初のhook・MCPの呼び出しで、srwrが作る（`srwr init`でも作られる。鍵`key`・`lock`・`active`・`tapes/`・`init-backup/`）。**コミットするのは閉じて圧縮したテープ`tapes/*.tape.jsonl.gz`だけ**。書き込み中の生のテープ`*.tape.jsonl`は`.gitignore`で対象外（閉じると`.gz`に置き換わり、生のままコミットすると「削除＋追加」の差分が出るため）。鍵は共有しない。テンプレートには置いていない（鍵をプロジェクトごとに別にするため）
- **`.srwrignore`**: テープに記録しないファイル（`.gitignore`と同じ書き方）。生成物や巨大なファイルはここに足す。`.env`・`*.pem`・`*.key`などはsrwrが元から外す
- **`.devcontainer/devcontainer-lock.json`**: コンテナのビルドで自動で作られる（このテンプレートには置いていない）。**できたらコミットする**（この版のfeatureでテストなどが通った証明になる）。featureを新しくしたいときは、lockを消してビルドし直し、新しいlockをコミットする
- **VSCodeの設定**: `.devcontainer/devcontainer.json`の`customizations.vscode`に置く（拡張と設定を1か所にまとめる）。`.vscode/`は作らない
- **`.gitattributes`**: `* text=auto eol=lf`。Windowsランナーでの改行コード変換による誤検知を防ぐ（`.claude/rules/testing.md`）。`.mtqg/`の中の`.gitattributes`は`mtqg init`が置くもので、別物
- **`.gitignore`のドットファイル**: ルート直下のドットファイル・ドットディレクトリは**許可リスト方式**（`/.*`を無視し、追跡するものだけ`!`で戻す。`.claude/`の中は`/.claude/*`を無視して`settings.json`・`rules/`・`skills/`・`hooks/`だけ戻す）。サンドボックスが作業ディレクトリ直下に出すダミーファイルを`git add -A`で混ぜないため（`.claude/rules/testing.md`）。**追跡するドットファイルを新しく足すときは、`.gitignore`に`!`の行を足すこと**。足さないと`git add`が無視する
- **配布物（ビルド済みバイナリ）**: リポジトリにコミットしない。`.gitignore`には**ルート直下に限定して**書く（`/名前`）。名前だけを書くと、同名のソースディレクトリまで無視される
- **`docs/`**: 文書。`design/`（計画書`plan.md`・実装計画`mvp.md`）、`reference/`（言語・標準の箱・コマンドのリファレンス。**仕様の正**）、`examples/`（例。テストでも使う）。書き方は`.claude/rules/documentation.md`
- **シェルスクリプト**: 最小限にする。追跡中のものは`qsoku shellcheck`の対象になる（`.claude/rules/testing.md`）
