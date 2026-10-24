> [docs](../README.md) / examples

# 例

| ファイル | 内容 |
|---|---|
| `order.bx` | 注文を処理する箱 `B7`。在庫が全部足りれば請求書を出して在庫を減らし、足りなければ不足商品を返す |
| `checkout.bx` | `order.bx` の内容に、`B7` と `std.clock.now` を呼ぶ箱 `Checkout` を加えたもの（MVPには複数ファイルの読み込みがないため、同じ内容を含めている） |
| `order_inputs.json` | `B7` の入力 |
| `order_expected.json` | その入力に対する `B7` の期待される出力 |

```sh
boxundle check order.bx
boxundle test .
boxundle show checkout.bx Checkout --depth 1
boxundle run order.bx B7 --input order_inputs.json
boxundle run checkout.bx Checkout --input order_inputs.json --allow clock
```
