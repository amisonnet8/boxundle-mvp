package value

import (
	"slices"
	"strconv"
	"strings"

	"github.com/cockroachdb/apd/v3"
)

// Key は、値の鍵を返す。Equal で等しい2つの値には同じ鍵を、等しくない値には違う鍵を返す
// （distinct が、要素を Equal で比べ直さずに、出現済みかを引くため）。
// Dec は数として（1.0 と 1.00、0 と -0 は同じ鍵）、レコードはフィールドの名前の順で鍵にする。
func Key(v Value) string {
	var b strings.Builder
	writeKey(&b, v)
	return b.String()
}

// writeKey は、v の鍵を b に書く。どの形も、どこで終わるかが自分で分かる（文字列は長さを先に書く）ので、並べても混ざらない。
func writeKey(b *strings.Builder, v Value) {
	switch v := v.(type) {
	case Int:
		b.WriteString("i" + v.V.String() + ";")
	case Dec:
		var r apd.Decimal
		r.Reduce(&v.V)
		if r.IsZero() {
			b.WriteString("d0;")
			return
		}
		sign := ""
		if r.Negative {
			sign = "-"
		}
		b.WriteString("d" + sign + r.Coeff.String() + "e" + strconv.Itoa(int(r.Exponent)) + ";")
	case Bool:
		if v {
			b.WriteString("t")
		} else {
			b.WriteString("f")
		}
	case Str:
		b.WriteString("s" + strconv.Itoa(len(v)) + ":" + string(v))
	case List:
		b.WriteString("l" + strconv.Itoa(len(v)) + ":")
		for _, x := range v {
			writeKey(b, x)
		}
	case Map:
		b.WriteString("m" + strconv.Itoa(len(v.Keys)) + ":")
		for i := range v.Keys {
			writeKey(b, v.Keys[i])
			writeKey(b, v.Vals[i])
		}
	case Record:
		b.WriteString("r" + strconv.Itoa(len(v.Tag)) + ":" + v.Tag + strconv.Itoa(len(v.Names)) + ":")
		idx := make([]int, len(v.Names))
		for i := range idx {
			idx[i] = i
		}
		slices.SortFunc(idx, func(x, y int) int { return strings.Compare(v.Names[x], v.Names[y]) })
		for _, i := range idx {
			b.WriteString(strconv.Itoa(len(v.Names[i])) + ":" + v.Names[i])
			writeKey(b, v.Fields[i])
		}
	default:
		panic("value.Key: 知らない値")
	}
}
