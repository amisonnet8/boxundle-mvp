package value

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cockroachdb/apd/v3"

	"github.com/amisonnet8/boxundle-mvp/internal/types"
)

// Error は、JSON を値にできなかった理由。Path は値の位置で、呼び出し側が渡した根から
// .フィールド・["キー"]・[番号] でたどる（order.lines[0].qty）。
type Error struct {
	Path string
	Msg  string
}

func (e *Error) Error() string {
	if e.Path == "" {
		return e.Msg
	}
	return e.Path + ": " + e.Msg
}

// FromJSON は、data を型 t の値として読む（language.md §7.1）。path は誤りの位置の根（ポート名など）。
func FromJSON(t types.Type, data []byte, path string) (Value, error) {
	dec := newDecoder(data)
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return nil, syntaxError(path, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, &Error{path, "値のあとに、余分なものがあります"}
	}
	return decode(t, raw, path)
}

func newDecoder(data []byte) *json.Decoder {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber() // 大きな整数・小数の精度を落とさない
	return dec
}

func syntaxError(path string, err error) error {
	var se *json.SyntaxError
	switch {
	case errors.As(err, &se):
		return &Error{path, fmt.Sprintf("JSON の構文の誤りです（%d バイト目）", se.Offset)}
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return &Error{path, "JSON が途中で終わっています"}
	}
	return &Error{path, "JSON として読めません"}
}

// decNumber は、文字列で書いた Dec の書き方（JSON の数値と同じ）。
var decNumber = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

// decode は、構文の正しい1つの JSON の値 raw を、型 t の値にする。
func decode(t types.Type, raw json.RawMessage, path string) (Value, error) {
	dec := newDecoder(raw)
	tok, _ := dec.Token() // raw は Decode が読んだ1つの値なので、誤りは無い
	mismatch := func() error {
		return &Error{path, fmt.Sprintf("%sが要りますが、%sがあります", expectName(t), tokenName(tok))}
	}
	switch t := t.(type) {
	case types.Basic:
		return decodeBasic(t, tok, path, mismatch)
	case *types.List:
		if tok != json.Delim('[') {
			return nil, mismatch()
		}
		list := List{}
		for i := 0; dec.More(); i++ {
			var elem json.RawMessage
			_ = dec.Decode(&elem)
			v, err := decode(t.Elem, elem, fmt.Sprintf("%s[%d]", path, i))
			if err != nil {
				return nil, err
			}
			list = append(list, v)
		}
		return list, nil
	case *types.Map:
		if t.Key == types.Str {
			return decodeStrMap(t, dec, tok, path, mismatch)
		}
		return decodePairMap(t, dec, tok, path, mismatch)
	case *types.Record:
		if tok != json.Delim('{') {
			return nil, mismatch()
		}
		names, raws, err := readObject(dec, path)
		if err != nil {
			return nil, err
		}
		return decodeRecord(t, names, raws, path)
	case *types.Union:
		if tok != json.Delim('{') {
			return nil, mismatch()
		}
		names, raws, err := readObject(dec, path)
		if err != nil {
			return nil, err
		}
		tagRaw, ok := raws["$tag"]
		if !ok {
			return nil, &Error{path, fmt.Sprintf("ユニオン型 %s の位置には \"$tag\" が要ります", t)}
		}
		var tag string
		if json.Unmarshal(tagRaw, &tag) != nil {
			return nil, &Error{path + ".$tag", "$tag には型名の文字列を書きます"}
		}
		i := slices.IndexFunc(t.Members, func(m *types.Record) bool { return m.Name == tag })
		if i < 0 {
			return nil, &Error{path + ".$tag", fmt.Sprintf("%s は、ユニオン型 %s のどの型でもありません", tag, t)}
		}
		return decodeRecord(t.Members[i], names, raws, path)
	}
	panic(fmt.Sprintf("value.decode: 知らない型 %T", t))
}

func decodeBasic(t types.Basic, tok json.Token, path string, mismatch func() error) (Value, error) {
	switch t {
	case types.Int:
		n, ok := tok.(json.Number)
		if !ok {
			return nil, mismatch()
		}
		if strings.ContainsAny(string(n), ".eE") {
			return nil, &Error{path, fmt.Sprintf("整数が要りますが、%s は小数点か指数を含みます", n)}
		}
		v, _ := new(big.Int).SetString(string(n), 10) // JSON の数値で、小数点も指数も無いので読める
		return Int{V: v}, nil
	case types.Dec:
		var s string
		switch tok := tok.(type) {
		case json.Number:
			s = string(tok)
		case string:
			s = tok
			if !decNumber.MatchString(s) {
				return nil, &Error{path, fmt.Sprintf("%q は小数として読めません", s)}
			}
		default:
			return nil, mismatch()
		}
		d, _, err := apd.NewFromString(s)
		if err != nil {
			return nil, &Error{path, fmt.Sprintf("%s は小数として扱える範囲を超えています", s)}
		}
		return Dec{V: *d}, nil
	case types.Bool:
		b, ok := tok.(bool)
		if !ok {
			return nil, mismatch()
		}
		return Bool(b), nil
	case types.Str:
		s, ok := tok.(string)
		if !ok {
			return nil, mismatch()
		}
		return Str(s), nil
	}
	panic(fmt.Sprintf("value.decode: 知らない型 %v", t))
}

// decodeStrMap は、{Str -> V} をオブジェクトから読む。
func decodeStrMap(t *types.Map, dec *json.Decoder, tok json.Token, path string, mismatch func() error) (Value, error) {
	if tok != json.Delim('{') {
		return nil, mismatch()
	}
	names, raws, err := readObject(dec, path)
	if err != nil {
		return nil, err
	}
	keys := make([]Value, len(names))
	vals := make([]Value, len(names))
	for i, name := range names {
		v, err := decode(t.Value, raws[name], path+"["+strconv.Quote(name)+"]")
		if err != nil {
			return nil, err
		}
		keys[i], vals[i] = Str(name), v
	}
	return NewMap(keys, vals), nil
}

// decodePairMap は、{Int -> V}・{Bool -> V} を [[キー, 値], ...] から読む。
func decodePairMap(t *types.Map, dec *json.Decoder, tok json.Token, path string, mismatch func() error) (Value, error) {
	if tok != json.Delim('[') {
		return nil, mismatch()
	}
	var keys, vals []Value
	for i := 0; dec.More(); i++ {
		var pair json.RawMessage
		_ = dec.Decode(&pair)
		at := fmt.Sprintf("%s[%d]", path, i)
		var parts []json.RawMessage
		if json.Unmarshal(pair, &parts) != nil || len(parts) != 2 {
			return nil, &Error{at, "[キー, 値] の2つを並べた配列が要ります"}
		}
		k, err := decode(t.Key, parts[0], at+"[0]")
		if err != nil {
			return nil, err
		}
		v, err := decode(t.Value, parts[1], at+"[1]")
		if err != nil {
			return nil, err
		}
		if slices.ContainsFunc(keys, func(x Value) bool { return Equal(x, k) }) {
			return nil, &Error{at, fmt.Sprintf("キー %s が重なっています", parts[0])}
		}
		keys, vals = append(keys, k), append(vals, v)
	}
	return NewMap(keys, vals), nil
}

// readObject は、オブジェクトの中身を、書かれた順の名前と、名前ごとの値の JSON にする。名前の重なりは誤り。
func readObject(dec *json.Decoder, path string) ([]string, map[string]json.RawMessage, error) {
	var names []string
	raws := map[string]json.RawMessage{}
	for dec.More() {
		tok, _ := dec.Token()
		name := tok.(string) // オブジェクトの中で、値の前は必ず名前
		var raw json.RawMessage
		_ = dec.Decode(&raw)
		if _, ok := raws[name]; ok {
			return nil, nil, &Error{path, fmt.Sprintf("%s が重なっています", strconv.Quote(name))}
		}
		names = append(names, name)
		raws[name] = raw
	}
	return names, raws, nil
}

// decodeRecord は、オブジェクトの中身を、レコード型 t の値にする。フィールドは過不足なく要る。
// 名前付きレコード型では "$tag" を書いてよく、書くなら型名と同じにする。
func decodeRecord(t *types.Record, names []string, raws map[string]json.RawMessage, path string) (Value, error) {
	for _, name := range names {
		if name == "$tag" && t.Name != "" {
			var tag string
			if json.Unmarshal(raws[name], &tag) != nil || tag != t.Name {
				return nil, &Error{path + ".$tag", fmt.Sprintf("$tag には %q を書きます", t.Name)}
			}
			continue
		}
		if _, ok := t.Field(name); !ok {
			return nil, &Error{path, fmt.Sprintf("レコード型 %s にフィールド %s はありません", t, name)}
		}
	}
	r := Record{Tag: t.Name, Names: make([]string, len(t.Fields)), Fields: make([]Value, len(t.Fields))}
	for i, f := range t.Fields {
		raw, ok := raws[f.Name]
		if !ok {
			return nil, &Error{path, fmt.Sprintf("フィールド %s がありません", f.Name)}
		}
		v, err := decode(f.Type, raw, path+"."+f.Name)
		if err != nil {
			return nil, err
		}
		r.Names[i], r.Fields[i] = f.Name, v
	}
	return r, nil
}

// expectName は、型 t の位置に要る JSON の値の言い方。
func expectName(t types.Type) string {
	switch t := t.(type) {
	case types.Basic:
		return map[types.Basic]string{types.Int: "整数", types.Dec: "小数（数値か文字列）", types.Bool: "真偽値", types.Str: "文字列"}[t]
	case *types.List:
		return "配列"
	case *types.Map:
		if t.Key == types.Str {
			return "オブジェクト"
		}
		return "[キー, 値] の配列"
	}
	return "オブジェクト"
}

// tokenName は、JSON の値の始まり tok の言い方。
func tokenName(tok json.Token) string {
	switch tok := tok.(type) {
	case json.Delim:
		if tok == '[' {
			return "配列"
		}
		return "オブジェクト"
	case bool:
		return "真偽値"
	case json.Number:
		return "数値"
	case string:
		return "文字列"
	}
	return "null"
}

// ToJSON は、型 t の値 v を JSON にする（language.md §7.1）。Dec は指数表記を使わない文字列、
// 名前付きレコードは "$tag" を先頭に置き、マップはキーの昇順。v が t に合わなければ boxundle の不具合で、panic する。
func ToJSON(t types.Type, v Value) []byte {
	return appendJSON(nil, t, v)
}

func appendJSON(b []byte, t types.Type, v Value) []byte {
	switch v := v.(type) {
	case Int:
		return v.V.Append(b, 10)
	case Dec:
		return appendString(b, v.V.Text('f'))
	case Bool:
		return strconv.AppendBool(b, bool(v))
	case Str:
		return appendString(b, string(v))
	case List:
		elem := t.(*types.List).Elem
		b = append(b, '[')
		for i, e := range v {
			if i > 0 {
				b = append(b, ',')
			}
			b = appendJSON(b, elem, e)
		}
		return append(b, ']')
	case Map:
		mt := t.(*types.Map)
		if mt.Key == types.Str {
			b = append(b, '{')
			for i, k := range v.Keys {
				if i > 0 {
					b = append(b, ',')
				}
				b = appendString(b, string(k.(Str)))
				b = append(b, ':')
				b = appendJSON(b, mt.Value, v.Vals[i])
			}
			return append(b, '}')
		}
		b = append(b, '[')
		for i, k := range v.Keys {
			if i > 0 {
				b = append(b, ',')
			}
			b = append(b, '[')
			b = appendJSON(b, mt.Key, k)
			b = append(b, ',')
			b = appendJSON(b, mt.Value, v.Vals[i])
			b = append(b, ']')
		}
		return append(b, ']')
	case Record:
		rt := recordType(t, v.Tag)
		b = append(b, '{')
		if v.Tag != "" {
			b = appendString(b, "$tag")
			b = append(b, ':')
			b = appendString(b, v.Tag)
		}
		for i, name := range v.Names {
			if i > 0 || v.Tag != "" {
				b = append(b, ',')
			}
			ft, _ := rt.Field(name)
			b = appendString(b, name)
			b = append(b, ':')
			b = appendJSON(b, ft, v.Fields[i])
		}
		return append(b, '}')
	}
	panic(fmt.Sprintf("value.ToJSON: 知らない値 %T", v))
}

// recordType は、レコードの値の型を、レコード型かユニオン型 t から、タグで引く。
func recordType(t types.Type, tag string) *types.Record {
	if u, ok := t.(*types.Union); ok {
		for _, m := range u.Members {
			if m.Name == tag {
				return m
			}
		}
		panic(fmt.Sprintf("value.ToJSON: %s は %s の型ではありません", tag, u))
	}
	return t.(*types.Record)
}

// appendString は、s を JSON の文字列にして足す。<・>・& はそのまま書く。
func appendString(b []byte, s string) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // 文字列は必ず符号化できる
	return append(b, bytes.TrimSuffix(buf.Bytes(), []byte("\n"))...)
}
