package board

import (
	"testing"
	"time"
)

func TestTodayAndOverdue(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("no time zone data:", err)
	}
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Skip("no time zone data:", err)
	}
	now := time.Date(2026, 11, 3, 0, 30, 0, 0, berlin) // just after midnight
	day := func(d int) time.Time { return time.Date(2026, 11, d, 0, 0, 0, 0, berlin) }
	cases := []struct {
		name                     string
		column                   int
		item                     Task
		overdue, dueToday, today bool
	}{
		{"no due date", ToDo, Task{}, false, false, false},
		{"due yesterday", ToDo, Task{Due: day(2)}, true, false, true},
		{"due today", InProgress, Task{Due: day(3)}, false, true, true},
		{"due tomorrow", ToDo, Task{Due: day(4)}, false, false, false},
		// A due date keeps its calendar day, whatever zone it was saved in.
		{"due today, saved in UTC", ToDo, Task{Due: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)}, false, true, true},
		{"due tomorrow, saved in Tokyo", ToDo, Task{Due: time.Date(2026, 11, 4, 0, 0, 0, 0, tokyo)}, false, false, false},
		{"done and due yesterday", Completed, Task{Due: day(2)}, false, false, false},
		{"completed today", Completed, Task{Completed: day(3).Add(10 * time.Minute)}, false, false, true},
		{"completed yesterday", Completed, Task{Completed: day(3).Add(-time.Minute)}, false, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Overdue(c.column, c.item, now); got != c.overdue {
				t.Errorf("Overdue = %v, want %v", got, c.overdue)
			}
			if got := DueToday(c.column, c.item, now); got != c.dueToday {
				t.Errorf("DueToday = %v, want %v", got, c.dueToday)
			}
			if got := InToday(c.column, c.item, now); got != c.today {
				t.Errorf("InToday = %v, want %v", got, c.today)
			}
		})
	}
}

func TestSorted(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC) }
	tasks := []Task{
		{ID: "a", Priority: "low", Due: day(9), Created: day(1)},
		{ID: "b", Created: day(3)},
		{ID: "c", Priority: "high", Due: day(5), Created: day(2)},
		{ID: "d", Priority: "low", Due: day(5), Created: day(4)},
	}
	cases := map[Sort]string{
		ByManual:   "abcd",
		ByPriority: "cadb", // ties keep their manual order
		ByDue:      "cdab", // no due date last
		ByCreated:  "dbca",
	}
	for by, want := range cases {
		t.Run(by.String(), func(t *testing.T) {
			got := ""
			for _, item := range Sorted(tasks, by) {
				got += item.ID
			}
			if got != want {
				t.Errorf("got %s, want %s", got, want)
			}
		})
	}
	if tasks[0].ID != "a" || tasks[1].ID != "b" {
		t.Error("sorting must leave the manual order alone")
	}
	if ByCreated.Next() != ByManual {
		t.Error("sorting should wrap back to manual order")
	}
}
