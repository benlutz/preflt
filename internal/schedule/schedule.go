package schedule

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/benlutz/preflt/internal/checklist"
	"github.com/benlutz/preflt/internal/store"
)

// DueItem pairs a checklist with its path, reason, and optional period hint.
type DueItem struct {
	Checklist *checklist.Checklist
	Path      string
	Reason    string
	Period    string // morning | afternoon | evening, or ""
}

// Due returns all checklists that are due right now. It checks two sources:
//  1. schedules.json entries (user-managed, take precedence)
//  2. YAML schedule blocks (self-contained fallback, for checklists not in schedules.json)
func Due(paths []string) []DueItem {
	entries, _ := store.LoadSchedules()

	entryByName := make(map[string]store.ScheduleEntry, len(entries))
	for _, e := range entries {
		entryByName[e.Name] = e
	}

	var due []DueItem
	seen := make(map[string]bool)

	// (1) schedules.json entries — load their checklist by path or name.
	for _, entry := range entries {
		nameOrPath := entry.Path
		if nameOrPath == "" {
			nameOrPath = entry.Name
		}
		cl, err := checklist.Load(nameOrPath)
		if err != nil {
			continue
		}
		history, _ := store.LoadHistory(cl.Name, 10)
		if isEntryDue(entry, history) {
			due = append(due, DueItem{
				Checklist: cl,
				Path:      nameOrPath,
				Reason:    entryReason(entry),
				Period:    entry.Period,
			})
			seen[cl.Name] = true
		}
	}

	// (2) YAML schedule blocks for checklists not already handled above.
	for _, path := range paths {
		cl, err := checklist.Load(path)
		if err != nil || cl.Schedule == nil {
			continue
		}
		if seen[cl.Name] {
			continue
		}
		history, _ := store.LoadHistory(cl.Name, 10)
		if isDue(cl.Schedule, history) {
			due = append(due, DueItem{
				Checklist: cl,
				Path:      path,
				Reason:    reason(cl.Schedule),
				Period:    cl.Schedule.Period,
			})
		}
	}

	return due
}

// isEntryDue checks whether a schedules.json entry is currently due.
func isEntryDue(entry store.ScheduleEntry, history []*store.RunLog) bool {
	return isEntryDueAt(entry, history, time.Now())
}

func isEntryDueAt(entry store.ScheduleEntry, history []*store.RunLog, now time.Time) bool {
	switch entry.Mode {
	case "pending":
		// Show until completed at least once after the schedule was created.
		for _, log := range history {
			if log.Status == "completed" && !log.CompletedAt.Before(entry.CreatedAt) {
				return false
			}
		}
		return true

	case "date":
		// Show on/after the from date, until completed once on/after that date.
		if entry.From != "" {
			from, err := time.Parse("2006-01-02", entry.From)
			if err != nil || now.Before(from) {
				return false
			}
			for _, log := range history {
				if log.Status == "completed" && !log.CompletedAt.Before(from) {
					return false
				}
			}
		}
		return true

	case "recurring":
		sched := &checklist.Schedule{
			Frequency: entry.Frequency,
			On:        entry.On,
			Cooldown:  entry.Cooldown,
		}
		return isDueAt(sched, history, now)
	}

	return false
}

// isDue returns true if the YAML-based schedule says the checklist should run now.
func isDue(s *checklist.Schedule, history []*store.RunLog) bool {
	return isDueAt(s, history, time.Now())
}

func isDueAt(s *checklist.Schedule, history []*store.RunLog, now time.Time) bool {
	if s.Cooldown != "" {
		d, err := parseCooldown(s.Cooldown)
		if err == nil && len(history) > 0 {
			if now.Sub(history[0].CompletedAt) < d {
				return false
			}
		}
	}

	switch s.Frequency {
	case "daily":
		return !completedToday(history, now)

	case "weekly":
		if !containsWeekday(parseWeekdays(s.On), now.Weekday()) {
			return false
		}
		return !completedToday(history, now)

	case "monthly":
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		for _, log := range history {
			if !log.CompletedAt.Before(monthStart) {
				return false
			}
		}
		return true
	}

	return false
}

func completedToday(history []*store.RunLog, now time.Time) bool {
	today := now.Truncate(24 * time.Hour)
	for _, log := range history {
		if log.CompletedAt.Truncate(24 * time.Hour).Equal(today) {
			return true
		}
	}
	return false
}

func entryReason(entry store.ScheduleEntry) string {
	switch entry.Mode {
	case "pending":
		return "pending"
	case "date":
		if entry.From != "" {
			return "due since " + entry.From
		}
		return "pending"
	case "recurring":
		return reason(&checklist.Schedule{Frequency: entry.Frequency, On: entry.On})
	}
	return ""
}

func reason(s *checklist.Schedule) string {
	switch s.Frequency {
	case "daily":
		return "daily — not done today"
	case "weekly":
		return fmt.Sprintf("weekly (%s) — not done yet", FormatDays(s.On))
	case "monthly":
		return "monthly — not done this month"
	}
	return ""
}

// parseCooldown parses strings like "7d", "2h", or standard Go durations.
func parseCooldown(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, fmt.Errorf("invalid cooldown %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// parseWeekdays splits a comma-separated weekday string and returns each as a
// time.Weekday. Accepts "monday", "monday,wednesday", etc.
func parseWeekdays(s string) []time.Weekday {
	parts := strings.Split(s, ",")
	days := make([]time.Weekday, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		days = append(days, parseWeekday(p))
	}
	return days
}

func parseWeekday(s string) time.Weekday {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "monday":
		return time.Monday
	case "tuesday":
		return time.Tuesday
	case "wednesday":
		return time.Wednesday
	case "thursday":
		return time.Thursday
	case "friday":
		return time.Friday
	case "saturday":
		return time.Saturday
	default:
		return time.Sunday
	}
}

func containsWeekday(days []time.Weekday, d time.Weekday) bool {
	for _, day := range days {
		if day == d {
			return true
		}
	}
	return false
}

// FormatDays formats a comma-separated weekday string for display.
// "monday,wednesday" → "Mon, Wed"
func FormatDays(s string) string {
	parts := strings.Split(s, ",")
	names := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if len(p) > 3 {
			p = titleCase(p[:3])
		} else {
			p = titleCase(p)
		}
		names = append(names, p)
	}
	return strings.Join(names, ", ")
}

// titleCase returns s with the first letter uppercased and the rest lowercased.
// For use with ASCII weekday abbreviations only.
func titleCase(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

// Greeting returns a time-appropriate greeting with the user's name.
func Greeting(name string) string {
	hour := time.Now().Hour()
	var salutation string
	switch {
	case hour < 12:
		salutation = "Good morning"
	case hour < 17:
		salutation = "Good afternoon"
	default:
		salutation = "Good evening"
	}
	if name != "" {
		return fmt.Sprintf("%s, %s.", salutation, name)
	}
	return salutation + "."
}
