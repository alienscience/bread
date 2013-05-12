package session

import (
	"bread/story"
	"testing"
)

var storyOne = &story.Story{Id: 1, Wordlist: []string{"fox", "jumped", "cat"}}
var storyTwo = &story.Story{Id: 2, Wordlist: []string{"cow", "jumped", "moon"}}
var storyThree = &story.Story{Id: 3, Wordlist: []string{"man", "shoots", "cat"}}

func TestReading(t *testing.T) {

	stories.add(storyOne)
	stories.add(storyTwo)
	stories.add(storyThree)

	setupCookies()
	sess := newSession()
	create(sess)

	markRead(sess.id, storyOne.Id)

	if !sess.haveRead[storyOne.Id] {
		t.Error("Have not read story after markRead")
	}

	markBrowsed(sess, storyThree.Id)

	if sess.haveBrowsed != storyTwo.Id {
		t.Error("Have not browsed story that was markBrowsed")
	}

	if sess.haveRead[storyTwo.Id] {
		t.Error("Have read story that was not markRead")
	}

	if sess.classifier.Classify(storyThree.Wordlist) != Interesting {
		t.Error("Classifier not classifying when used through session")
	}

}

var story1 = &story.Story{Id: 1, Wordlist: []string{"fox", "jumped", "cat"}}
var story2 = &story.Story{Id: 2, Wordlist: []string{"cow", "jumped", "moon"}}
var story3 = &story.Story{Id: 3, Wordlist: []string{"man", "shoots", "cat"}}
var story4 = &story.Story{Id: 4, Wordlist: []string{"car", "eating", "cat"}}

func TestFifoAdd(t *testing.T) {
	fifo := newFifo(3)

	fifo.add(story1)

	if fifo.start != 1 || fifo.end != 1 {
		t.Error("fifo first: ", fifo.start, " --> ", fifo.end)
	}

	fifo.add(story2)

	if fifo.start != 1 || fifo.end != 2 {
		t.Error("fifo filling: ", fifo.start, " --> ", fifo.end)
	}

	fifo.add(story3)

	if fifo.start != 1 || fifo.end != 3 {
		t.Error("fifo fill: ", fifo.start, " --> ", fifo.end)
	}

	fifo.add(story4)

	if fifo.start != 2 || fifo.end != 4 {
		t.Error("fifo wrap around: ", fifo.start, " --> ", fifo.end)
	}
}
