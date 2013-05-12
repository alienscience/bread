package session

import (
	"bread/config"
	"bread/story"
	"log"
	"sync"
)

// A fifo type that holds stories
type fifo struct {
	index []*story.Story
	start int64 // The id of the oldest story
	end   int64 // The id of the newest story
	head  int   // Index to the oldest item in the fifo

	// Most accesses to the fifo are reads. Writes only occur when a 
	// feed is downloaded
	mutex sync.RWMutex
}

// Create a new fifo
func newFifo(capacity int) fifo {
	return fifo{index: make([]*story.Story, 0, capacity)}
}

// Add a story to the fifo
func (f *fifo) add(s *story.Story) {

	// The fifo relies on the assumption that storyids always 
	// increase
	if s.Id < f.end {
		log.Panicln("Non-increasing story id added to fifo: ",
			s.Id, ", ", f.start, ", ", f.end)
	}

	if len(f.index) == 0 {
		f.start = s.Id
		f.index = append(f.index, s)
		f.end = s.Id
	} else if len(f.index) == cap(f.index) {
		f.index[f.head] = s
		f.head = (f.head + 1) % cap(f.index)
		f.start = f.index[f.head].Id
		f.end = s.Id
	} else {
		f.index = append(f.index, s)
		f.end = s.Id
	}

	config.Debug("Added story: ", s.Id, ", ", f.start, "-", f.end)
}

// Get the given story from the fifo
func (f *fifo) get(story int64) (*story.Story, bool) {

	if len(f.index) == 0 || story < f.start || story > f.end {
		return nil, false
	} else if story == f.start {
		return f.index[f.head], true
	}

	idx := (f.head + int(story-f.start)) % len(f.index)
	return f.index[idx], true
}
