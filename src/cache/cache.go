/*
 An in-memory, non-blocking, Least Recently Used, time limited, copy back cache
 with thundering herd prevention.  Keys are strings.
*/

package cache

import (
	"container/list"
	"log"
	"time"
)

// A concurrent copy back cache
type Cache struct {
	lines       map[string]*line
	get         func(string) (Entry, bool)
	getCh       chan string // A cache makes requests to get data on this channel
	put         chan Entry  // Data can be put into a cache on this channel
	notFound    chan string // Keys that are not found are put onto this channel
	cp          func(Entry)
	cpCh        chan Entry // A cache makes requests to copy modifications back on this channel
	size        int
	ttl         time.Duration
	ttlPoll     <-chan time.Time
	lruList     *list.List
	lookupSync  chan lookup
	lookupAsync chan lookup
	createCh    chan create
}

// A cache entry
type Entry interface {
	Key() string
}

// An augumented entry within the cache
type line struct {
	payload  Entry
	empty    bool          // Indicates if this is just a placeholder
	waiting  []chan result // Slice of channels waiting on the result
	lruEntry *list.Element // The corresponding entry in the LRU list
	lastUse  time.Time     // The last time this entry was used
}

// A cache lookup request
type lookup struct {
	key   string
	resCh chan result
}

// A create request
type create struct {
	entry Entry
	resCh chan bool
}

// The result of a lookup request
type result struct {
	value Entry
	ok    bool
}

// Create a new cache with functions to make get requests and to copy back data
func New(size int, ttl time.Duration, get func(string) (Entry, bool), cp func(Entry)) *Cache {
	ret := new(Cache)
	ret.lines = make(map[string]*line)
	ret.get = get
	ret.getCh = make(chan string, 8)
	ret.put = make(chan Entry, 8)
	ret.notFound = make(chan string, 8)
	ret.cpCh = make(chan Entry, 8)
	ret.cp = cp
	ret.size = size
	ret.ttl = ttl
	ret.ttlPoll = time.Tick(ttl / 5)
	ret.lruList = list.New()
	ret.lookupSync = make(chan lookup)
	ret.lookupAsync = make(chan lookup, 8)
	ret.createCh = make(chan create)

	// Start a go routine to process cache requests
	go cacheRequests(ret)

	// Start a go routine to process get requests
	go getRequests(ret)

	// Start a go routine to process cp requests
	go cpRequests(ret)

	return ret
}

// Get an entry from the cache only if immediately available and fill the 
// cache asynchronously if the key is not present
func (c *Cache) GetAsync(key string) (value interface{}, present bool) {
	res := make(chan result)
	c.lookupAsync <- lookup{key: key, resCh: res}
	ret := <-res
	return ret.value, ret.ok
}

// Get an entry from the cache and synchronously get the value if not present
func (c *Cache) Get(key string) (value interface{}, ok bool) {
	res := make(chan result)
	c.lookupSync <- lookup{key: key, resCh: res}
	ret := <-res
	return ret.value, ret.ok
}

// Create a new entry
func (c *Cache) Create(e Entry) bool {
	res := make(chan bool)
	c.createCh <- create{entry: e, resCh: res}
	ret := <-res
	return ret
}

// Handle requests made to the given cache
func cacheRequests(c *Cache) {
	for {
		select {
		case ls := <-c.lookupSync:
			c.syncLookup(ls)
		case as := <-c.lookupAsync:
			c.asyncLookup(as)
		case p := <-c.put:
			c.putEntry(p)
		case cr := <-c.createCh:
			cr.resCh <- c.createEntry(cr.entry)
		case n := <-c.notFound:
			c.keyNotFound(n)
		case _ = <-c.ttlPoll:
			c.timeout()
		}
	}
}

// Handle get requests
func getRequests(c *Cache) {
	// This is cyclical to prevent deadlock
	for {
		key := <-c.getCh
		e, ok := c.get(key)
		if ok {
			c.put <- e
		} else {
			c.notFound <- key
		}
	}
}

// Handle copy back requests
func cpRequests(c *Cache) {
	for {
		e := <-c.cpCh
		c.cp(e)
	}
}

// Lookup an entry synchronously
func (c *Cache) syncLookup(l lookup) {
	line, ok := c.lines[l.key]
	if !ok {
		// Create an empty cache line to wait for the result
		line = c.newLine(l.key)
		line.waiting = append(line.waiting, l.resCh)

		// Lookup the value
		c.getCh <- l.key
		return
	} else if line.empty {
		// Append to the waiting slice
		line.waiting = append(line.waiting, l.resCh)
		return
	} else {
		// A non-empty entry exists
		c.updateLRU(line)
		l.resCh <- result{value: line.payload, ok: true}
	}
}

// Lookup an entry asynchronously
func (c *Cache) asyncLookup(l lookup) {
	line, ok := c.lines[l.key]
	if !ok {
		// Create an empty cache line
		line = c.newLine(l.key)

		// Signal value not present
		l.resCh <- result{value: nil, ok: false}

		// Lookup the value
		c.getCh <- l.key

	} else if line.empty {
		// Signal value not present
		l.resCh <- result{value: nil, ok: false}
	} else {
		// The entry is valid
		c.updateLRU(line)
		l.resCh <- result{value: line.payload, ok: true}
	}
}

// Put a value into the cache
func (c *Cache) putEntry(e Entry) {
	line, ok := c.lines[e.Key()]
	if !ok {
		// Create a new cache line
		line = c.newLine(e.Key())
		line.payload = e
		line.empty = false
		return
	}

	// Update the entry that is already present
	line.payload = e
	line.empty = false

	// Inform any channels that are waiting on the result
	for _, ch := range line.waiting {
		ch <- result{value: e, ok: true}
	}
}

// Create a new entry in the cache
func (c *Cache) createEntry(e Entry) bool {
	line, ok := c.lines[e.Key()]
	if ok {
		return false
	}

	// Create a new cache line
	line = c.newLine(e.Key())
	line.payload = e
	line.empty = false
	return true
}

// Indicate that an entry requested by the cache was not found
func (c *Cache) keyNotFound(k string) {
	line, ok := c.lines[k]
	if !ok {
		// Nobody is waiting on the result
		return
	}

	// Inform any channels that are waiting 
	if line.empty {
		for _, ch := range line.waiting {
			ch <- result{value: nil, ok: false}
		}
	}

	// Remove the cache line 
	c.removeLine(k)
	return
}

// Remove entries from the cache that have exceeded the ttl
func (c *Cache) timeout() {
	timeout := time.Now().Add(-c.ttl)

	// Loop through the LRU list
	for lru := c.lruList.Back(); lru != nil; lru = c.lruList.Back() {
		key, ok := lru.Value.(string)
		if !ok {
			log.Fatal("Got a non-string key out of lruList")
		}

		// Check when the entry was last used
		line, ok := c.lines[key]
		if ok {
			if line.lastUse.Before(timeout) {
				c.removeLine(key)
			} else {
				break
			}
		} else {
			c.lruList.Remove(lru)
		}
	}

}

// Create a new cache line with the given key
func (c *Cache) newLine(k string) *line {

	line := &line{empty: true, lastUse: time.Now()}
	line.waiting = make([]chan result, 0, 4)

	// Is the cache full?
	if len(c.lines) >= c.size {
		c.removeLRU()
	}

	// Add an entry in the LRU list
	line.lruEntry = c.lruList.PushFront(k)
	c.lines[k] = line
	return line
}

// Update the LRU entry for the given cache line
func (c *Cache) updateLRU(l *line) {
	l.lastUse = time.Now()
	c.lruList.MoveToFront(l.lruEntry)
}

// Remove a line from the cache
func (c *Cache) removeLine(key string) {
	line, ok := c.lines[key]
	if ok {
		lru := line.lruEntry
		c.lruList.Remove(lru)
		delete(c.lines, key)
	}

	// Copy back the entry in case it has changed
	if !line.empty {
		c.cpCh <- line.payload
	}
}

// Remove the least recently used line from the cache
func (c *Cache) removeLRU() {
	oldest := c.lruList.Back()
	key, ok := oldest.Value.(string)
	if !ok {
		log.Fatal("Got a non-string key out of lruList")
	}

	c.removeLine(key)
}
