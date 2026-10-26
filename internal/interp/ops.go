package interp

import (
	"fmt"
	"math/big"
	"slices"
	"unicode/utf8"

	"github.com/cockroachdb/apd/v3"

	"github.com/amisonnet8/boxundle-mvp/internal/diag"
	"github.com/amisonnet8/boxundle-mvp/internal/types"
	"github.com/amisonnet8/boxundle-mvp/internal/value"
)

// rangeLimit は、range が作れるリストの長さの上限（language.md §4.3）。
const rangeLimit = 1_000_000

// opError は、組み込み演算の実行時の誤り。評価器が、箱とノードの位置を付けた診断にする。
type opError struct {
	code diag.Code
	msg  string
}

func (e *opError) Error() string { return string(e.code) + ": " + e.msg }

func errorf(code diag.Code, format string, args ...any) *opError {
	return &opError{code: code, msg: fmt.Sprintf(format, args...)}
}

// apply は、ブロックを取らない組み込み演算 name を、評価した引数 args に当てはめる。
// typ はノードの値の型で、値から決まらないもの（空のリストの sum）にだけ使う。引数の型は検証が確かめている。
func apply(name string, args []value.Value, typ types.Type) (value.Value, error) {
	switch name {
	case "val":
		return args[0], nil
	case "add", "sub", "mul", "min", "max":
		if _, ok := args[0].(value.Int); ok {
			return intArith(name, ints(args[0]), ints(args[1])), nil
		}
		return decArith(name, decs(args[0]), decs(args[1]))
	case "div":
		return decArith(name, decs(args[0]), decs(args[1]))
	case "idiv", "mod":
		b := ints(args[1])
		if b.Sign() == 0 {
			return nil, errorf(diag.BXRun001, "%s の除数が 0 です", name)
		}
		q, r := floorDivMod(ints(args[0]), b)
		if name == "idiv" {
			return value.Int{V: q}, nil
		}
		return value.Int{V: r}, nil
	case "neg", "abs":
		if a, ok := args[0].(value.Int); ok {
			if name == "neg" {
				return value.Int{V: new(big.Int).Neg(a.V)}, nil
			}
			return value.Int{V: new(big.Int).Abs(a.V)}, nil
		}
		f := value.DecContext().Neg
		if name == "abs" {
			f = value.DecContext().Abs
		}
		return decResult(name, func(d *apd.Decimal) (apd.Condition, error) { return f(d, decs(args[0])) })
	case "to_dec":
		return value.Dec{V: *apd.NewWithBigInt(new(apd.BigInt).SetMathBigInt(ints(args[0])), 0)}, nil
	case "floor":
		return value.Int{V: floorDec(decs(args[0]))}, nil
	case "round":
		return round(decs(args[0]), ints(args[1]))
	case "to_str":
		switch a := args[0].(type) {
		case value.Int:
			return value.Str(a.V.String()), nil
		case value.Dec:
			return value.Str(a.V.Text('f')), nil
		case value.Bool:
			if a {
				return value.Str("true"), nil
			}
			return value.Str("false"), nil
		}
	case "eq":
		return value.Bool(value.Equal(args[0], args[1])), nil
	case "ne":
		return value.Bool(!value.Equal(args[0], args[1])), nil
	case "lt":
		return value.Bool(value.Compare(args[0], args[1]) < 0), nil
	case "le":
		return value.Bool(value.Compare(args[0], args[1]) <= 0), nil
	case "gt":
		return value.Bool(value.Compare(args[0], args[1]) > 0), nil
	case "ge":
		return value.Bool(value.Compare(args[0], args[1]) >= 0), nil
	case "and":
		return args[0].(value.Bool) && args[1].(value.Bool), nil
	case "or":
		return args[0].(value.Bool) || args[1].(value.Bool), nil
	case "not":
		return !args[0].(value.Bool), nil
	case "concat":
		if a, ok := args[0].(value.Str); ok {
			return a + args[1].(value.Str), nil
		}
		return slices.Concat(args[0].(value.List), args[1].(value.List)), nil
	case "len", "empty":
		var n int
		switch a := args[0].(type) {
		case value.Str:
			n = utf8.RuneCountInString(string(a))
		case value.List:
			n = len(a)
		case value.Map:
			n = len(a.Keys)
		}
		if name == "len" {
			return value.NewInt(int64(n)), nil
		}
		return value.Bool(n == 0), nil
	case "sum":
		return sum(args[0].(value.List), typ)
	case "reverse":
		out := slices.Clone(args[0].(value.List))
		slices.Reverse(out)
		return out, nil
	case "take", "drop":
		xs := args[0].(value.List)
		n := clampLen(ints(args[1]), len(xs))
		if name == "take" {
			return slices.Clone(xs[:n]), nil
		}
		return slices.Clone(xs[n:]), nil
	case "distinct":
		out := value.List{}
		seen := map[string]bool{}
		for _, x := range args[0].(value.List) {
			if k := value.Key(x); !seen[k] {
				seen[k] = true
				out = append(out, x)
			}
		}
		return out, nil
	case "range":
		n := ints(args[0])
		if n.Cmp(big.NewInt(rangeLimit)) > 0 {
			return nil, errorf(diag.BXRun003, "range の大きさ %s が上限 %d を超えています", n, rangeLimit)
		}
		out := value.List{}
		if n.Sign() <= 0 {
			return out, nil
		}
		for i := range n.Int64() {
			out = append(out, value.NewInt(i))
		}
		return out, nil
	case "lookup":
		if v, ok := args[0].(value.Map).Get(args[1]); ok {
			return v, nil
		}
		return args[2], nil
	case "has":
		_, ok := args[0].(value.Map).Get(args[1])
		return value.Bool(ok), nil
	case "set":
		return args[0].(value.Map).Set(args[1], args[2]), nil
	case "remove":
		return args[0].(value.Map).Remove(args[1]), nil
	case "keys":
		return value.List(slices.Clone(args[0].(value.Map).Keys)), nil
	case "values":
		return value.List(slices.Clone(args[0].(value.Map).Vals)), nil
	}
	panic(fmt.Sprintf("interp: 演算 %s を %d 個の引数に当てはめられません", name, len(args)))
}

func ints(v value.Value) *big.Int { return v.(value.Int).V }

func decs(v value.Value) *apd.Decimal {
	d := v.(value.Dec)
	return &d.V
}

func intArith(name string, a, b *big.Int) value.Value {
	switch name {
	case "add":
		return value.Int{V: new(big.Int).Add(a, b)}
	case "sub":
		return value.Int{V: new(big.Int).Sub(a, b)}
	case "mul":
		return value.Int{V: new(big.Int).Mul(a, b)}
	case "min":
		if a.Cmp(b) <= 0 {
			return value.Int{V: a}
		}
		return value.Int{V: b}
	}
	if a.Cmp(b) >= 0 { // max
		return value.Int{V: a}
	}
	return value.Int{V: b}
}

// decArith は、2つの Dec の演算。有効桁28・HALF_EVEN に丸める。
// min・max は Python の Decimal.min・max と同じく、数として等しければ全順序（1.00 < 1.0）で選ぶ（q 6b850b78e2）。
func decArith(name string, x, y *apd.Decimal) (value.Value, error) {
	c := value.DecContext()
	return decResult(name, func(d *apd.Decimal) (apd.Condition, error) {
		switch name {
		case "add", "sum":
			return c.Add(d, x, y)
		case "sub":
			return c.Sub(d, x, y)
		case "mul":
			return c.Mul(d, x, y)
		case "div":
			cond, err := c.Quo(d, x, y)
			if err == nil && cond&apd.Inexact == 0 {
				idealExponent(d, x.Exponent-y.Exponent)
			}
			return cond, err
		}
		cmp := x.Cmp(y)
		if cmp == 0 {
			cmp = x.CmpTotal(y)
		}
		pick := x
		if (name == "max" && cmp < 0) || (name == "min" && cmp > 0) {
			pick = y
		}
		return c.Round(d, pick)
	})
}

// decResult は、f が書いた Dec を返す。apd の誤りは、実行時の誤りにする（q 6b850b78e2：指数の範囲を超えたら BX-RUN-003）。
func decResult(name string, f func(d *apd.Decimal) (apd.Condition, error)) (value.Value, error) {
	var out value.Dec
	cond, err := f(&out.V)
	if err == nil {
		return out, nil
	}
	if cond&(apd.DivisionByZero|apd.DivisionUndefined) != 0 {
		return nil, errorf(diag.BXRun001, "%s の除数が 0 です", name)
	}
	return nil, errorf(diag.BXRun003, "%s の結果が Dec の範囲（指数 %d〜%d）を超えます", name, apd.MinExponent, apd.MaxExponent)
}

// idealExponent は、割り切れた商 d の末尾の0を、指数が ideal になるまで除く（Python の decimal の除算と同じ）。0 なら指数を ideal にする。
func idealExponent(d *apd.Decimal, ideal int32) {
	if d.IsZero() {
		d.Exponent = ideal
		return
	}
	coeff := d.Coeff.MathBigInt()
	ten := big.NewInt(10)
	q, r := new(big.Int), new(big.Int)
	for d.Exponent < ideal {
		if q.QuoRem(coeff, ten, r); r.Sign() != 0 {
			break
		}
		coeff.Set(q)
		d.Exponent++
	}
	d.Coeff.SetMathBigInt(coeff)
}

// floorDivMod は、床除算の商と余りを返す（余りの符号は除数と同じ）。b は 0 でない。
func floorDivMod(a, b *big.Int) (q, r *big.Int) {
	q, r = new(big.Int).QuoRem(a, b, new(big.Int))
	if r.Sign() != 0 && r.Sign() != b.Sign() {
		q.Sub(q, big.NewInt(1))
		r.Add(r, b)
	}
	return q, r
}

// floorDec は、x 以下で最大の整数を、丸めずに返す（Python の math.floor と同じ）。
func floorDec(x *apd.Decimal) *big.Int {
	n := x.Coeff.MathBigInt()
	if x.Negative {
		n.Neg(n)
	}
	if x.Exponent >= 0 {
		return n.Mul(n, pow10(int64(x.Exponent)))
	}
	q, _ := floorDivMod(n, pow10(-int64(x.Exponent)))
	return q
}

func pow10(e int64) *big.Int { return new(big.Int).Exp(big.NewInt(10), big.NewInt(e), nil) }

// round は、x を小数点以下 places 桁に HALF_EVEN で丸める。桁が足りなければ 0 を足して揃える
// （Python の round(Decimal, n) と同じ。q 6b850b78e2）。結果が有効桁28に収まらなければ BX-RUN-002。
func round(x *apd.Decimal, places *big.Int) (value.Value, error) {
	if places.Sign() < 0 {
		return nil, errorf(diag.BXRun002, "round の桁数 %s が負です", places)
	}
	fail := errorf(diag.BXRun002, "round の結果が有効桁 28 に収まりません（桁数 %s）", places)
	if places.Cmp(big.NewInt(-apd.MinExponent)) > 0 {
		return nil, fail
	}
	var out value.Dec
	if _, err := value.DecContext().Quantize(&out.V, x, int32(-places.Int64())); err != nil { //nolint:gosec // places はすぐ上で 0〜100000 に限った
		return nil, fail
	}
	return out, nil
}

// sum は、リストの和。空なら、ノードの型（Int か Dec）の 0。
func sum(xs value.List, typ types.Type) (value.Value, error) {
	if typ == types.Int {
		total := new(big.Int)
		for _, x := range xs {
			total.Add(total, ints(x))
		}
		return value.Int{V: total}, nil
	}
	var total value.Value = value.Dec{}
	for _, x := range xs {
		v, err := decArith("sum", decs(total), decs(x))
		if err != nil {
			return nil, err
		}
		total = v
	}
	return total, nil
}

// clampLen は、take・drop の n を 0〜length に収める。
func clampLen(n *big.Int, length int) int {
	switch {
	case n.Sign() < 0:
		return 0
	case n.Cmp(big.NewInt(int64(length))) > 0:
		return length
	}
	return int(n.Int64())
}
