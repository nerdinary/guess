package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	debug         = flag.Bool("debug", false, "Trace program execution")
	verbose       = flag.Bool("verbose", false, "Print more information")
	printUnlikely = flag.Bool("unlikely", false, "Also show unlikely matches")
	sortGuesses   = flag.Bool("sort", true, "Sort guesses by likeliness")
	timezones     = flag.String("timezones", "UTC,Local", "Timezones that to convert to/from for timestamps and dates")
)

var (
	TZs []*time.Location
)

var Trace *log.Logger

func trace(s string, args ...interface{}) {
	if *debug && Trace != nil {
		Trace.Printf(s, args...)
	}
}

type Guess struct {
	meaning, comment string
	additional       []string
	source           string
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
		a = "\n"
		for _, l := range g.additional {
			a = a + "    " + l + "\n"
		}
	}
	v := ""
	if *verbose {
		v = fmt.Sprintf("[goodness: %d, source: %s]\n", g.goodness, g.source)
	}
	return v + m + c + a
}

type ByGoodness []Guess

func (gs ByGoodness) Len() int           { return len(gs) }
func (gs ByGoodness) Less(i, j int) bool { return gs[i].goodness > gs[j].goodness }
func (gs ByGoodness) Swap(i, j int)      { gs[i], gs[j] = gs[j], gs[i] }

// TODO: It might be interesting to also define a type GuessGroup []Guess, and
// then sort within the group, and sort a []GuessGroup collection by e.g.
// maximum element or sum of guesses.

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
		gs = append(gs, Guess{meaning: fmt.Sprintf("%.1f %s", q, u.symbol), goodness: good, source: "byte-sized unit"})
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
		// truncate sub-second part of duration because it hurts readability
		subms := d.Nanoseconds() % 1000000000
		d -= time.Duration(subms) * time.Nanosecond
		return d, fmt.Sprintf("%s %s", d.String(), suff)
	}

	tries := []struct {
		ts  time.Time
		src string
	}{
		{time.Unix(int64(ts), 0), "timestamp (seconds)"},
		{time.Unix(0, 1000000*int64(ts)), "timestamp (milliseconds)"},
		{time.Unix(0, 1000*int64(ts)), "timestamp (microseconds)"},
		{time.Unix(0, int64(ts)), "timestamp (nanoseconds)"},
	}
	for _, i := range tries {
		t := i.ts
		d, dstr := delta(t)
		pref := ""
		good := 0
		wantcal := false
		wanttzs := false
		switch {
		case d < time.Minute:
			pref = "within the minute, "
			good = 200
			wanttzs = true
		case d < time.Hour:
			pref = "within the hour, "
			good = 180
			wanttzs = true
		case d < 24*time.Hour:
			pref = "within the day, "
			good = 150
			wanttzs = true
		case d < 7*24*time.Hour:
			pref = "within the week, "
			good = 120
			wanttzs = true
			wantcal = true
		case d < 365*24*time.Hour:
			good = 20
			wantcal = true
		default:
			good = -100
		}
		dstr = pref + dstr
		var additional []string
		if wanttzs {
			additional = append(additional, differentTZs(t)...)
		}
		if wantcal {
			additional = append(additional, calendar(t)...)
		}
		g := Guess{
			meaning:    t.String(),
			comment:    dstr,
			additional: additional,
			goodness:   good,
			source:     i.src,
		}
		gs = append(gs, g)
	}
	trace("guessTimestamp: %+v", gs)
	return gs
}

func differentTZs(t time.Time) []string {
	var lines []string
	for _, loc := range TZs {
		lines = append(lines, t.In(loc).String())
	}
	return lines
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
	return gs
}

func usage() {
	fmt.Printf("Usage: %s <string-to-guess>\n", os.Args[0])
}

func main() {
	Trace = log.New(os.Stderr, "TRACE: ", log.LstdFlags)

	flag.Parse()

	if *timezones != "" {
		for _, tz := range strings.Split(*timezones, ",") {
			loc, err := time.LoadLocation(tz)
			if err != nil {
				log.Fatalf("Cannot find time zone: %s", err)
			}
			TZs = append(TZs, loc)
		}
	}

	input := flag.Arg(0)
	if input == "" {
		usage()
		os.Exit(-1)
	}
	trace("Trying to guess %q", input)
	guesses := guess(input)
	if guesses == nil {
		fmt.Print("Could not guess anything.")
		os.Exit(-1)
	}
	if *sortGuesses {
		sort.Sort(ByGoodness(guesses))
	}
	for _, g := range guesses {
		fmt.Println(g.String())
		if !*printUnlikely && g.goodness < 0 {
			break
		}
	}
}

// vim:set noet sw=8 ts=8:
