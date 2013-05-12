package index

import (
	"bread/config"
	"bread/db"
	"bread/rss"
	"bread/session"
	"bread/story"
	"log"
	"time"
)

const indexDir = "./index"

var feedCh = make(chan string, 8)

func addStory(newStories []*story.Story, rs *rss.Story) []*story.Story {

	id, seen := db.SeenStory(rs.Id)
	if seen {
		return newStories
	} else {
		id = db.AddStory(rs)
	}

	// Convert into a story struct
	s := story.FromRSS(id, rs)
	return append(newStories, s)
}

func readFeed(filename string) {
	config.Debug("Reading feed: ", filename)

	stories, err := rss.Decode(filename)
	if err != nil {
		log.Println("Cannot decode ", filename, " :", err)
		return
	}

	todo := make([]*story.Story, 0, 64)
	for _, s := range stories {
		todo = addStory(todo, s)
	}

	// Add any new stories
	session.AddStories(todo)
}

// Read the feed with the given filename
func ReadFeed(filename string) {
	feedCh <- filename
}

// The indexer function that is run within a go routine
func indexer() {
	for {
		select {
		case file := <-feedCh:
			readFeed(file)
		}
	}
}

// Start the indexing go routine
func Start() {
	go indexer()

	// Start pulling feeds
	NewFeed("hn.rss", 2*time.Hour, "http://news.ycombinator.com/bigrss")

}
