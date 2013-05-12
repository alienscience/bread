package session

import (
	"sort"
)

type WordCount struct {
	Word  string
	Count int
}

type WordCounts []WordCount

func (wc WordCounts) Len() int {
	return len(wc)
}

func (wc WordCounts) Less(i, j int) bool {
	return wc[i].Count > wc[j].Count
}

func (wc WordCounts) Swap(i, j int) {
	wc[i], wc[j] = wc[j], wc[i]
}

func (wc WordCounts) Sort() {
	sort.Sort(wc)
}
