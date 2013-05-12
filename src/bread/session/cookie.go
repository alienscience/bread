package session

import (
	"bread/config"
	"bread/nbc"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"log"
	"math"
	"math/big"
	"net/http"
)

var maxSessionId *big.Int

// Generate a unique id as a base64 encoded string
func generateId() string {

	// Generate a random 128 bit Id 
	z, err := rand.Int(rand.Reader, maxSessionId)
	if err != nil {
		log.Fatal("Cannot generate session id", err)
	}

	// Take the hash of the random id
	sha1 := sha1.New()
	sha1.Write(z.Bytes())

	return base64.StdEncoding.EncodeToString(sha1.Sum([]byte{}))
}

// Create a new session structure
func newSession() *Session {
	s := new(Session)
	s.id = generateId()
	s.isNew = true
	s.classifier = nbc.New([]float64{InterestingPrior, UninterestingPrior})
	s.haveRead = make(map[int64]bool)
	s.haveIgnored = make(map[int64]bool)
	s.haveBrowsed = 0
	return s
}

// Get the session id from the session cookie if available
func sessionCookie(w http.ResponseWriter, req *http.Request) (string, bool) {
	cookie, err := req.Cookie("id")
	if err != nil {
		return "", false
	}

	return cookie.Value, true
}

// Get the session from the cookie in the request
// Creates a new session and cookie if none can be found
func getSession(w http.ResponseWriter, req *http.Request) (*Session, bool) {

	cookie, err := req.Cookie("id")
	if err == nil {
		s, ok := sessionSync(cookie.Value)
		if ok {
			return s, ok
		}

		log.Println("Invalid session cookie presented: ", cookie.Value)
	}

	// Create a session
	s := newSession()
	config.Debug("Creating new session ", s.id)
	sessions.Create(s)

	// Write the session to the HTTP response
	newcookie := http.Cookie{
		Name:     "id",
		Value:    s.id,
		Path:     "/",
		MaxAge:   math.MaxInt32,
		HttpOnly: true}
	w.Header().Add("Set-Cookie", newcookie.String())

	s.mutex.Lock()
	return s, true
}

func setupCookies() {
	maxSessionId = big.NewInt(0)
	maxSessionId.Exp(big.NewInt(2), big.NewInt(128), nil)
}
