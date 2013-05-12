/*******************************************************************************
 * Decodes RSS feeds
 ******************************************************************************/

package rss

import (
	"encoding/xml"
	"log"
	"os"
	"regexp"
)

type Story struct {
	Id       string
	Title    string `xml:"title"`
	Link     string `xml:"link"`
	Comments string `xml:"comments"`
	Summary  string
}

type Channel struct {
	Title       string   `xml:"title"`
	Description string   `xml:"description"`
	Items       []*Story `xml:"item"`
}

type RSS struct {
	Ch Channel `xml:"channel"`
}

// Decode the given file containing an RSS feed
func Decode(filename string) ([]*Story, error) {

	// Get a reader for the given file
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Create an XML decoder
	dec := xml.NewDecoder(file)

	// Unmarshall the XML into a slice of Items
	feed := new(RSS)
	err = dec.Decode(feed)
	if err != nil {
		return nil, err
	}

	log.Println("Feed title, ", feed.Ch.Title)
	log.Println("Feed description, ", feed.Ch.Description)

	// Loop through the items and extract an id
	for i, _ := range feed.Ch.Items {
		feed.Ch.Items[i].Id = extractId(feed.Ch.Items[i].Comments)
	}

	return feed.Ch.Items, nil
}

var idRe = regexp.MustCompile(`id=(\d+)`)

// Extract the id of a story from a comment link
func extractId(link string) string {
	matches := idRe.FindStringSubmatch(link)
	if matches == nil {
		log.Println("Cannot extract HN id from <", link, ">")
		return "unknownHN"
	}

	return matches[1] + "HN"
}
