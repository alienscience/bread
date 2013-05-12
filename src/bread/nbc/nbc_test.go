package nbc

import (
	"testing"
)

// Classes
const (
	interesting = iota
	uninteresting
	numclasses
)

type Classification struct {
	text  string
	class int
}

// Training set
var training = []Classification{
	{`Researchers trained in C++.`, interesting},
	{`Britney buys shoes.`, uninteresting},
	{`Pop stars are famous.`, uninteresting},
	{`Scaling Ruby.`, interesting}}

// Test set
var testset = []Classification{
	{`Britney buys a ruby.`, interesting},
	{`Pop stars shoes`, uninteresting},
	{`Scaling c++`, interesting},
	{`Famous researchers in ruby shoes.`, uninteresting}}

func TestWordlist(t *testing.T) {
	words := Wordlist("A car drives to the AA")

	if words[0] != "drives" || words[1] != "aa" {
		t.Error(words)
	}
}

func TestClassEasy(t *testing.T) {
	c := New([]float64{0.5, 0.5})

	// Train
	for _, t := range training {
		c.TrainText(t.text, t.class)
	}

	// Test
	for _, tst := range testset {
		class := c.ClassifyText(tst.text)
		if class != tst.class {
			t.Error(tst.text, "CLASS", class, "!=", tst.class)
		}
	}
}

func TestSerialise(t *testing.T) {
	c := New([]float64{0.5, 0.5})

	// Train
	for _, t := range training {
		c.TrainText(t.text, t.class)
	}

	// Serialise into a byte slice
	b, err := c.Serialise()
	if err != nil {
		t.Error(err)
	}

	// Deserialise from a byte slice
	cnew, err := Deserialise(b)
	if err != nil {
		t.Error(err)
	}

	// Compare the two classifiers
	if c.Total != cnew.Total ||
		c.Words != cnew.Words ||
		len(c.Classes) != len(cnew.Classes) ||
		c.Classes[0].Count != cnew.Classes[0].Count ||
		len(c.Classes[0].Vocabulary) != len(cnew.Classes[0].Vocabulary) {
		t.Error(c, " != ", cnew)
	}

	// Confirm the new classifier can be used
	for _, tst := range testset {
		class := c.ClassifyText(tst.text)
		if class != tst.class {
			t.Error(tst.text, "CLASS", class, "!=", tst.class)
		}
	}
}
