package story

import (
	"bread/nbc"
	"bread/rss"
)

type Story struct {
	Id       int64 // Numeric id assigned by the db
	Rss      rss.Story
	Wordlist []string
}

// Create a story from an rss story
func FromRSS(id int64, rs *rss.Story) *Story {

	wordlist := nbc.Wordlist(rs.Title + rs.Summary)

	return &Story{
		Id:       id,
		Rss:      *rs,
		Wordlist: wordlist}
}

