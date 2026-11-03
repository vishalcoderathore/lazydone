// Package due turns due date input, such as "tomorrow", "fri", "+3d" or
// "05-11-2026", into a date, given the current time.
package due

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Layout is how dates are written: DD-MM-YYYY. Parsing also accepts a
// single-digit day or month, such as 5-1-2027.
const (
	Layout      = "02-01-2006"
	parseLayout = "2-1-2006"
)

// weekdays maps each accepted weekday name to its day.
var weekdays = map[string]time.Weekday{}

func init() {
	for day := time.Sunday; day <= time.Saturday; day++ {
		name := strings.ToLower(day.String())
		weekdays[name] = day
		weekdays[name[:3]] = day
	}
}

// Parse reads a due date and returns midnight at its start, in now's time
// zone. It accepts, ignoring case:
//
//   - today and tomorrow
//   - a weekday, such as fri or friday, meaning its next day after today, so
//     fri on a Friday means a week later
//   - +Nd or +Nw, meaning N days or N weeks from today
//   - a date in DD-MM-YYYY form
func Parse(value string, now time.Time) (time.Time, error) {
	today := Day(now)
	value = strings.ToLower(value)
	switch value {
	case "today":
		return today, nil
	case "tomorrow":
		return today.AddDate(0, 0, 1), nil
	}
	if day, ok := weekdays[value]; ok {
		ahead := (int(day)-int(today.Weekday())+6)%7 + 1 // 1 to 7 days
		return today.AddDate(0, 0, ahead), nil
	}
	if rest, ok := strings.CutPrefix(value, "+"); ok && len(rest) >= 2 {
		digits, unit := rest[:len(rest)-1], rest[len(rest)-1]
		// Digits only: Atoi alone would also take a sign, so "++3d" would pass.
		count, err := strconv.Atoi(digits)
		if err == nil && strings.Trim(digits, "0123456789") == "" && count <= 3650 {
			days := -1
			switch unit {
			case 'd':
				days = count
			case 'w':
				days = 7 * count
			}
			if days >= 0 && days <= 3650 { // ten years at most, in days or weeks
				return today.AddDate(0, 0, days), nil
			}
		}
	}
	if date, err := time.ParseInLocation(parseLayout, value, now.Location()); err == nil {
		return date, nil
	}
	return time.Time{}, fmt.Errorf("due date %q isn't one I know: use today, tomorrow, a weekday such as fri, +3d, +2w or DD-MM-YYYY", value)
}

// Day returns midnight at the start of t's day, in t's time zone.
func Day(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}
