package format_test

import (
	"testing"
	"time"

	"github.com/benlutz/preflt/internal/format"
)

func TestDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{4*time.Minute + 23*time.Second, "4m 23s"},
		{0, "0s"},
	}
	for _, c := range cases {
		if got := format.Duration(c.d); got != c.want {
			t.Errorf("format.Duration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestTimeAgo_zero(t *testing.T) {
	if got := format.TimeAgo(time.Time{}); got != "never" {
		t.Errorf("expected 'never' for zero time, got %q", got)
	}
}

func TestTimeAgo_justNow(t *testing.T) {
	if got := format.TimeAgo(time.Now().Add(-10 * time.Second)); got != "just now" {
		t.Errorf("expected 'just now', got %q", got)
	}
}

func TestTimeAgo_minutes(t *testing.T) {
	if got := format.TimeAgo(time.Now().Add(-23 * time.Minute)); got != "23m ago" {
		t.Errorf("expected '23m ago', got %q", got)
	}
}

func TestTimeAgo_yesterday(t *testing.T) {
	if got := format.TimeAgo(time.Now().Add(-25 * time.Hour)); got != "yesterday" {
		t.Errorf("expected 'yesterday', got %q", got)
	}
}

func TestTimeAgo_days(t *testing.T) {
	if got := format.TimeAgo(time.Now().Add(-72 * time.Hour)); got != "3d ago" {
		t.Errorf("expected '3d ago', got %q", got)
	}
}
