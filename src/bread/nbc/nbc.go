package nbc

// A naive Bayes classifier
//  Uses a boolean multinomial model
//  Fields are exported so that they can be serialized

import (
	"bread/config"
	"bytes"
	"encoding/gob"
	"log"
	"math"
	"regexp"
	"strings"
)

// A class that an item is classified into
type Class struct {
	Count      int            // The number of times this class has been assigned to
	Prior      float64        // Log2 of the prior of this class
	Vocabulary map[string]int // Word frequencies in the class
}

// A classifier
type Classifier struct {
	Classes []Class // The classes making up the classification
	Total   int     // The total number of times the classifier has been trained
	Words   int     // The total number of words seen during training
}

// Punctuation that will be removed
// TODO: word style quotes
var punctuation = "()?'[]`,:-!’‘" + `"`

var isNumber = regexp.MustCompile(`^[0-9$,.]+$`)

// Create a classifier
func New(priors []float64) *Classifier {
	ret := Classifier{}

	numclasses := len(priors)
	if numclasses < 2 {
		log.Fatal("Classifier needs at least 2 classes")
	}

	ret.Classes = make([]Class, numclasses, numclasses)

	for i := 0; i < numclasses; i++ {
		ret.Classes[i] = Class{0, math.Log2(priors[i]), make(map[string]int)}
	}

	return &ret
}

// Separate a string into a slice of words
func Wordlist(text string) []string {

	words := strings.Split(text, " ")

	// Make the words lowercase 
	// Throw away small words that are not acronyms
	// Throw away numbers
	// Remove fullstops 
	// Remove punctuation
	ret := make([]string, 0)
	for _, w := range words {
		lw := len(w)
		if lw < 2 {
			continue
		}
		if lw < 5 {
			if strings.ToUpper(w) != w {
				continue
			}
		}
		if isNumber.MatchString(w) {
			continue
		}
		w, ok := removePunctuation(w)

		if ok  {
			lc := strings.ToLower(w)
			ret = append(ret, lc)
		}
	}

	return ret
}

// Remove punctuation from a word returns the word and a boolean indicating
// if the word is still valid
func removePunctuation(word string) (string, bool) {

	ret := make([]rune, 0, len(word))

	// Loop through the runes in the string
	for _, r := range []rune(word) {
		if strings.ContainsRune(punctuation, r) {
			continue
		}
		ret = append(ret, r)
	}

	// Ignore empty words
	if len(ret) == 0 {
		return "", false
	}

	// Remove trailing full stops
	if ret[len(ret)-1] == '.' {
		ret = ret[0 : len(ret)-1]
	}

	return string(ret), true
}

// Update the given vocabulary with the given wordlist
func updateVocabulary(vocab map[string]int, words []string) {
	for _, w := range words {
		count := vocab[w]
		count += 1
		vocab[w] = count
	}
}

// Train the classifier - the given wordlist belongs to the given class
func (c *Classifier) Train(words []string, class int) {
	// Get the class the words belong to
	tc := &c.Classes[class]

	// Update the count and vocabularly
	tc.Count += 1
	updateVocabulary(tc.Vocabulary, words)

	// Update the total counts
	c.Total += 1
	c.Words += len(words)
}

// Train the classifier - the given text belongs to the given class
func (c *Classifier) TrainText(text string, class int) {
	c.Train(Wordlist(text), class)
}

// Calculate the weight of the given wordlist for the given class
func (c *Classifier) Weight(class int, words []string) float64 {
	return c.weight(c.Classes[class], words)
}

// Calculate the weight of the given wordlist for the given class
func (c *Classifier) weight(class Class, words []string) float64 {

	// Handle pathological cases
	if c.Total == 0 {
		return 0
	} else if class.Count == 0 {
		return -500
	}

	//log.Println("Vocabulary:", class.vocabulary)

	// Calculate the prior, log(P(class))
	//prior := math.Log2(float64(class.Count) / float64(c.Total))
	//config.Debug("prior ", prior)
	prior := class.Prior

	// TODO: improve the smoothing - currently wholly unknown titles get
	//       classified as interesting

	// Calculate the log likelihood log(P(words|class))
	ll := prior
	k := float64(c.Words)
	for _, w := range words {
		occurs := class.Vocabulary[w]
		// Calculate the probability of the word appearing in this class
		// but add smoothing to avoid overfitting
		// P(w|class) = N(w,class) + 1
		//              --------------
		//              N(class) + k
		// where:
		//   k = number of words in the training set
		pw := (float64(occurs) + 1) / (float64(class.Count) + k)
		config.Debug("P(", w, "|class) = ", pw)
		ll += math.Log2(pw)
	}

	config.Debug("ll =", ll)
	return ll
}

// Measure the ambiguity of a word
// Taken from:
//  Ambiguity Measure Feature-Selection Algorithm, 
//  Saket S.R. Mengle and Nazli Goharian
//  JOURNAL OF THE AMERICAN SOCIETY FOR INFORMATION SCIENCE AND TECHNOLOGY—May 2009
func (c *Classifier) ambiguityMeasure(word string) (measure float64, total int) {
	
	// Loop through the classes getting the max word frequency and total
	max := 0
	total = 0
	measure = 0
	for _, class := range c.Classes {
		freq := class.Vocabulary[word]
		if freq > max {
			max = freq
		}
		total += freq
	}

	if total > 0 {
		measure = float64(max)/float64(total)
	}

	return measure, total
}


// Get the cutoff value for prefiltering
func (c *Classifier) cutoff() float64 {

	// This cut off should be measured empirically instead of the following fudge
	return 2.0/float64(len(c.Classes) + 1)
}

// Prefilter the given list of words
func (c *Classifier) Prefilter(words []string) []string {

	// This cut off should be measured empirically instead of the following fudge
	cutoff := c.cutoff()

	ret := make([]string, 0, len(words))

	// Measure the ambiguity of each word and only consider
	// unambigious words
	for _, w := range words {
		am, _ := c.ambiguityMeasure(w)
		if am > cutoff {
			ret = append(ret, w)
		}
	}

	return ret
}


// Classify the given word list, returns the id of the class
func (c *Classifier) Classify(words []string) int {

	// Prefilter the words
	filtered := c.Prefilter(words)

	weights := make([]float64, len(c.Classes))

	// Loop through the classes
	for i := range c.Classes {
		config.Debug("Weighting class ", i)
		weights[i] = c.weight(c.Classes[i], filtered)
	}

	// Find the heaviest class
	h := 0
	hw := weights[h]
	for j, w := range weights {
		if w > hw {
			hw = w
			h = j
		}
	}

	return h
}

// Classify the given text, returns the id of the class
func (c *Classifier) ClassifyText(text string) int {
	return c.Classify(Wordlist(text))
}

// Get the words in a class
func (c *Classifier) WordsInClass(class int) map[string]int {

	cutoff := c.cutoff()
	classes := len(c.Classes)
	ret := make(map[string]int)

	// Get unambigious words in each class
	for word, count := range c.Classes[class].Vocabulary {
		am, total := c.ambiguityMeasure(word)
		if am > cutoff && count > total/classes {
			ret[word] = count
		}
	}

	return ret
}

// Serialise to a gob
func (c *Classifier) Serialise() ([]byte, error) {
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	err := enc.Encode(*c)
	if err != nil {
		log.Println("Failed to encode classifier:", err)
		return nil, err
	}

	return b.Bytes(), nil
}

// Create a classifier from a serialised buffer
func Deserialise(b []byte) (*Classifier, error) {
	r := bytes.NewReader(b)
	dec := gob.NewDecoder(r)
	var classifier Classifier
	err := dec.Decode(&classifier)

	if err != nil {
		log.Println("Failed to decode classifier:", err)
		return nil, err
	}

	return &classifier, nil
}
