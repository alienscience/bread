package session

// Maintains a sorted list of high scoring stories

import (
	"container/list"
	"log"
)

type score struct {
	storyid int64
	score   float64
}

// Adds the given story id and bayes factor to the given list if it
// is higher than at least one of the ones already in the list
func addIfHigh(scores *list.List, length int, storyid int64, k float64) {

	s := score{storyid: storyid, score: k}

	// Add the score if the list is empty
	last := scores.Back()
	if last == nil {
		scores.PushBack(s)
		return
	}

	if scores.Len() < length {
		insertScore(scores, length, s)
		return
	}

	// Add the score to the list if it is high enough
	lowest, ok := last.Value.(score)
	if !ok {
		log.Fatal("Could not extract score from sorted list")
	}

	if k < lowest.score {
		return
	}

	// If this point is reached, we insert the score
	insertScore(scores, length, s)
}

// Insert the given score into the given list
func insertScore(scores *list.List, length int, s score) {
	// Loop through the scores
	for e := scores.Front(); e != nil; e = e.Next() {
		if e == scores.Back() {
			scores.InsertAfter(s, e)
			break
		}
		v, ok := e.Value.(score)
		if !ok {
			log.Fatal("Could not extract score from sorted list")
		}
		if s.score > v.score {
			scores.InsertBefore(s, e)
			break
		}
	}

	// Remove the last entry if the list is too long
	if scores.Len() > length {
		scores.Remove(scores.Back())
	}
}

// Get the story ids from the given list of scores
func idsFromScores(scores *list.List) []int64 {
	ret := make([]int64, 0, scores.Len())

	// Loop through the scores
	for e := scores.Front(); e != nil; e = e.Next() {
		v, ok := e.Value.(score)
		if !ok {
			log.Fatal("Could not extract score from sorted list")
		}
		ret = append(ret, v.storyid)
	}

	return ret
}
