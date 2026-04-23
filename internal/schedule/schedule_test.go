package schedule

import (
	"testing"
	"time"

	"github.com/benlutz/preflt/internal/checklist"
	"github.com/benlutz/preflt/internal/store"
)

// mkLog creates a completed RunLog with the given completion time.
func mkLog(t time.Time) *store.RunLog {
	return &store.RunLog{Status: "completed", CompletedAt: t}
}

// ── parseCooldown ─────────────────────────────────────────────────────────────

func TestParseCooldown(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
		err  bool
	}{
		{"7d", 7 * 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"12h", 12 * time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"", 0, true},
		{"bad", 0, true},
		{"d", 0, true},
	}
	for _, tt := range tests {
		d, err := parseCooldown(tt.in)
		if tt.err {
			if err == nil {
				t.Errorf("parseCooldown(%q): expected error, got nil", tt.in)
			}
		} else {
			if err != nil {
				t.Errorf("parseCooldown(%q): unexpected error: %v", tt.in, err)
			} else if d != tt.want {
				t.Errorf("parseCooldown(%q) = %v, want %v", tt.in, d, tt.want)
			}
		}
	}
}

// ── isDueAt — daily ───────────────────────────────────────────────────────────

func TestIsDueAt_daily_neverRun(t *testing.T) {
	s := &checklist.Schedule{Frequency: "daily"}
	if !isDueAt(s, nil, time.Now()) {
		t.Error("daily checklist never run should be due")
	}
}

func TestIsDueAt_daily_completedToday(t *testing.T) {
	now := time.Now()
	s := &checklist.Schedule{Frequency: "daily"}
	history := []*store.RunLog{mkLog(now.Add(-1 * time.Hour))}
	if isDueAt(s, history, now) {
		t.Error("daily checklist completed today should not be due")
	}
}

func TestIsDueAt_daily_completedYesterday(t *testing.T) {
	now := time.Now()
	s := &checklist.Schedule{Frequency: "daily"}
	history := []*store.RunLog{mkLog(now.Add(-25 * time.Hour))}
	if !isDueAt(s, history, now) {
		t.Error("daily checklist completed yesterday should be due")
	}
}

func TestCompletedToday_localTimezoneRespected(t *testing.T) {
	// In UTC+12: 23:00 on Jan 16 locally = 11:00 UTC on Jan 16.
	// A run completed at 00:30 UTC+12 on Jan 16 = 12:30 UTC on Jan 15.
	// Old code (Truncate in UTC) would see different UTC days and report "not done today".
	// Correct behaviour: both timestamps are Jan 16 in UTC+12 → done today → not due.
	loc := time.FixedZone("UTC+12", 12*60*60)
	now := time.Date(2024, 1, 16, 23, 0, 0, 0, loc)
	completedAt := time.Date(2024, 1, 16, 0, 30, 0, 0, loc)
	s := &checklist.Schedule{Frequency: "daily"}
	history := []*store.RunLog{mkLog(completedAt)}
	if isDueAt(s, history, now) {
		t.Error("checklist completed today in UTC+12 local time should not be due")
	}
}

// ── isDueAt — weekly ──────────────────────────────────────────────────────────

func TestIsDueAt_weekly_wrongDay(t *testing.T) {
	now := time.Date(2024, 1, 9, 10, 0, 0, 0, time.UTC) // Tuesday
	s := &checklist.Schedule{Frequency: "weekly", On: "monday"}
	if isDueAt(s, nil, now) {
		t.Error("weekly Monday checklist should not be due on Tuesday")
	}
}

func TestIsDueAt_weekly_rightDay_notDone(t *testing.T) {
	now := time.Date(2024, 1, 8, 10, 0, 0, 0, time.UTC) // Monday
	s := &checklist.Schedule{Frequency: "weekly", On: "monday"}
	if !isDueAt(s, nil, now) {
		t.Error("weekly Monday checklist should be due on Monday when not done")
	}
}

func TestIsDueAt_weekly_rightDay_doneToday(t *testing.T) {
	now := time.Date(2024, 1, 8, 10, 0, 0, 0, time.UTC) // Monday
	s := &checklist.Schedule{Frequency: "weekly", On: "monday"}
	history := []*store.RunLog{mkLog(now.Add(-2 * time.Hour))}
	if isDueAt(s, history, now) {
		t.Error("weekly Monday checklist should not be due when already done today")
	}
}

func TestIsDueAt_weekly_multipledays(t *testing.T) {
	now := time.Date(2024, 1, 10, 10, 0, 0, 0, time.UTC) // Wednesday
	s := &checklist.Schedule{Frequency: "weekly", On: "monday,wednesday"}
	if !isDueAt(s, nil, now) {
		t.Error("weekly Mon/Wed checklist should be due on Wednesday")
	}
}

// ── isDueAt — monthly ─────────────────────────────────────────────────────────

func TestIsDueAt_monthly_notDoneThisMonth(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	s := &checklist.Schedule{Frequency: "monthly"}
	history := []*store.RunLog{mkLog(time.Date(2023, 12, 31, 10, 0, 0, 0, time.UTC))}
	if !isDueAt(s, history, now) {
		t.Error("monthly checklist not done this month should be due")
	}
}

func TestIsDueAt_monthly_doneThisMonth(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	s := &checklist.Schedule{Frequency: "monthly"}
	history := []*store.RunLog{mkLog(time.Date(2024, 1, 5, 10, 0, 0, 0, time.UTC))}
	if isDueAt(s, history, now) {
		t.Error("monthly checklist done this month should not be due")
	}
}

// ── isDueAt — cooldown ────────────────────────────────────────────────────────

func TestIsDueAt_cooldown_blocks(t *testing.T) {
	now := time.Now()
	s := &checklist.Schedule{Frequency: "daily", Cooldown: "7d"}
	history := []*store.RunLog{mkLog(now.Add(-3 * 24 * time.Hour))} // 3 days ago
	if isDueAt(s, history, now) {
		t.Error("checklist within 7d cooldown should not be due after 3 days")
	}
}

func TestIsDueAt_cooldown_expired(t *testing.T) {
	now := time.Now()
	s := &checklist.Schedule{Frequency: "daily", Cooldown: "7d"}
	history := []*store.RunLog{mkLog(now.Add(-10 * 24 * time.Hour))} // 10 days ago
	if !isDueAt(s, history, now) {
		t.Error("checklist past 7d cooldown should be due after 10 days")
	}
}

// ── isEntryDueAt — pending ────────────────────────────────────────────────────

func TestIsEntryDueAt_pending_neverRun(t *testing.T) {
	now := time.Now()
	entry := store.ScheduleEntry{Mode: "pending", CreatedAt: now.Add(-1 * time.Hour)}
	if !isEntryDueAt(entry, nil, now) {
		t.Error("pending schedule never completed should be due")
	}
}

func TestIsEntryDueAt_pending_completedAfterCreation(t *testing.T) {
	now := time.Now()
	entry := store.ScheduleEntry{Mode: "pending", CreatedAt: now.Add(-2 * time.Hour)}
	history := []*store.RunLog{mkLog(now.Add(-1 * time.Hour))}
	if isEntryDueAt(entry, history, now) {
		t.Error("pending schedule completed after creation should not be due")
	}
}

func TestIsEntryDueAt_pending_completedBeforeCreation(t *testing.T) {
	now := time.Now()
	// Schedule created 1h ago; run completed 2h ago (before schedule existed).
	entry := store.ScheduleEntry{Mode: "pending", CreatedAt: now.Add(-1 * time.Hour)}
	history := []*store.RunLog{mkLog(now.Add(-2 * time.Hour))}
	if !isEntryDueAt(entry, history, now) {
		t.Error("pending schedule completed before creation should still be due")
	}
}

// ── isEntryDueAt — date ───────────────────────────────────────────────────────

func TestIsEntryDueAt_date_beforeFromDate(t *testing.T) {
	now := time.Date(2024, 1, 10, 10, 0, 0, 0, time.UTC)
	entry := store.ScheduleEntry{Mode: "date", From: "2024-01-15", CreatedAt: now.Add(-24 * time.Hour)}
	if isEntryDueAt(entry, nil, now) {
		t.Error("date schedule before from date should not be due")
	}
}

func TestIsEntryDueAt_date_afterFromDate_notDone(t *testing.T) {
	now := time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC)
	entry := store.ScheduleEntry{Mode: "date", From: "2024-01-15", CreatedAt: now.Add(-10 * 24 * time.Hour)}
	if !isEntryDueAt(entry, nil, now) {
		t.Error("date schedule after from date not done should be due")
	}
}

func TestIsEntryDueAt_date_completedAfterFromDate(t *testing.T) {
	now := time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC)
	entry := store.ScheduleEntry{Mode: "date", From: "2024-01-15", CreatedAt: now.Add(-10 * 24 * time.Hour)}
	history := []*store.RunLog{mkLog(time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC))}
	if isEntryDueAt(entry, history, now) {
		t.Error("date schedule completed after from date should not be due")
	}
}

// ── isEntryDueAt — recurring ──────────────────────────────────────────────────

func TestIsEntryDueAt_recurring_delegates_to_isDueAt(t *testing.T) {
	now := time.Now()
	entry := store.ScheduleEntry{
		Mode:      "recurring",
		Frequency: "daily",
		CreatedAt: now.Add(-24 * time.Hour),
	}
	// Not done today — should be due.
	if !isEntryDueAt(entry, nil, now) {
		t.Error("recurring daily entry not done today should be due")
	}
	// Done today — should not be due.
	history := []*store.RunLog{mkLog(now.Add(-30 * time.Minute))}
	if isEntryDueAt(entry, history, now) {
		t.Error("recurring daily entry done today should not be due")
	}
}

// ── FormatDays ────────────────────────────────────────────────────────────────

func TestFormatDays(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"monday", "Mon"},
		{"monday,wednesday", "Mon, Wed"},
		{"FRIDAY", "Fri"},
		{"saturday,sunday", "Sat, Sun"},
		{"Mon", "Mon"},
		{"", ""},
	}
	for _, tt := range tests {
		got := FormatDays(tt.in)
		if got != tt.want {
			t.Errorf("FormatDays(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
