package main

import (
	"bread/config"
	"bread/db"
	"bread/index"
	"bread/pages"
	"bread/session"
	"log"
	"net/http"
	"runtime"
)

func main() {
	// Get configuration
	config.Init()

	// Initialise packages
	db.Start()
	session.Start()
	index.Start()
	pages.Start()

	// Setup HTTP server
	log.Println("Starting HTTP server")
	runtime.GOMAXPROCS(20)
	http.HandleFunc("/", pages.Home)
	http.HandleFunc("/read", pages.Read)
	http.HandleFunc("/readagain", pages.ReadAgain)
	http.HandleFunc("/comments", pages.Comments)
	http.HandleFunc("/next", pages.Next)
	http.HandleFunc("/prev", pages.Previous)
	http.HandleFunc("/static/", pages.Static)
	http.HandleFunc("/haveread", pages.HaveRead)
	http.HandleFunc("/profile", pages.Profile)

	// Start the HTTP Server
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
