package due

import (
	"strings"
	"testing"
	"time"
)

// now is a Tuesday afternoon.
var now = time.Date(2026, 11, 3, 15, 4, 5, 0, time.UTC)

func TestParse(t *testing.T) {
	cases := map[string]string{
		"today":      "03-11-2026",
		"TODAY":      "03-11-2026",
		"tomorrow":   "04-11-2026",
		"wed":        "04-11-2026",
		"fri":        "06-11-2026",
		"Friday":     "06-11-2026",
		"mon":        "09-11-2026",
		"tue":        "10-11-2026", // a weekday means its next day after today
		"tuesday":    "10-11-2026",
		"sun":        "08-11-2026",
		"+0d":        "03-11-2026",
		"+3d":        "06-11-2026",
		"+30d":       "03-12-2026",
		"+2w":        "17-11-2026",
		"+3650d":     "31-10-2036", // ten years is the limit
		"+521w":      "28-10-2036",
		"05-11-2026": "05-11-2026",
		"5-1-2027":   "05-01-2027",
		"29-02-2028": "29-02-2028",
	}
	for input, want := range cases {
		t.Run(input, func(t *testing.T) {
			got, err := Parse(input, now)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Format(Layout) != want {
				t.Errorf("got %s, want %s", got.Format(Layout), want)
			}
			if !got.Equal(Day(got)) || got.Location() != now.Location() {
				t.Errorf("got %v, want midnight in now's time zone", got)
			}
		})
	}
}

func TestParseKeepsDaysWholeAcrossClockChanges(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("no time zone data:", err)
	}
	beforeChange := time.Date(2026, 10, 24, 23, 30, 0, 0, berlin) // clocks go back on the 25th

	got, err := Parse("+2d", beforeChange)
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 10, 26, 0, 0, 0, 0, berlin); !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseRejects(t *testing.T) {
	for _, input := range []string{"", "fri.", "frid", "yesterday", "+d", "+3", "+3m", "+-1d", "++3d", "+ 3d", "+3651d", "+522w", "+3650w", "+99999d", "2026-11-05", "11/05/2026", "31-02-2026", "05-13-2026", "5-11-26"} {
		t.Run(input, func(t *testing.T) {
			_, err := Parse(input, now)
			if err == nil || !strings.Contains(err.Error(), "DD-MM-YYYY") {
				t.Errorf("err = %v, want one listing the accepted forms", err)
			}
		})
	}
}

func FuzzParse(f *testing.F) {
	for _, seed := range []string{"today", "fri", "+3d", "+2w", "05-11-2026", "+", "+9999999999999999999d"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		got, err := Parse(input, now)
		if err == nil && !got.Equal(Day(got)) {
			t.Fatalf("%q gave %v, not midnight", input, got)
		}
	})
}
