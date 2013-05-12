package db

import (
	"bread/rss"
	"bread/story"
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
)

// Database queries and statements
const (
	seenStory = iota
	addStory
	getLatestStories
	createSession
	updateSession
	getSession
	markRead
	getRead
	allRead
	getStory
	numStatements
)

var statementDefs = []statement{
	{seenStory, "seenStory",
		"select ROWID from story where providerid = ?;"},
	{addStory, "addStory",
		"insert into story (providerid, title, summary, link, comments)" +
			" values (?,?,?,?,?);"},
	{getLatestStories, "getLatestStories",
		"select ROWID, providerid, title, summary, link, comments" +
			" from story order by ROWID desc limit ?"},
	{createSession, "createSession",
		"insert into session (id, classifier, ignored, browsed, classified)" +
			" values (?, ?, ?, ?, ?);"},
	{updateSession, "updateSession",
		"update session set classifier = ?, ignored = ?, browsed = ?, classified = ?" +
			" where id = ?"},
	{getSession, "getSession",
		"select id, classifier, ignored, browsed, classified" +
			" from session where id = ?"},
	{markRead, "markRead",
		"insert into read (sessionid, storyid)" +
			" values (?, ?);"},
	{getRead, "getRead",
		"select storyid from read" +
			" where sessionid = ? and storyid >= ? and storyid <= ?"},
	{allRead, "allRead",
		"select story.ROWID, providerid, title, summary, link, comments" +
			" from story, read" +
			" where story.ROWID = read.storyid and sessionid = ?"},
	{getStory, "getStory",
		"select story.ROWID, providerid, title, summary, link, comments" +
			" from story where story.ROWID = ?"}}

type statement struct {
	id   int
	name string
	sql  string
}

// A user session in a form serializable to the DB
type Session struct {
	Id             string
	Classifier     []byte
	HaveIgnored    []byte
	HaveClassified int64
	HaveBrowsed    int64
}

// A request that reads something from the database
type readReq struct {
	stmt     int                         // The id of the statement to run
	replyCh  chan interface{}            // A channel to send the results on
	readRows func(*sql.Stmt) interface{} // Build a datastructure from the query result
}

// A request to write something to the database without returning data
type writeReq struct {
	stmt    int             // The id of the statement to run
	replyCh chan bool       // A reply channel indicating success
	write   func(*sql.Stmt) // Write to the db using the given statement
}

// A type that indicates if a story has been seen before
type seen struct {
	id       int64
	haveSeen bool
}

// A type that holds a session and a success bool
type sessionOK struct {
	session *Session
	ok      bool
}

// Channels used to send requests to the db go routine
var readCh = make(chan *readReq)
var writeCh = make(chan *writeReq, 8)

// Fufil db requests from the given channels
func dbRequests() {

	// Open the DB
	db, err := sql.Open("sqlite3", "./db/bread.db")
	if err != nil {
		log.Fatal("Cannot open db/bread.db: ", err)
	}
	defer db.Close()

	// Setup database statements
	statements := createStatements(db)
	defer closeStatements(statements)

	// Loop reading requests and executing DB statements
	for {
		select {
		case rr := <-readCh:
			rr.replyCh <- rr.readRows(statements[rr.stmt])
		case wr := <-writeCh:
			wr.write(statements[wr.stmt])
		}
	}
}

// Create statement handles
func createStatements(db *sql.DB) []*sql.Stmt {
	ret := make([]*sql.Stmt, numStatements)

	for _, def := range statementDefs {
		sth, err := db.Prepare(def.sql)
		if err != nil {
			log.Fatal("Cannot prepare ", def.name, " statement: ", err)
		}
		ret[def.id] = sth
	}

	return ret
}

//  Close statement handles
func closeStatements(statements []*sql.Stmt) {
	for _, sth := range statements {
		sth.Close()
	}
}

// Indicate if the story with the given provider id has already been by this application
func SeenStory(providerid string) (int64, bool) {

	rr := new(readReq)
	rr.stmt = seenStory
	rr.replyCh = make(chan interface{})

	rr.readRows = func(stmt *sql.Stmt) interface{} {

		// Run the query
		rows, err := stmt.Query(providerid)

		if err != nil {
			log.Fatal("Cannot execute seenStory stmt: ", err)
		}
		defer rows.Close()

		var id int64
		found := false
		for rows.Next() {
			rows.Scan(&id)
			found = true
		}

		if found {
			return seen{id, true}
		}

		return seen{0, false}
	}

	readCh <- rr

	// Wait for a reply
	res := <-rr.replyCh
	ret, ok := res.(seen)
	if !ok {
		log.Fatal("Returned seen struct failed type assertion")
	}

	return ret.id, ret.haveSeen
}

// Add a story to the database and return its storyid
func AddStory(s *rss.Story) int64 {

	rr := new(readReq)
	rr.stmt = addStory
	rr.replyCh = make(chan interface{})

	rr.readRows = func(stmt *sql.Stmt) interface{} {

		// Run the query
		result, err := stmt.Exec(s.Id, s.Title, s.Summary, s.Link, s.Comments)
		if err != nil {
			log.Fatal("Cannot execute addStory stmt: ", err)
		}

		id, err := result.LastInsertId()
		if err != nil {
			log.Fatal("Error retrieving insert id:", err)
		}

		return id
	}

	readCh <- rr

	// Wait for a reply
	res := <-rr.replyCh
	ret, ok := res.(int64)
	if !ok {
		log.Fatal("Returned int64 failed type assertion")
	}

	return ret

}

// Read the latest stories
func GetLatestStories(numStories int) []*story.Story {

	rr := new(readReq)
	rr.stmt = getLatestStories
	rr.replyCh = make(chan interface{})

	rr.readRows = func(stmt *sql.Stmt) interface{} {

		// Run the query
		rows, err := stmt.Query(numStories)

		if err != nil {
			log.Fatal("Cannot execute getLatestStories stmt: ", err)
		}
		defer rows.Close()

		var id int64
		r := rss.Story{}
		stories := make([]*story.Story, 0, numStories)

		for rows.Next() {
			rows.Scan(&id, &r.Id, &r.Title, &r.Summary, &r.Link, &r.Comments)
			story := story.FromRSS(id, &r)
			stories = append(stories, story)
		}

		return stories
	}

	readCh <- rr

	// Wait for a reply
	res := <-rr.replyCh
	ret, ok := res.([]*story.Story)
	if !ok {
		log.Fatal("Returned []*story.Story failed type assertion")
	}

	return ret
}

// Get a session
func GetSession(sessionid string) (*Session, bool) {

	rr := new(readReq)
	rr.stmt = getSession
	rr.replyCh = make(chan interface{})

	rr.readRows = func(stmt *sql.Stmt) interface{} {

		// Run the query
		// TODO: swap to Query/RawBytes to save memcpy
		rows, err := stmt.Query(sessionid)

		if err != nil {
			log.Fatal("Failed to execute getSession stmt: ", err)
		}

		defer rows.Close()

		var id string
		var classifier []byte
		var ignored []byte
		var browsed int64
		var classified int64
		have_row := false
		for rows.Next() {
			rows.Scan(&id, &classifier, &ignored, &browsed, &classified)
			have_row = true
		}

		if have_row {
			return sessionOK{
				session: &Session{Id: id,
					Classifier:     classifier,
					HaveIgnored:    ignored,
					HaveBrowsed:    browsed,
					HaveClassified: classified},
				ok: true}
		}

		return sessionOK{session: nil, ok: false}
	}

	readCh <- rr

	// Wait for a reply
	res := <-rr.replyCh
	ret, ok := res.(sessionOK)
	if !ok {
		log.Fatal("Returned *Session failed type assertion")
	}

	return ret.session, ret.ok
}

// Create a session
func CreateSession(session *Session) {

	wr := new(writeReq)
	wr.stmt = createSession

	wr.write = func(stmt *sql.Stmt) {

		// Execute the statement
		_, err := stmt.Exec(
			session.Id,
			session.Classifier,
			session.HaveIgnored,
			session.HaveBrowsed,
			session.HaveClassified)
		if err != nil {
			log.Fatal("Cannot execute createSession stmt: ", err)
		}
	}

	writeCh <- wr
}

// Write to an existing session
func WriteSession(session *Session) {

	wr := new(writeReq)
	wr.stmt = updateSession

	wr.write = func(stmt *sql.Stmt) {

		// Execute the statement
		_, err := stmt.Exec(
			session.Classifier,
			session.HaveIgnored,
			session.HaveBrowsed,
			session.HaveClassified,
			session.Id)
		if err != nil {
			log.Fatal("Cannot execute updateSession stmt: ", err)
		}
	}

	writeCh <- wr
}

// Mark a story as read
func MarkRead(sessionid string, storyid int64) {

	wr := new(writeReq)
	wr.stmt = markRead

	wr.write = func(stmt *sql.Stmt) {

		// Execute the statment
		_, err := stmt.Exec(sessionid, storyid)
		if err != nil {
			log.Println("Cannot execute markRead(", sessionid, ",", storyid, ") stmt: ", err)
		}
	}

	writeCh <- wr
}

// Get the read stories for a session
func GetRead(sessionid string, minid, maxid int64) []int64 {

	rr := new(readReq)
	rr.stmt = getRead
	rr.replyCh = make(chan interface{})

	rr.readRows = func(stmt *sql.Stmt) interface{} {

		// Run the query
		rows, err := stmt.Query(sessionid, minid, maxid)

		if err != nil {
			log.Fatal("Cannot execute getRead stmt: ", err)
		}
		defer rows.Close()

		var id int64
		read := make([]int64, 0, 8)

		for rows.Next() {
			rows.Scan(&id)
			read = append(read, id)
		}

		return read
	}

	readCh <- rr

	// Wait for a reply
	res := <-rr.replyCh
	ret, ok := res.([]int64)
	if !ok {
		log.Fatal("Returned []int64 failed type assertion")
	}

	return ret
}

// Get all the stories read by a session
func AllRead(sessionid string) []*story.Story {

	rr := new(readReq)
	rr.stmt = allRead
	rr.replyCh = make(chan interface{})

	rr.readRows = func(stmt *sql.Stmt) interface{} {

		// Run the query
		rows, err := stmt.Query(sessionid)

		if err != nil {
			log.Fatal("Cannot execute allRead stmt: ", err)
		}
		defer rows.Close()

		var id int64
		r := rss.Story{}
		stories := make([]*story.Story, 0, 8)

		for rows.Next() {
			rows.Scan(&id, &r.Id, &r.Title, &r.Summary, &r.Link, &r.Comments)
			story := story.FromRSS(id, &r)
			stories = append(stories, story)
		}

		return stories
	}

	readCh <- rr

	// Wait for a reply
	res := <-rr.replyCh
	ret, ok := res.([]*story.Story)
	if !ok {
		log.Fatal("Returned []*story.Story failed type assertion")
	}

	return ret
}

// Get the story with the given id
func GetStory(storyid int64) *story.Story {

	rr := new(readReq)
	rr.stmt = getStory
	rr.replyCh = make(chan interface{})

	rr.readRows = func(stmt *sql.Stmt) interface{} {

		// Run the query
		rows, err := stmt.Query(storyid)

		if err != nil {
			log.Fatal("Cannot execute getStory stmt: ", err)
		}
		defer rows.Close()

		var id int64
		var s *story.Story

		r := rss.Story{}
		for rows.Next() {
			rows.Scan(&id, &r.Id, &r.Title, &r.Summary, &r.Link, &r.Comments)
			s = story.FromRSS(id, &r)
		}

		return s
	}

	readCh <- rr

	// Wait for a reply
	res := <-rr.replyCh
	ret, ok := res.(*story.Story)
	if !ok {
		log.Fatal("Returned *story.Story failed type assertion")
	}

	return ret
}

// Start the DB go routine
func Start() {
	go dbRequests()
}
