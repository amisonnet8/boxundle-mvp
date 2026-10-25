package check

import (
	"slices"
	"testing"
)

func TestDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"", "abc", 3},
		{"abc", "", 3},
		{"kitten", "sitting", 3},
		{"Lin", "Line", 1},
		{"Ordr", "Order", 1},
		{"ab", "ba", 2},
		{"注文", "注文書", 1}, // コードポイント単位（バイトなら3）
		{"在庫", "在個", 1},
	}
	for _, tt := range tests {
		if got := distance(tt.a, tt.b); got != tt.want {
			t.Errorf("distance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestSuggest(t *testing.T) {
	cands := []string{"split", "splits", "spl", "sp", "join", "split", "spilt"}
	// split からの距離：splits 1、spl 2、sp 3、join 5、spilt 2。同じ名前の split は除く
	if got, want := suggest("split", cands), []string{"splits", "spilt", "spl"}; !slices.Equal(got, want) {
		t.Errorf("suggest(split) = %q, want %q", got, want)
	}
	// splt からの距離：spilt 1（i を足す）、spl 1、split 1（2回あっても1つ）、sp 2、splits 2
	if got, want := suggest("splt", cands), []string{"spilt", "spl", "split", "sp", "splits"}; !slices.Equal(got, want) {
		t.Errorf("suggest(splt) = %q, want %q", got, want)
	}
	if got := suggest("zzzzzz", cands); len(got) != 0 {
		t.Errorf("suggest(zzzzzz) = %q, want 空", got)
	}
}
