package check

import (
	"cmp"
	"slices"
)

// maxSuggestDistance は、候補にする編集距離の上限（language.md §6.1）。
const maxSuggestDistance = 2

// suggest は、candidates のうち name との編集距離が2以内のものを、近い順（同じなら名前の順）に返す。
// 同じ名前は候補にしない。
func suggest(name string, candidates []string) []string {
	type scored struct {
		name string
		dist int
	}
	var found []scored
	for _, c := range candidates {
		if c == name {
			continue
		}
		if d := distance(name, c); d <= maxSuggestDistance {
			found = append(found, scored{c, d})
		}
	}
	slices.SortFunc(found, func(a, b scored) int {
		return cmp.Or(cmp.Compare(a.dist, b.dist), cmp.Compare(a.name, b.name))
	})
	names := make([]string, 0, len(found))
	for _, s := range found {
		if !slices.Contains(names, s.name) {
			names = append(names, s.name)
		}
	}
	return names
}

// distance は、コードポイント単位のレーベンシュタイン距離（挿入・削除・置換がそれぞれ1）。
func distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur := make([]int, len(rb)+1)
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(rb)]
}
