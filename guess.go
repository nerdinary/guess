package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

var Trace *log.Logger

func trace(s string, args ...interface{}) {
	if Trace != nil {
		Trace.Printf(s, args...)
	}
}

type Guess struct {
	meaning, comment string
	additional       string
}

type Guesser interface {
	Guess() []Guess
}

func (g *Guess) String() string {
	m, c, a := g.meaning, "", ""
	if g.comment != "" {
		c = fmt.Sprintf(" (%s)", g.comment)
	}
	if g.additional != "" {
		a = fmt.Sprintf("\nadditional: %s", g.additional)
	}
	return fmt.Sprintf("%s%s%s", m, c, a)
}

func interpret(s string) []Guesser {
	if n, err := strconv.Atoi(s); err == nil {
		return []Guesser{Int(n)}
	}
	return nil
}

type Int int

func (n Int) Guess() []Guess {
	var gs []Guess
	gs = append(gs, guessByteSize(int(n))...)
	return gs
}

func guessByteSize(n int) []Guess {
	var gs []Guess
	units := []struct {
		divisor int
		symbol  string
	}{
		{1024 * 1024 * 1024 * 1024 * 1024 * 1024, "EiB"},
		{1000 * 1000 * 1000 * 1000 * 1000 * 1000, "EB"},
		{1024 * 1024 * 1024 * 1024 * 1024, "PiB"},
		{1000 * 1000 * 1000 * 1000 * 1000, "PB"},
		{1024 * 1024 * 1024 * 1024, "TiB"},
		{1000 * 1000 * 1000 * 1000, "TB"},
		{1024 * 1024 * 1024, "GiB"},
		{1000 * 1000 * 1000, "GB"},
		{1024 * 1024, "MiB"},
		{1000 * 1000, "MB"},
		{1024, "KiB"},
		{1000, "KB"},
	}
	for _, u := range units {
		if 50*n < u.divisor {
			continue
		}
		q := float64(n) / float64(u.divisor)
		gs = append(gs, Guess{meaning: fmt.Sprintf("%.1f %s", q, u.symbol)})
	}
	trace("guessBytesSize: %+v", gs)
	return gs
}

func main() {
	Trace = log.New(os.Stderr, "TRACE: ", log.Ldate|log.Ltime|log.Lshortfile)

	input := "1443237670"
	gs := interpret(input)
	trace("guessers: %#v", gs)
	for _, g := range gs {
		trace("guessers: %#v", gs)
		for _, x := range g.Guess() {
			fmt.Println(x.String())
		}
	}
}
