package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
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
	goodness         int
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
	gs = append(gs, guessTimestamp(int(n))...)
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

// TODO: Try several timezones
func guessTimestamp(ts int) []Guess {
	var gs []Guess
	now := time.Now()
	delta := func(t time.Time) (time.Duration, string) {
		var suff string
		var d time.Duration
		if now.Equal(t) {
			return time.Duration(0), "right now"
		}
		if now.Before(t) {
			suff = "ahead"
			d = t.Sub(now)
		} else {
			suff = "ago"
			d = now.Sub(t)
		}
		return d, fmt.Sprintf("%s %s", d.String(), suff)
	}

	tries := []time.Time{time.Unix(int64(ts), 0), time.Unix(0, int64(ts))}
	for _, t := range tries {
		d, dstr := delta(t)
		good := 0
		wantcal := true
		switch {
		case d < time.Minute:
			good = 200
		case d < time.Hour:
			good = 180
		case d < 24*time.Hour:
			good = 150
		case d < 7*24*time.Hour:
			good = 120
		case d < 365*24*time.Hour:
			good = 20
		default:
			good = -100
			wantcal = false
		}
		var cal string
		if wantcal {
			cal = calendar(t)
		}
		gs = append(gs, Guess{meaning: t.String(), comment: dstr, additional: cal, goodness: good})
	}
	trace("guessTimestamp: %+v", gs)
	return gs
}

func calendar(t time.Time) string {
	// Print ASCII art calendar
	return ""
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
