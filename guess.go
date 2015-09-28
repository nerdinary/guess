package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
)

var (
	doTrace       = flag.Bool("trace", false, "Trace program execution")
	verbose       = flag.Bool("verbose", false, "Print more information")
	printUnlikely = flag.Bool("unlikely", false, "Also show unlikely matches")
	sortGuesses   = flag.Bool("sort", true, "Sort guesses by likeliness")
	timezones     = flag.String("timezones",
		"America/Los_Angeles,America/New_York,UTC,Europe/Berlin,Asia/Dubai,Asia/Singapore,Australia/Sydney",
		"Timezones that to convert to/from for timestamps and dates")
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
		"2006-01-02 15:04:05 MST",
		"2006-01-02 15:04 MST",
		"2006/01/02 15:04:05.999999999 MST",
		"2006/01/02-15:04:05.999999999 MST",
	}
	badTZformats = []string{
		time.ANSIC,
		"Jan _2 2006 15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05",
		"01/02/2006 15:04:05",
		"02/01/2006 15:04:05",
		"2006/01/02 15:04:05.999999999",
		"2006/01/02-15:04:05.999999999",
		"20060102150405",
	}
)

var byteUnits = []struct {
	mult, altMult int
	sym, altSym   string
	alias         string
}{
	{1024, 1000, "KiB", "KB", "K"},
	{1024 * 1024, 1000 * 1000, "MiB", "MB", "M"},
	{1024 * 1024 * 1024, 1000 * 1000 * 1000, "GiB", "GB", "G"},
	{1024 * 1024 * 1024 * 1024, 1000 * 1000 * 1000 * 1000, "TiB", "TB", "T"},
	{1024 * 1024 * 1024 * 1024 * 1024, 1000 * 1000 * 1000 * 1000 * 1000, "PiB", "PB", "P"},
	{1024 * 1024 * 1024 * 1024 * 1024 * 1024, 1000 * 1000 * 1000 * 1000 * 1000 * 1000, "EiB", "EB", "E"},
}

var Trace *log.Logger

func trace(s string, args ...interface{}) {
	if *doTrace && Trace != nil {
		Trace.Printf(s, args...)
	}
}

type Guess struct {
	guess, comment string
	additional     []string
	source         string
	goodness       int
}

func (g *Guess) String() string {
	t, c, a := g.guess, "", ""
	if g.comment != "" {
		c = fmt.Sprintf(" (%s)", g.comment)
	}
	if g.additional != nil {
		for _, l := range g.additional {
			a = a + "    " + l + "\n"
		}
	}
	v := ""
	if *verbose {
		v = fmt.Sprintf("[goodness: %d, source: %s]\n", g.goodness, g.source)
	}
	highlight := color.New(color.Bold).SprintFunc()
	return v + highlight(t) + c + "\n" + a
}

type ByGoodness []Guess

func (gs ByGoodness) Len() int           { return len(gs) }
func (gs ByGoodness) Less(i, j int) bool { return gs[i].goodness > gs[j].goodness }
func (gs ByGoodness) Swap(i, j int)      { gs[i], gs[j] = gs[j], gs[i] }

// TODO: It might be interesting to also define a type GuessGroup []Guess, and
// then sort within the group, and sort a []GuessGroup collection by e.g.
// maximum element or sum of guesses.

func guess(s string) []Guess {
	var g []Guess
	if n, err := strconv.Atoi(s); err == nil {
		trace("parsed as integer")
		g = append(g, guessByteSize(n)...)
		g = append(g, guessTimestamp(int64(n))...)
	}

	if s == "now" {
		g = append(g, guessTimestamp(time.Now().Unix())...)
	}

	founddate := false
	for _, format := range goodTZformats {
		d, err := time.Parse(format, s)
		if err != nil {
			trace("error parsing as date: %v", err)
			continue
		}
		// Special treatment for formats that specify a timezone
		// identifier but no explicit offset, in which case
		// time.Parse() simply creates an artificial time zone with
		// zero offset; knowing which time zones are interesting, we
		// can do better here, e.g. we successfully parse
		// 2015-09-26 11:29:43 PDT as 2015-09-26 11:29:43 -0700 PDT.
		z, o := d.Zone()
		if o == 0 {
			for _, loc := range TZs {
				cand, _ := time.Now().In(loc).Zone()
				if z != cand {
					continue
				}
				d, err = time.ParseInLocation(format, s, loc)
				if err != nil {
					panic(err)
				}
			}
		}
		trace("successfully parsed date %q as %s", s, d)
		gg := dateGuess(d)
		gg.source = "date string with timezone"
		g = append(g, gg)
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
			g = append(g, guessBadDate(format, s, t)...)
		}
	}

	if ip := net.ParseIP(s); ip != nil {
		trace("successfully parsed as IP address: %v", ip)
		g = append(g, guessIP(ip)...)
	}

	for _, i := range byteUnits {
		mult := 0
		switch {
		case strings.HasSuffix(s, i.sym):
			mult = i.mult
			s = strings.TrimSuffix(s, i.sym)
		case strings.HasSuffix(s, i.alias):
			mult = i.mult
			s = strings.TrimSuffix(s, i.alias)
		case strings.HasSuffix(s, i.altSym):
			mult = i.altMult
			s = strings.TrimSuffix(s, i.altSym)
		}
		if mult == 0 {
			continue
		}
		s = strings.TrimSpace(s)
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			trace("cannot parse %s as float: %v", s, err)
			continue
		}
		g = append(g, guessBytesWithUnit(mult, f)...)
	}

	return g
}

func guessBadDate(f, i string, d time.Time) []Guess {
	var lines []string

	delta, ds := deltaNow(d)
	wantcal := false
	if delta > 2*24*time.Hour && delta < 365*24*time.Hour {
		wantcal = true
	}

	for _, loc := range TZs {
		t, err := time.ParseInLocation(f, i, loc)
		if err != nil {
			trace("previously parsable date is not parsable: %+v", d)
			continue
		}
		zone, _ := t.Zone()
		l := fmt.Sprintf("From %s (%s): %s", zone, loc, t.Local())
		if !wantcal {
			_, s := deltaNow(t)
			l += fmt.Sprintf(" (%s)", s)
		}
		lines = append(lines, l)
	}
	ut, _ := time.ParseInLocation(f, i, time.UTC)
	lines = append(lines, fmt.Sprintf("As UNIX timestamp: %d", ut.Unix()))

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
	if wantcal {
		additional = sideBySide(additional, calendar(d))
	}

	return []Guess{{
		guess:      "In local time: " + d.String(),
		comment:    ds,
		additional: additional,
		goodness:   good,
		source:     "date string without timezone",
	}}
}

type SizeWithUnit struct {
	mult int
	val  float64
}

func guessBytesWithUnit(mult int, val float64) []Guess {
	n := int(val * float64(mult))
	return []Guess{{
		guess:      fmt.Sprintf("%d bytes", n),
		additional: bytesInfo(n),
		source:     "byte count with unit",
	}}
}

func guessByteSize(n int) []Guess {
	return []Guess{{
		guess:      fmt.Sprintf("%d bytes", n),
		additional: bytesInfo(n),
		source:     "byte count without explicit unit",
	}}
}

func bytesInfo(n int) []string {
	var lines []string
	for _, u := range byteUnits {
		p := float64(n) / float64(u.mult)
		q := float64(n) / float64(u.altMult)
		if q < 1 {
			continue
		}
		lines = append(lines, fmt.Sprintf("%.1f %s (%.1f %s)", p, u.sym, q, u.altSym))
	}
	trace("bytesInfo: %+v", lines)
	return lines
}

func guessTimestamp(ts int64) []Guess {
	var gs []Guess

	if ts <= 0 {
		return nil
	}

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
		g.guess = fmt.Sprintf("Timestamp %d is ", ts) + g.guess
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
	if now.Before(t) {
		suff = "ahead"
		d = t.Sub(now)
	} else {
		suff = "ago"
		d = now.Sub(t)
	}
	if d < 1*time.Second {
		return time.Duration(0), "right now"
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
	good := -10
	wantcal := false
	wanttzs := true
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
	case d < 5*365*24*time.Hour:
		good = 0
	}
	var tzs, cal []string
	trace("%v: tz=%v cal=%v", t, wanttzs, wantcal)
	if wanttzs {
		tzs = []string{"In other time zones:"}
		tzs = append(tzs, differentTZs(t)...)
		tzs = append(tzs, fmt.Sprintf("UNIX timestamp: %d", t.Unix()))
	}
	if wantcal {
		cal = calendar(t)
	}
	additional := sideBySide(tzs, cal)
	return Guess{
		guess:      t.String(),
		comment:    dstr,
		additional: additional,
		goodness:   good,
	}
}

func differentTZs(t time.Time) []string {
	var lines []string
	for _, loc := range TZs {
		lines = append(lines, fmt.Sprintf("%s (%s)", t.In(loc).String(), loc.String()))
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

	ctoday := color.New(color.Bold).Add(color.Underline).SprintFunc()
	cgiven := color.New(color.BgRed).Add(color.Bold).SprintFunc()
	csunday := color.New(color.FgMagenta).SprintFunc()

	dom := t.Day()
	now := time.Now()
	today := now.Day()
	currentmonth := t.Year() == now.Year() && t.Month() == now.Month()

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
			day := j.Day()
			switch {
			case day == dom:
				days = append(days, cgiven(fmt.Sprintf("%2d", day)))
			case currentmonth && day == today:
				days = append(days, ctoday(fmt.Sprintf("%2d", day)))
			case j.Weekday() == time.Sunday:
				days = append(days, csunday(fmt.Sprintf("%2d", day)))
			default:
				days = append(days, fmt.Sprintf("%2d", day))
			}
		}
		line := strings.Join(days, " ")
		lines = append(lines, line)
	}
	return lines
}

func guessIP(ip net.IP) []Guess {
	var additional []string
	r, err := net.LookupAddr(ip.String())
	if err != nil {
		additional = append(additional, "(address does not resolve to a host name)")
	} else {
		for _, h := range r {
			additional = append(additional, fmt.Sprintf("reverse lookup: %s", h))
			addrs, err := net.LookupHost(h)
			if err == nil {
				additional = append(additional, fmt.Sprintf("which resolves to: %s", strings.Join(addrs, ", ")))
			} else {
				additional = append(additional, "(which does not forward-resolve to anything)")
			}
		}
	}
	return []Guess{{
		guess:      "IP address " + ip.String(),
		additional: additional,
		source:     "IP address",
		goodness:   200,
	}}
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

	input := strings.TrimSpace(flag.Arg(0))
	if input == "" {
		usage()
		os.Exit(-1)
	}
	trace("Trying to guess %q", input)
	guesses := guess(input)
	if guesses == nil {
		fmt.Println("Could not guess anything.")
		os.Exit(-1)
	}
	if *sortGuesses {
		sort.Sort(ByGoodness(guesses))
	}
	n := 0
	for _, g := range guesses {
		if *printUnlikely || g.goodness >= 0 {
			n++
			fmt.Print(g.String())
		}
	}
	if !*printUnlikely && n == 0 {
		fmt.Println("No good guesses found. How about these unlikely ones?")
		for _, g := range guesses {
			fmt.Print(g.String())
		}
	}
}

// vim:set noet sw=8 ts=8:
