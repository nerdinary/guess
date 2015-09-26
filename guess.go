package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
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
	additional       []string
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
	if g.additional != nil {
		var s string
		for _, l := range g.additional {
			s = fmt.Sprintf("%s  %s\n", s, l)
		}
		a = fmt.Sprintf("\nadditional\n%s", s)
	}
	return fmt.Sprintf("%s%s%s   [goodness: %d]", m, c, a, g.goodness)
}

type ByGoodness []Guess

func (gs ByGoodness) Len() int           { return len(gs) }
func (gs ByGoodness) Less(i, j int) bool { return gs[i].goodness > gs[j].goodness }
func (gs ByGoodness) Swap(i, j int)      { gs[i], gs[j] = gs[j], gs[i] }

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
		q := float64(n) / float64(u.divisor)
		good := 0
		switch {
		case q < 0.01:
			good = -50
		case q > 1000:
			good = 10
		case q > 1:
			good = 50
		default:
			good = -10
		}
		gs = append(gs, Guess{meaning: fmt.Sprintf("%.1f %s", q, u.symbol), goodness: good})
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
		wantcal := false
		switch {
		case d < time.Minute:
			good = 200
		case d < time.Hour:
			good = 180
		case d < 24*time.Hour:
			good = 150
		case d < 7*24*time.Hour:
			good = 120
			wantcal = true
		case d < 365*24*time.Hour:
			good = 20
			wantcal = true
		default:
			good = -100
		}
		var cal []string
		if wantcal {
			cal = calendar(t)
		}
		gs = append(gs, Guess{meaning: t.String(), comment: dstr, additional: cal, goodness: good})
	}
	trace("guessTimestamp: %+v", gs)
	return gs
}

func calendar(t time.Time) []string {
	lines := []string{
		fmt.Sprintf("%s%s %d", strings.Repeat(" ", (20-(len(t.Month().String())+1+4))/2), t.Month(), t.Year()),
		"Mo Tu We Th Fr Sa Su",
	}
	dom := t.Day()

	// First day of the given month
	i := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	// Rewind to previous month’s monday
	for ; i.Weekday() != time.Monday; i = i.AddDate(0, 0, -1) {
	}
	done := false
	for ; !done; i = i.AddDate(0, 0, 7) {
		var days []string
		for j := i; ; j = j.AddDate(0, 0, 1) {
			if j.Day() == 1 && j.Month() != t.Month() {
				done = true
				break // We are in the next month already
			}
			if j.Month() != t.Month() {
				// We are in the previous month, pad with spaces
				days = append(days, "  ")
				continue
			}
			if j.Day() != i.Day() && j.Weekday() == time.Monday {
				break // We have reached the end of the week
			}
			trace("inner loop j = %v, WD %v", j, j.Weekday())
			day := j.Day()
			if day == dom {
				days = append(days, fmt.Sprintf("%2d", day))
			} else {
				days = append(days, fmt.Sprintf("%2d", day))
			}
		}
		line := strings.Join(days, " ")
		lines = append(lines, line)
	}
	return lines
}

func guess(s string) []Guess {
	var gs []Guess
	guessers := interpret(s)
	trace("guessers: %#v", guessers)
	for _, g := range guessers {
		gs = append(gs, g.Guess()...)
	}
	sort.Sort(ByGoodness(gs))
	return gs
}

func usage() {
	fmt.Println("Usage: %s <string-to-guess>", os.Args[0])
}

func main() {
	Trace = log.New(os.Stderr, "TRACE: ")

	if len(os.Args) < 2 {
		usage()
		os.Exit(-1)
	}

	input := os.Args[1]
	trace("Trying to guess %q", input)
	guesses := guess(input)
	if guesses == nil {
		fmt.Print("Could not guess anything.")
		os.Exit(-1)
	}
	for _, g := range guesses {
		fmt.Println(g.String())
	}
}

// vim:set noet sw=8 ts=8:
