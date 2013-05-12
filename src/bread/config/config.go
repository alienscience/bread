package config

import (
	"flag"
	"log"
	"os"
)

// Global configuration
var Standalone bool // Indicates that the server is not connected to the internet
var Devmode bool    // Indicates that the server is in development mode

// Logger for debug information
var dbg = log.New(os.Stdout, "Debug: ", 0)

// Output debug information
func Debug(v ...interface{}) {
	if Devmode {
		dbg.Println(v...)
	}
}

// Read configuration from the command line
func Init() {
	flag.BoolVar(&Standalone, "standalone", false, "Run the server without an internet connection.")
	flag.BoolVar(&Devmode, "dev", false, "Run the server in development mode.")
	flag.Parse()
}
