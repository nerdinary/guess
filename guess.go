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
	doTrace       = flag.Bool("trace", false, "Trace program execution")
	verbose       = flag.Bool("verbose", false, "Print more information")
	printUnlikely = flag.Bool("unlikely", false, "Also show unlikely matches")
	sortGuesses   = flag.Bool("sort", true, "Sort guesses by likeliness")
	timezones     = flag.String("timezones", "America/Los_Angeles,America/New_York,UTC,Asia/Tokyo", "Timezones that to convert to/from for timestamps and dates")
)

var (
	TZs           []*time.Location
	goodTZformats = []string{
		time.RFC3339Nano,
		time.RFC3339,
		time.RFC1123Z,
		time.RFC1123,
		time.RFC850,
		time.RFC822Z,
		time.RFC822,
		time.RubyDate,
		time.UnixDate,
		"2006-01-02 15:04:05.999999999 -0700 MST", // as used by time.Time.String() method
	}
	badTZformats = []string{
		time.ANSIC,
		"Jan _2 2006 15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"01/02/2006 15:04:05",
		"02/01/2006 15:04:05",
	}
)

var Trace *log.Logger

func trace(s string, args ...interface{}) {
	if *doTrace && Trace != nil {
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
	var g []Guesser
	if n, err := strconv.Atoi(s); err == nil {
		g = append(g, Int(n))
	}

	founddate := false
	for _, format := range goodTZformats {
		d, err := time.Parse(format, s)
		if err != nil {
			trace("error parsing as date: %v", err)
			continue
		}
		trace("successfully parsed date %q as %s", s, d)
		g = append(g, Date(d))
		founddate = true
	}
	if !founddate {
		for _, format := range badTZformats {
			t, err := time.ParseInLocation(format, s, time.Local)
			if err != nil {
				trace("error parsing as date: %v", err)
				continue
			}
			trace("%q is parsable from format %q", s, format)
			g = append(g, BadDate{f: format, i: s, t: t})
		}
	}

	return g
}

type Int int

func (n Int) Guess() []Guess {
	var gs []Guess
	gs = append(gs, guessByteSize(int(n))...)
	gs = append(gs, guessTimestamp(int(n))...)
	return gs
}

type Date time.Time

func (d Date) Guess() []Guess {
	g := dateGuess(time.Time(d))
	g.source = "date string with timezone"
	return []Guess{g}
}

type BadDate struct {
	f, i string
	t    time.Time
}

func (d BadDate) Guess() []Guess {
	var lines []string
	for _, loc := range TZs {
		t, err := time.ParseInLocation(d.f, d.i, loc)
		if err != nil {
			trace("previously parsable date is not parsable: %+v", d)
			continue
		}
		zone, _ := t.Zone()
		l := fmt.Sprintf("From %s (%s): %s", zone, loc, t.Local())
		lines = append(lines, l)
	}
	delta := time.Now().Sub(d.t)
	good := 0
	switch {
	case delta < 24*time.Hour:
		good = 200
	case delta < 7*24*time.Hour:
		good = 50
	case delta < 365*24*time.Hour:
		good = 10
	}
	additional := lines
	if delta < 365*24*time.Hour {
		additional = sideBySide(calendar(d.t), additional)
	}

	_, ds := deltaNow(d.t)
	meaning := fmt.Sprintf("In local time: %s (%s)", d.t, ds)
	return []Guess{{
		meaning:    meaning,
		additional: additional,
		goodness:   good,
		source:     "date string without timezone",
	}}
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

func guessTimestamp(ts int) []Guess {
	var gs []Guess

	tries := []struct {
		t   time.Time
		src string
	}{
		{time.Unix(int64(ts), 0), "timestamp (seconds)"},
		{time.Unix(0, 1000000*int64(ts)), "timestamp (milliseconds)"},
		{time.Unix(0, 1000*int64(ts)), "timestamp (microseconds)"},
		{time.Unix(0, int64(ts)), "timestamp (nanoseconds)"},
	}
	for _, i := range tries {
		g := dateGuess(i.t)
		g.source = i.src
		gs = append(gs, g)
	}
	trace("guessTimestamp: %+v", gs)
	return gs
}

func deltaNow(t time.Time) (time.Duration, string) {
	var suff string
	var d time.Duration

	now := time.Now()
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

	interv := []struct {
		d    time.Duration
		desc string
	}{
		{time.Minute, "minute"},
		{time.Hour, "hour"},
		{24 * time.Hour, "day"},
		{7 * 24 * time.Hour, "week"},
	}

	var roughly string
	for _, i := range interv {
		if d < i.d {
			roughly = "within the " + i.desc + ", "
			break
		}
	}

	days, hours, minutes, seconds := 0, int(d.Hours()), int(d.Minutes()), int(d.Seconds())
	minutes -= 60 * hours
	seconds -= 3600*hours + 60*minutes
	for hours >= 24 {
		days += 1
		hours -= 24
	}
	exact := suff
	plural := map[bool]string{true: "s", false: ""}
	if seconds != 0 {
		exact = fmt.Sprintf("%d second%s %s", seconds, plural[seconds > 1], exact)
	}
	if minutes != 0 {
		exact = fmt.Sprintf("%d minute%s %s", minutes, plural[minutes > 1], exact)
	}
	if hours != 0 {
		exact = fmt.Sprintf("%d hour%s %s", hours, plural[hours > 1], exact)
	}
	if days != 0 {
		exact = fmt.Sprintf("%d day%s %s", days, plural[days > 1], exact)
	}
	return d, roughly + exact
}

func dateGuess(t time.Time) Guess {
	d, dstr := deltaNow(t)
	pref := ""
	good := 0
	wantcal := false
	wanttzs := false
	switch {
	case d < time.Minute:
		good = 200
		wanttzs = true
	case d < time.Hour:
		good = 180
		wanttzs = true
	case d < 24*time.Hour:
		good = 150
		wanttzs = true
	case d < 7*24*time.Hour:
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
	var tzs, cal, additional []string
	if wanttzs {
		tzs = differentTZs(t)
	}
	if wantcal {
		cal = calendar(t)
	}
	additional = sideBySide(cal, tzs)
	return Guess{
		meaning:    t.String(),
		comment:    dstr,
		additional: additional,
		goodness:   good,
	}
}

func differentTZs(t time.Time) []string {
	var lines []string
	for _, loc := range TZs {
		lines = append(lines, t.In(loc).String())
	}
	return lines
}

// Function calendar prints an ASCII art calendar for the given timestamp `t`, which looks like this:
//       September 2015
//    Mo Tu We Th Fr Sa Su
//        1  2  3  4  5  6
//     7  8  9 10 11 12 13
//    14 15 16 17 18 19 20
//    21 22 23 24 25 26 27
//    28 29 30
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

func sideBySide(left, right []string) []string {
	maxlen := 0
	for _, l := range left {
		if len(l) > maxlen {
			maxlen = len(l)
		}
	}
	if maxlen == 0 {
		return right
	}
	lines := len(left)
	if len(right) > lines {
		lines = len(right)
	}
	out := make([]string, lines)
	for i, _ := range out {
		if i >= len(right) {
			out[i] = left[i]
			continue
		}
		l := ""
		if i < len(left) {
			l = left[i]
		}
		spaces := 4 + maxlen - len(l)
		out[i] = l + strings.Repeat(" ", spaces) + right[i]
	}
	return out
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
