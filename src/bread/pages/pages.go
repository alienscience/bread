
package pages

import (
	"bread/session"
	"bread/config"
	"html/template"
	"net/http"
	"log"
	"path"
	"fmt"
)

// Package scope variables
var indexTemplate *template.Template
var profileTemplate *template.Template
var readTemplate *template.Template

// Get static content
func Static(w http.ResponseWriter, req *http.Request) {
	clean := path.Clean(req.URL.Path)
	file := path.Base(clean)
	http.ServeFile(w, req, "./static/"+file)
}

// Mark a story as read 
func Read(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()

	// Get the id of the read story
	qid := req.Form.Get("id")
	var storyid int64
	cnt, err := fmt.Sscan( qid, &storyid)
	if err != nil {
		log.Println("Cannot read storyid: ", err)
	} else if cnt == 1 {
		session.MarkRead(w, req, storyid)

		if ( !config.Standalone ) {
			story, ok := session.GetStory(storyid)
			if ok {
				http.Redirect(w, req, story.Rss.Link, http.StatusTemporaryRedirect)
				return
			}
		}
	}

	// Fall back to just redisplaying the index 
	http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
}

// Read a story again
func ReadAgain(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()

	// Get the id of the read story
	qid := req.Form.Get("id")
	var storyid int64
	cnt, err := fmt.Sscan( qid, &storyid)
	if err != nil {
		log.Println("Cannot read storyid: ", err)
	} else if cnt == 1 && !config.Standalone {
		story, ok := session.GetStory(storyid)
		if ok {
			http.Redirect(w, req, story.Rss.Link, http.StatusTemporaryRedirect)
			return
		}
	}

	// Fall back to just redisplaying the index 
	http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
}

// Get the comments for a story
func Comments(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()

	// Get the id of the read story
	qid := req.Form.Get("id")
	var storyid int64
	cnt, err := fmt.Sscan( qid, &storyid)
	if err != nil {
		log.Println("Cannot read storyid: ", err)
	} else if cnt == 1 {
		session.MarkRead(w, req, storyid)
	}

	if ( !config.Standalone ) {
		story, ok := session.GetStory(storyid)
		if ok {
			http.Redirect(w, req, story.Rss.Comments, http.StatusTemporaryRedirect)
			return
		}
	}

	// Fall back to just redisplaying the index 
	http.Redirect(w, req, "/", http.StatusTemporaryRedirect)
}

// Display the homepage
func Home(w http.ResponseWriter, req *http.Request) {

	// Only accept the root path 
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}

	// Request the stories 
	stories := session.FilteredStories(w, req, 0)

	// Display the index page
	index(w, stories)
}

// Get the next page
func Next(w http.ResponseWriter, req *http.Request) {

	req.ParseForm()

	// Get the id of the next story to get
	qid := req.Form.Get("id")
	var storyid int64
	cnt, _ := fmt.Sscan( qid, &storyid)
	if cnt != 1 {
		storyid = 0
	}

	// Mark the stories up to here as being browsed
	session.MarkBrowsed(w, req, storyid)

	// Request more stories
	stories := session.FilteredStories(w, req, storyid)

	// Display the index page
	index(w, stories)

}

// Get the previous page
func Previous(w http.ResponseWriter, req *http.Request) {

	req.ParseForm()

	// Get the id of the previous story to get
	qid := req.Form.Get("id")
	var storyid int64
	cnt, _ := fmt.Sscan( qid, &storyid)
	if cnt != 1 {
		storyid = 0
	}

	// Request the previous stories
	stories := session.UnfilteredStories(w, req, storyid)

	// Display the index page
	index(w, stories)
}

// Stories that have been read
func HaveRead(w http.ResponseWriter, req *http.Request) {

	// Request the read stories
	stories := session.HaveReadStories(w, req)

	// Display the have read page
	err := readTemplate.Execute(w, stories)
	if err != nil {
		log.Println("Executing read.tmpl: ", err)
	}
}

// Display an index of stories
func index(w http.ResponseWriter, i *session.StoryIndex) {

	// Display the index page
	err := indexTemplate.Execute(w, i)
	if err != nil {
		log.Println("Executing index.tmpl: ", err)
	}
}

// Show the users profile
func Profile(w http.ResponseWriter, req *http.Request) {
	// Request the user's profile
	profile := session.Profile(w, req)

	// Display the profile page
	err := profileTemplate.Execute(w, profile)
	if err != nil {
		log.Println("Executing index.tmpl: ", err)
	}
}

// Parse required templates
func Start() {

	var err error
	indexTemplate, err = template.ParseFiles("templates/index.tmpl")
	if err != nil {
		log.Fatal("Parsing index.tmpl: ", err)
	}

	readTemplate, err = template.ParseFiles("templates/read.tmpl")
	if err != nil {
		log.Fatal("Parsing read.tmpl: ", err)
	}

	profileTemplate, err = template.ParseFiles("templates/profile.tmpl")
	if err != nil {
		log.Fatal("Parsing profile.tmpl: ", err)
	}
}

