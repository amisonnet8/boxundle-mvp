> [docs](../README.md) / reference / std

# 標準の箱リファレンス

> 版：v0.3（2026-10-25）

---

## 1. 概要

- 標準の箱は `boxundle` に組み込まれた、本体を持たない箱。名前は `std.` で始まり、宣言なしで `call` できる
- 中身は Boxundle ではなく `boxundle` 自体（Go）で実装されている。そのため `show` では「標準の箱（組み込み実装）」として、Boxundle で書いた箱と区別して表示される
- `perm none` 以外の箱（作用のある箱）は、呼び出す箱の `perm` に権限を宣言しなければならない（[language.md §4.6](language.md)）
- 作用のある箱は、テストではスタブが必須（`BX-TEST-003`）。`run` では `--allow` で許可した権限の範囲でしか動かない（`BX-RUN-020`）
- 失敗はすべて `BX-RUN-010`（原因を含む）。ゼロでの除算に当たるものは `BX-RUN-001`
- 一覧は `boxundle std [NAMESPACE]` でも表示できる

| 名前空間 | 権限 | 内容 |
|---|---|---|
| `std.text` | none | 文字列 |
| `std.re` | none | 正規表現（Go の RE2 構文） |
| `std.math` | none | 平方根・べき乗 |
| `std.date` | none | 日付（`YYYY-MM-DD`） |
| `std.hash` | none | ハッシュ |
| `std.clock` | clock | 現在時刻 |
| `std.random` | random | 乱数 |
| `std.env` | env | 環境変数 |
| `std.fs` | fs.read / fs.write | ファイル（`--fs-root` の中だけ） |
| `std.http` | net | HTTP |

## 2. 宣言の文法

標準の箱は `boxundle` に同梱される宣言（`.bx`）で定義される。利用者はこの形式を書かない（書くと `BX070`）。型と記号は [language.md](language.md) と共通。

```ebnf
extern_file = "externs" QNAME { ext_typedef | extern } ;   (* QNAME は名前空間。std. で始まる *)
ext_typedef = "type" QNAME "=" type ;                      (* 名前は名前空間で始まること *)
extern      = "extern" "box" QNAME [ LABEL ] "{"
                { port } perm { test } impl
              "}" ;
impl        = "impl" "builtin" STRING ;                    (* Goの実装の登録名 *)
```

## 3. std.text（perm none）

| 箱 | 入力 | 出力 | 補足 |
|---|---|---|---|
| `std.text.split` | s: Str, sep: Str | parts: [Str] | sepが空なら `BX-RUN-010` |
| `std.text.join` | parts: [Str], sep: Str | s: Str | |
| `std.text.replace` | s: Str, old: Str, new: Str | s: Str | 全置換。oldが空なら `BX-RUN-010` |
| `std.text.trim` | s: Str | s: Str | 前後のUnicode空白を除く |
| `std.text.lower` / `std.text.upper` | s: Str | s: Str | |
| `std.text.starts_with` / `std.text.ends_with` / `std.text.contains` | s: Str, part: Str | result: Bool | |
| `std.text.slice` | s: Str, start: Int, end: Int | s: Str | コードポイント単位。負の値は末尾から数える。範囲は [0, 長さ] に丸める。start ≥ end なら空 |
| `std.text.parse_int` | s: Str | ok: Bool, value: Int | 前後の空白を許す。失敗時は ok=false, value=0 |
| `std.text.parse_dec` | s: Str | ok: Bool, value: Dec | 同上 |
| `std.text.pad_left` | s: Str, width: Int, fill: Str | s: Str | fillは1コードポイント。それ以外は `BX-RUN-010` |

## 4. std.re（perm none）

正規表現の構文は Go の `regexp`（RE2）に従う。

| 箱 | 入力 | 出力 |
|---|---|---|
| `std.re.matches` | pattern: Str, s: Str | result: Bool（全体一致） |
| `std.re.search` | pattern: Str, s: Str | result: Bool（部分一致） |
| `std.re.find_all` | pattern: Str, s: Str | found: [Str] |
| `std.re.replace` | pattern: Str, s: Str, repl: Str | s: Str（`$1` などのグループ参照はGoの `regexp` の規則） |

不正なパターンは `BX-RUN-010`。

## 5. std.math（perm none）

| 箱 | 入力 | 出力 | 補足 |
|---|---|---|---|
| `std.math.sqrt` | x: Dec | result: Dec | 有効桁28。xが負なら `BX-RUN-010` |
| `std.math.pow` | x: Dec, n: Int | result: Dec | nは負も可。x=0 かつ n<0 は `BX-RUN-001` |

## 6. std.date（perm none。日付は `YYYY-MM-DD` の文字列）

| 箱 | 入力 | 出力 |
|---|---|---|
| `std.date.is_valid` | date: Str | result: Bool |
| `std.date.add_days` | date: Str, days: Int | date: Str |
| `std.date.diff_days` | from: Str, to: Str | days: Int（to - from） |
| `std.date.weekday` | date: Str | weekday: Int（月曜=0） |

不正な日付は `BX-RUN-010`（`is_valid` を除く）。

## 7. std.hash（perm none）

| 箱 | 入力 | 出力 |
|---|---|---|
| `std.hash.sha256` | s: Str | hex: Str（UTF-8のバイト列のハッシュ。小文字16進） |

## 8. 作用のある箱

| 箱 | 権限 | 入力 | 出力 | 補足 |
|---|---|---|---|---|
| `std.clock.now` | clock | なし | now: Str | RFC 3339、秒精度、ローカルのオフセット付き |
| `std.clock.today` | clock | なし | date: Str | `YYYY-MM-DD`（ローカル） |
| `std.random.int` | random | lo: Int, hi: Int | value: Int | lo以上hi以下。lo > hi は `BX-RUN-010` |
| `std.random.uuid` | random | なし | id: Str | UUID v4（`crypto/rand`） |
| `std.env.get` | env | name: Str, default: Str | value: Str | |
| `std.fs.read_text` | fs.read | path: Str | text: Str | `--fs-root` からの相対パス。UTF-8でなければ `BX-RUN-010` |
| `std.fs.write_text` | fs.write | path: Str, text: Str | ok: Bool | 親ディレクトリがなければ `BX-RUN-010`。既存なら上書き |
| `std.http.get` | net | url: Str | status: Int, body: Str | |
| `std.http.post` | net | url: Str, body: Str, content_type: Str | status: Int, body: Str | |

- HTTPは `net/http` を使う。タイムアウト30秒、応答本文の上限10MB（超えたら `BX-RUN-010`）。本文は不正なUTF-8を置換文字にして返す。通信の失敗は `BX-RUN-010`（HTTPの4xx・5xxは失敗ではなく、status で返す）
- 作用のある箱はテスト（`boxundle test`）の中ではスタブが必須（なければ `BX-TEST-003`）

---

## 9. 宣言の例

`std.text`（抜粋）

```
externs std.text

extern box std.text.split @"文字列を区切り文字で分割する" {
  in $s : Str
  in $sep : Str
  out $parts : [Str]
  perm none

  test "カンマで分割" {
    given {
      s: "a,b,c",
      sep: ","
    }
    expect {
      parts: ["a", "b", "c"]
    }
  }

  impl builtin "text.split"
}

extern box std.text.parse_int @"文字列を整数として読む" {
  in $s : Str
  out $ok : Bool
  out $value : Int
  perm none

  test "数字なら読める" {
    given {
      s: " 42 "
    }
    expect {
      ok: true,
      value: 42
    }
  }

  test "数字でなければ失敗" {
    given {
      s: "abc"
    }
    expect {
      ok: false,
      value: 0
    }
  }

  impl builtin "text.parse_int"
}
```

`std.clock`（抜粋）

```
externs std.clock

extern box std.clock.now @"現在時刻を返す" {
  out $now : Str
  perm clock
  impl builtin "clock.now"
}
```
