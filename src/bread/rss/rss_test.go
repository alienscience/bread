package rss

import (
	"testing"
)

func TestExtractId(t *testing.T) {
	id := extractId("http://news.ycombinator.com/item?id=3725302")
	if id != "3725302HN" {
		t.Error(id, "!=", "3725302HN")
	}
}

func TestDecode(t *testing.T) {

	items, err := Decode("testdata/big.rss")
	if err != nil {
		t.Error(err)
	}

	t.Error(items[0])
}
