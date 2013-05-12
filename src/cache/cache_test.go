package cache

import (
	"testing"
	"time"
)

// An entry type for testing
type testEntry struct {
	key   string
	value string
}

func (t *testEntry) Key() string {
	return t.key
}

// Test synchronous get
func TestSync(t *testing.T) {
	c := New(2, 20*time.Second,
		func(key string) (Entry, bool) { return &testEntry{key: key, value: "pass"}, true },
		func(e Entry) {})

	// Do the get request 
	res, ok := c.Get("testo")

	if !ok {
		t.Error("Failure with synchronous get")
	}

	entry, ok := res.(*testEntry)
	if !ok {
		t.Error("Cache returned incorrect entry type")
	} else if entry.value != "pass" {
		t.Error("Incorrect value from synchronous get", entry)
	}
}

// Test async get
func TestAsync(t *testing.T) {
	c := New(2, 20*time.Second,
		func(key string) (Entry, bool) { return &testEntry{key: key, value: "pass"}, true },
		func(e Entry) {})

	// Do the get request 
	_, ok := c.GetAsync("testo")

	if ok {
		t.Error("Asynchronous get returned an empty valid value")
	}

	// Do a synchronous get request 
	res, ok := c.Get("testo")

	entry, ok := res.(*testEntry)
	if !ok {
		t.Error("Cache returned incorrect entry type")
	} else if entry.value != "pass" {
		t.Error("Incorrect value from synchronous get", entry)
	}

	// Do an asynchronous get request
	res, ok = c.GetAsync("testo")

	entry, ok = res.(*testEntry)
	if !ok {
		t.Error("Cache returned incorrect entry type")
	} else if entry.value != "pass" {
		t.Error("Incorrect value from asynchronous get", entry)
	}
}

// Test cache entry timeouts
func TestTTL(t *testing.T) {

	cpCh := make(chan Entry, 1)

	c := New(2, 2*time.Second,
		func(key string) (Entry, bool) { return &testEntry{key: key, value: "pass"}, true },
		func(e Entry) { cpCh <- e })

	// Do a get request 
	_, ok := c.Get("testo")

	if !ok {
		t.Error("Failure with synchronous get")
	}

	// Sleep ttl
	time.Sleep(2 * time.Second)

	// Do an async get request
	_, ok = c.GetAsync("testo")

	if ok {
		t.Error("Entry did not time out")
	}

	// Check the cache entry was copied back when it timed out
	cpEntry := <-cpCh

	entry, ok := cpEntry.(*testEntry)
	if !ok {
		t.Error("Copy back sent wrong type of entry")
	} else if entry.value != "pass" {
		t.Error("Copy back sent wrong entry")
	}
}

// Test not found
func TestNotFound(t *testing.T) {

	c := New(2, 2*time.Second,
		func(key string) (Entry, bool) { return nil, false },
		func(e Entry) {})

	// Do the get request 
	_, ok := c.Get("testo")

	if ok {
		t.Error("Not found appears to be found")
	}
}

// Test creating an entry
func TestCreate(t *testing.T) {
	c := New(2, 2*time.Second,
		func(key string) (Entry, bool) { return &testEntry{key: key, value: "pass"}, true },
		func(e Entry) {})

	ok := c.Create(&testEntry{key: "testo", value: "pass"})
	if !ok {
		t.Error("Failed to create cache entry")
	}

	ok = c.Create(&testEntry{key: "testo", value: "clash"})
	if ok {
		t.Error("Created a clashing cache entry")
	}

	// Make sure the entry exists
	res, ok := c.Get("testo")

	if !ok {
		t.Error("Failed to get a newly created cache entry")
	} else {
		entry, ok := res.(*testEntry)
		if !ok {
			t.Error("Newly created cache entry is the wrong type")
		} else if entry.value != "pass" {
			t.Error("Newly created cache entry has the wrong value")
		}
	}
}
