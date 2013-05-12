package session

import (
	"bread/config"
	"bread/db"
	"bread/nbc"
	"bread/story"
	"bytes"
	"cache"
	"encoding/gob"
	"log"
	"net/http"
	"sync"
	"time"
)

// The maximum number of stories in the queue
const MaxStories = 512

// The number of stories back in the queue to start from for new users
const DefaultStart = 256

// The classes of stories
const (
	Interesting, InterestingPrior     = iota, 0.2
	Uninteresting, UninterestingPrior = iota, 0.8
)

const storiesPerPage = 10
const interestingPerPage = 2

type Session struct {
	id          string
	isNew       bool
	classifier  *nbc.Classifier
	haveRead    map[int64]bool // Stories that have been read
	haveIgnored map[int64]bool // Stories that have been ignored
	filtered    []*story.Story // The current filtered stories
	unfiltered  []*story.Story // The current unfiltered stories
	haveBrowsed int64          // Keep track of how far a user has browsed

	// A marker used to minimise the search for interesting stories
	haveClassified int64

	// A mutex to protect sessions from multiple requests. Contention for
	// a single session should be low.
	mutex sync.Mutex
}

// Get the session key/id
func (s *Session) Key() string {
	return s.id
}

type userStory struct {
	sessionid string
	storyid   int64
}

type StoryIndex struct {
	Previous     int64
	Next         int64
	HavePrevious bool
	HaveNext     bool
	Filtered     []*story.Story
	Unfiltered   []*story.Story
}

type UserProfile struct {
	Interesting   WordCounts
	Uninteresting WordCounts
}

var stories = newFifo(MaxStories)

// Sessions are held in the DB and are cached in memory
var sessions = cache.New(1024, 5*time.Minute, readSession, saveSession)

var storyCh = make(chan []*story.Story)
var readCh = make(chan userStory, 8)

// Add stories so that they are available to all users
func AddStories(s []*story.Story) {
	storyCh <- s
}

// Mark a story as read
func MarkRead(w http.ResponseWriter, req *http.Request, storyid int64) {

	sessid, ok := sessionCookie(w, req)
	if ok {
		readCh <- userStory{sessionid: sessid, storyid: storyid}
	}
}

// Mark a story as read (within the readstories go routine)
func markRead(sessionid string, storyid int64) {
	session, ok := sessionAsync(sessionid)
	if !ok {
		return
	}
	defer session.release()

	if !session.haveRead[storyid] {
		session.classifyStory(storyid, Interesting)
		session.haveRead[storyid] = true
		session.haveClassified = 0
		if session.haveIgnored[storyid] {
			delete(session.haveIgnored, storyid)
		}
		db.MarkRead(sessionid, storyid)
	}
}

// Indicate that a user has browsed up to the given storyid 
func MarkBrowsed(w http.ResponseWriter, req *http.Request, storyid int64) {

	session, ok := getSession(w, req)
	if !ok {
		return
	}

	defer session.release()

	markBrowsed(session, storyid)
}

// Indicate that a user has browsed up to the given storyid 
func markBrowsed(session *Session, storyid int64) {

	// We are about to access stories
	stories.mutex.RLock()
	defer stories.mutex.RUnlock()

	if storyid < stories.start || storyid < session.haveBrowsed {
		config.Debug("Not marking browsed ", storyid, ", ",
			stories.start, ", ", session.haveBrowsed)
		session.filtered = session.filtered[0:0]
		session.unfiltered = session.unfiltered[0:0]
		return
	}

	if storyid > stories.end {
		log.Println("User has browsed past the last story", storyid, ">", stories.end)
		session.filtered = session.filtered[0:0]
		session.unfiltered = session.unfiltered[0:0]
		return
	}

	// Loop through the filtered and unfiltered stories and mark 
	// the unread ones as uninteresting
	for _, s := range session.filtered {
		i := s.Id
		if i < session.haveBrowsed {
			config.Debug("Not uninteresting: ", i)
			continue
		}
		if !session.haveRead[i] && !session.haveIgnored[i] {
			session.classifyStory(i, Uninteresting)
			session.haveIgnored[i] = true
		}
	}

	for _, s := range session.unfiltered {
		i := s.Id
		if i < session.haveBrowsed {
			config.Debug("Not uninteresting: ", i)
			continue
		}
		if !session.haveRead[i] && !session.haveIgnored[i] {
			session.classifyStory(i, Uninteresting)
		}
	}

	// Clear the filtered and unfiltered stories
	session.filtered = session.filtered[0:0]
	session.unfiltered = session.unfiltered[0:0]

	// Keep track of how far the user has browsed
	session.haveBrowsed = storyid - 1
}

// Create a session
func create(s *Session) {
	_ = sessions.Create(s)
}

// Create a story index
func NewStoryIndex() *StoryIndex {
	ret := &StoryIndex{Filtered: make([]*story.Story, 0, storiesPerPage),
		Unfiltered: make([]*story.Story, 0, storiesPerPage)}
	return ret
}

// Get stories that have been read
func HaveReadStories(w http.ResponseWriter, req *http.Request) *StoryIndex {

	ret := NewStoryIndex()

	session, ok := getSession(w, req)
	if !ok {
		return ret
	}

	defer session.release()

	// Get the read stories from the db
	read := db.AllRead(session.id)

	for _, s := range read {
		ret.Unfiltered = append(ret.Unfiltered, s)
	}

	return ret
}

// Get a story
func GetStory(storyid int64) (*story.Story, bool) {
	// Access the stories fifo
	stories.mutex.RLock()
	defer stories.mutex.RUnlock()

	story, ok := stories.get(storyid)
	if ok {
		return story, true
	}

	// If the story is not in the fifo - try the db
	story = db.GetStory(storyid)
	if story == nil {
		log.Println("Could not find story id", storyid)
		return nil, false
	}

	return story, true
}

// Get the profile for a user
func Profile(w http.ResponseWriter, r *http.Request) *UserProfile {

	session, session_ok := getSession(w, r)
	if !session_ok {
		return &UserProfile{Interesting: make([]WordCount, 0), Uninteresting: make([]WordCount, 0)}
	}

	defer session.release()
	ret := new(UserProfile)

	// Get the interesting class
	ret.Interesting = mapToWordCount(session.classifier.WordsInClass(Interesting), 1)
	ret.Uninteresting = mapToWordCount(session.classifier.WordsInClass(Uninteresting), 1)

	ret.Interesting.Sort()
	ret.Uninteresting.Sort()

	return ret
}

// Convert a map of wordcounts into a slice of WordCount
func mapToWordCount(m map[string]int, min int) []WordCount {
	ret := make([]WordCount, 0, 32)
	for word, count := range m {
		if count >= min {
			wc := WordCount{Word: word, Count: count}
			ret = append(ret, wc)
		}
	}

	return ret
}

// Get interesting stories starting at the given story
func interesting(session *Session, start int64) []*story.Story {

	// We are about to access stories
	stories.mutex.RLock()
	defer stories.mutex.RUnlock()

	ret := make([]*story.Story, 0, interestingPerPage)

	if start > stories.end {
		return ret
	}

	for i := start; i < stories.end; i++ {
		if session.haveRead[i] || session.haveIgnored[i] {
			continue
		}
		story, ok := stories.get(i)
		if !ok {
			break
		}

		class := session.classifier.Classify(story.Wordlist)
		if class == Interesting {
			config.Debug("Interesting:", story.Rss.Title)
			ret = append(ret, story)
			if len(ret) == interestingPerPage {
				break
			}
		}
	}

	return ret
}

// Return n stories starting at the given story 
func unfiltered(s *Session, haveSession bool, start int64, n int, ignore map[int64]bool) []*story.Story {

	ret := make([]*story.Story, 0, n)

	if start > stories.end {
		return ret
	}

	for i := start; len(ret) < n; i++ {
		if ignore != nil && ignore[i] {
			continue
		}
		if haveSession && (s.haveRead[i] || s.haveIgnored[i]) {
			continue
		}
		story, ok := stories.get(i)
		if !ok {
			break
		}
		ret = append(ret, story)
	}

	return ret
}

// Get the interesting stories starting at the given story
func FilteredStories(w http.ResponseWriter, req *http.Request, storyid int64) *StoryIndex {

	// Get the users session if possible
	session, session_ok := getSession(w, req)

	if session_ok {
		defer session.release()
	}

	// We are about to access stories
	stories.mutex.RLock()
	defer stories.mutex.RUnlock()

	// Try and start the index at the latest browsed story if no story is specified
	start := storyid
	if session_ok && storyid == 0 {
		if session.haveBrowsed > 0 {
			start = session.haveBrowsed + 1
		} else {
			start = stories.end - DefaultStart
		}
		config.Debug("Building story index starting from", start)
	}
	start = max(start, stories.start)

	ret := NewStoryIndex()

	// Get stories 
	if !session_ok {
		ret.Unfiltered = unfiltered(nil, false, start, storiesPerPage, nil)
	} else if len(session.filtered) > 0 || len(session.unfiltered) > 0 {
		ret.Filtered = session.filtered
		ret.Unfiltered = session.unfiltered
	} else {
		ret.Filtered = interesting(session, start)
		todo := storiesPerPage - len(ret.Filtered)
		ret.Unfiltered = unfiltered(session, true, start, todo, storyIdMap(ret.Filtered))
		session.filtered = ret.Filtered
		session.unfiltered = ret.Unfiltered
	}

	previousNext(ret, start)
	return ret
}

// Get all stories starting at the given story
func UnfilteredStories(w http.ResponseWriter, req *http.Request, storyid int64) *StoryIndex {

	// Get the users session if possible
	session, session_ok := getSession(w, req)

	if session_ok {
		defer session.release()
	}

	// We are about to access stories
	stories.mutex.RLock()
	defer stories.mutex.RUnlock()

	// Try and start the index at the latest browsed story if no story is specified
	start := storyid
	if session_ok && storyid == 0 {
		start = session.haveBrowsed + 1
		log.Println("Building story index starting from", start)
	}
	start = max(start, stories.start)

	ret := NewStoryIndex()

	// Get stories 
	ret.Unfiltered = unfiltered(nil, false, start, storiesPerPage, nil)
	session.unfiltered = ret.Unfiltered

	previousNext(ret, start)
	return ret
}

// Calculate previous and next links and add them to the given story index
func previousNext(i *StoryIndex, start int64) {

	// We are about to access stories
	stories.mutex.RLock()
	defer stories.mutex.RUnlock()

	// Calculate previous link
	if start-storiesPerPage >= stories.start {
		i.Previous = start - storiesPerPage
		i.HavePrevious = true
	} else if start > stories.start {
		i.Previous = stories.start
		i.HavePrevious = true
	}

	// Calculate the next link
	if len(i.Unfiltered) == 0 {
		// This page is all interesting stories, the next link points
		// to the same start point
		i.Next = start
		i.HaveNext = true
	} else {
		next := i.Unfiltered[len(i.Unfiltered)-1].Id + 1
		if next < stories.end {
			i.Next = next
			i.HaveNext = true
		}
	}
}

// Fufil session requests in the background
func backgroundRequests() {
	for {
		select {
		case s := <-storyCh:
			// Add stories to the FIFO
			addStories(s)
		case mr := <-readCh:
			// Mark a story as read
			markRead(mr.sessionid, mr.storyid)
		}
	}
}

// Add a slice of stories to the stories FIFO
func addStories(s []*story.Story) {
	stories.mutex.Lock()
	defer stories.mutex.Unlock()

	for _, story := range s {
		stories.add(story)
	}

	config.Debug("Added ", len(s), "new stories")
	config.Debug("start =", stories.start, ", end =", stories.end)
}

// Asynchronously get the sesson with the given id
func sessionAsync(sessionid string) (*Session, bool) {
	s, ok := sessions.GetAsync(sessionid)
	if !ok {
		return nil, false
	}

	ret, ok := s.(*Session)
	if !ok {
		return nil, false
	}

	ret.mutex.Lock()
	return ret, ok
}

// Synchronously get the sesson with the given id
func sessionSync(sessionid string) (*Session, bool) {
	s, ok := sessions.Get(sessionid)
	if !ok {
		return nil, false
	}

	ret, ok := s.(*Session)
	if !ok {
		return nil, false
	}

	ret.mutex.Lock()
	return ret, ok
}

// Mark a story as Interesting/Uninteresting
func (s *Session) classifyStory(story int64, class int) {

	// We are about to access stories
	stories.mutex.RLock()
	defer stories.mutex.RUnlock()

	sty, ok := stories.get(story)
	if !ok {
		return
	}

	s.classifier.Train(sty.Wordlist, class)
}

// Unlock a session so that it can be accessed by other goroutines
func (s *Session) release() {
	s.mutex.Unlock()
}

// Read a session from the db
func readSession(key string) (cache.Entry, bool) {
	dbs, ok := db.GetSession(key)
	if !ok {
		log.Println("Failed to read session ", key, " from db")
		return nil, false
	}

	// Convert the session from db format
	// Deserialize the classsifier
	classifier, err := nbc.Deserialise(dbs.Classifier)
	if err != nil {
		return nil, false
	}

	// Get the read stories
	read := getReadMap(key)

	// Deserialise ignored stories
	ignored, err := deserialiseStoryMap(dbs.HaveIgnored)
	if err != nil {
		return nil, false
	}

	config.Debug("Deserialised classifier: ", classifier)

	// Return the deserialized session
	ret := &Session{
		id:             dbs.Id,
		classifier:     classifier,
		haveRead:       read,
		haveIgnored:    ignored,
		haveClassified: dbs.HaveClassified,
		haveBrowsed:    dbs.HaveBrowsed}

	ret.filtered = make([]*story.Story, 0, storiesPerPage)
	ret.unfiltered = make([]*story.Story, 0, storiesPerPage)
	return ret, true
}

// Save a session to the db
func saveSession(c cache.Entry) {
	// Convert from the cache format to a session
	session, ok := c.(*Session)
	if !ok {
		log.Fatal("Cannot convert cache.Entry to *Session")
	}

	config.Debug("Saving session ", session.id)
	config.Debug("Writing classifier:")
	config.Debug(session.classifier)

	config.Debug("Writing classified index: ", session.haveClassified)
	config.Debug("Writing browsed index: ", session.haveBrowsed)

	// Serialize the session
	cbytes, err := session.classifier.Serialise()
	if err != nil {
		return
	}
	ibytes, err := serialiseStoryMap(session.haveIgnored)
	if err != nil {
		return
	}

	// Write the session
	dbs := db.Session{
		Id:             session.id,
		Classifier:     cbytes,
		HaveIgnored:    ibytes,
		HaveClassified: session.haveClassified,
		HaveBrowsed:    session.haveBrowsed}

	if session.isNew {
		db.CreateSession(&dbs)
	} else {
		db.WriteSession(&dbs)
	}
}

// Serialise map of stories
func serialiseStoryMap(m map[int64]bool) ([]byte, error) {
	// Convert to a slice
	s := make([]int64, 0, len(m))
	for k, _ := range m {
		s = append(s, k)
	}

	// Serialise the slice
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	err := enc.Encode(s)
	if err != nil {
		log.Println("Failed to encode story slice:", err)
		return nil, err
	}

	return b.Bytes(), nil
}

// Deserialise into a map of stories
func deserialiseStoryMap(b []byte) (map[int64]bool, error) {
	// Unserialise the slice 
	r := bytes.NewReader(b)
	dec := gob.NewDecoder(r)
	var s []int64
	err := dec.Decode(&s)

	if err != nil {
		log.Println("Failed to decode map slice:", err)
		return nil, err
	}

	// We are about to access the stories fifo
	stories.mutex.RLock()
	defer stories.mutex.RUnlock()

	// Convert to a map
	ret := make(map[int64]bool)
	for _, v := range s {
		// Add all relevent stories to the map
		if _, ok := stories.get(v); ok {
			ret[v] = true
		}
	}

	return ret, nil
}

// Get the relevent read stories from the DB
func getReadMap(sessionid string) map[int64]bool {

	// We are about to access the stories fifo
	stories.mutex.RLock()
	defer stories.mutex.RUnlock()

	// Get the read stories from the DB
	read := db.GetRead(sessionid, stories.start, stories.end)

	ret := make(map[int64]bool)

	// Convert to a map
	for _, s := range read {
		ret[s] = true
	}

	return ret
}

// Start the go routine that wraps the session
func Start() {
	setupCookies()
	initFifo()
	go backgroundRequests()
}

// Fill the Fifo from the db
func initFifo() {
	s := db.GetLatestStories(MaxStories)

	// Add the stories in reverse order
	for i := len(s) - 1; i >= 0; i-- {
		stories.add(s[i])
	}
}

func max(a int64, b int64) int64 {
	if a >= b {
		return a
	}
	return b
}

// Extract a map of story ids from a slice of stories
func storyIdMap(stories []*story.Story) map[int64]bool {
	ret := make(map[int64]bool)

	for _, s := range stories {
		ret[s.Id] = true
	}

	return ret
}
