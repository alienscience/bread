/*
 * Pulls feeds 
 */

package index

import (
	"bread/config"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"time"
)

// A feed
type Feed struct {
	Name          string
	path          string
	refreshPeriod time.Duration
	url           string
	tick          <-chan time.Time
	client        http.Client
}

// Create a feed with the given filename that is updated every refresh period 
func NewFeed(name string, refreshPeriod time.Duration, url string) *Feed {

	// Create the feed
	feed := &Feed{
		Name:          name,
		path:          path.Join(indexDir, name),
		refreshPeriod: refreshPeriod,
		url:           url,
		tick:          time.Tick(refreshPeriod),
		client:        http.Client{}}

	// Create a go routine to process the feed
	go func() {
		pull(feed)

		for _ = range feed.tick {
			pull(feed)
		}
	}()

	return feed
}

// Pull a feed, dump it into a file and inform the indexer
func pull(feed *Feed) {

	// Only process local feeds if the server is not connected to the internet
	if config.Standalone {
		ReadFeed(feed.path)
		return
	}

	// Dump the feed to a temporary file
	res, err := feed.client.Get(feed.url)
	if err != nil {
		log.Println("Failure getting", feed.Name, ":", err)
		return
	}

	defer res.Body.Close()

	tmpname := path.Join(indexDir, feed.Name+".tmp")
	tmpfile, err := os.Create(tmpname)
	if err != nil {
		log.Println("Failure creating file", feed.Name, ".tmp:", err)
		return
	}

	io.Copy(tmpfile, res.Body)
	tmpfile.Close()

	// Move the file to the feed filename (assume atomic mv)
	os.Rename(tmpname, feed.path)

	// Inform the indexer
	ReadFeed(feed.path)
}
