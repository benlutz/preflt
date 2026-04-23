package checklist

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// validItemID matches legal item ID strings: letters, digits, hyphens, underscores.
var validItemID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Validate checks that the checklist is strictly well-formed. All violations are
// collected and returned together so callers see every problem at once.
func (c *Checklist) Validate() error {
	var errs []error

	// ── Checklist-level ──────────────────────────────────────────────────────

	if c.Name == "" {
		errs = append(errs, fmt.Errorf("checklist has no name"))
	} else if len(c.Name) > 100 {
		errs = append(errs, fmt.Errorf("checklist %q: name exceeds 100 characters", c.Name))
	}

	if c.Type != "" && c.Type != TypeNormal && c.Type != TypeEmergency {
		errs = append(errs, fmt.Errorf("checklist %q: unknown type %q (must be \"normal\" or \"emergency\")", c.Name, c.Type))
	}

	if len(c.Phases) > 0 && len(c.Items) > 0 {
		errs = append(errs, fmt.Errorf("checklist %q: cannot mix top-level items[] and phases[]", c.Name))
	}

	for i, step := range c.OnComplete {
		if err := validateStep(step, fmt.Sprintf("checklist %q on_complete[%d]", c.Name, i)); err != nil {
			errs = append(errs, err)
		}
	}

	// ── Phase-level ──────────────────────────────────────────────────────────

	for pi, ph := range c.Phases {
		if ph.Name == "" {
			errs = append(errs, fmt.Errorf("checklist %q: phases[%d] has no name", c.Name, pi))
		}
		if len(ph.Items) == 0 {
			errs = append(errs, fmt.Errorf("checklist %q: phase %q has no items", c.Name, ph.Name))
		}
	}

	// ── Item-level ───────────────────────────────────────────────────────────

	items := c.Flatten()
	if len(items) == 0 {
		errs = append(errs, fmt.Errorf("checklist %q has no items", c.Name))
	}

	seen := make(map[string]bool, len(items))
	for _, fi := range items {
		item := fi.Item
		loc := fmt.Sprintf("checklist %q item %q", c.Name, item.ID)

		if item.ID == "" {
			errs = append(errs, fmt.Errorf("checklist %q: item at index %d has no ID", c.Name, fi.GlobalIdx))
		} else {
			if !validItemID.MatchString(item.ID) {
				errs = append(errs, fmt.Errorf("%s: ID must contain only letters, digits, hyphens, or underscores", loc))
			}
			if len(item.ID) > 64 {
				errs = append(errs, fmt.Errorf("%s: ID exceeds 64 characters", loc))
			}
		}

		if seen[item.ID] {
			errs = append(errs, fmt.Errorf("duplicate item ID %q in checklist %q", item.ID, c.Name))
		}
		seen[item.ID] = true

		if item.Label == "" {
			errs = append(errs, fmt.Errorf("%s: has no label", loc))
		} else if len(item.Label) > 200 {
			errs = append(errs, fmt.Errorf("%s: label exceeds 200 characters", loc))
		}

		// Empty type is allowed — applyDefaults will have filled it in via Load.
		// An explicit non-empty unknown value is always an error.
		if item.Type != "" && item.Type != ItemDo && item.Type != ItemCheck {
			errs = append(errs, fmt.Errorf("%s: unknown type %q (must be \"do\" or \"check\")", loc, item.Type))
		}

		if c.Type == TypeEmergency && item.NAAllowed {
			errs = append(errs, fmt.Errorf("%s: na_allowed cannot be true on an emergency checklist", loc))
		}

		for i, step := range item.OnComplete {
			if err := validateStep(step, fmt.Sprintf("%s on_complete[%d]", loc, i)); err != nil {
				errs = append(errs, err)
			}
		}

		if item.Condition != nil {
			if item.Condition.IfYes == nil && item.Condition.IfNo == nil {
				errs = append(errs, fmt.Errorf("%s: condition has neither if_yes nor if_no", loc))
			}
			if item.Condition.IfYes != nil {
				if err := validateBranch(*item.Condition.IfYes, loc+".condition.if_yes"); err != nil {
					errs = append(errs, err)
				}
			}
			if item.Condition.IfNo != nil {
				if err := validateBranch(*item.Condition.IfNo, loc+".condition.if_no"); err != nil {
					errs = append(errs, err)
				}
			}
		}
	}

	// ── Schedule-level ───────────────────────────────────────────────────────

	if c.Schedule != nil {
		errs = append(errs, validateSchedule(c.Schedule, c.Name)...)
	}

	return errors.Join(errs...)
}

func validateStep(step AutomationStep, loc string) error {
	if step.Shell == "" && step.Webhook == "" {
		return fmt.Errorf("%s: automation step has neither shell nor webhook", loc)
	}
	if step.Webhook != "" &&
		!strings.HasPrefix(step.Webhook, "http://") &&
		!strings.HasPrefix(step.Webhook, "https://") {
		return fmt.Errorf("%s: webhook %q must start with http:// or https://", loc, step.Webhook)
	}
	return nil
}

func validateBranch(b ConditionBranch, loc string) error {
	if !b.Skip && b.TriggerChecklist == "" && !b.Abort {
		return fmt.Errorf("%s: branch has no action (set at least one of: skip, trigger_checklist, abort)", loc)
	}
	return nil
}

var (
	validFrequency = map[string]bool{"daily": true, "weekly": true, "monthly": true}
	validWeekday   = map[string]bool{
		"monday": true, "tuesday": true, "wednesday": true,
		"thursday": true, "friday": true, "saturday": true, "sunday": true,
	}
	validPeriod = map[string]bool{"morning": true, "afternoon": true, "evening": true}
)

func validateSchedule(s *Schedule, name string) []error {
	var errs []error
	loc := fmt.Sprintf("checklist %q schedule", name)

	if s.Frequency == "" {
		errs = append(errs, fmt.Errorf("%s: frequency is required (daily, weekly, or monthly)", loc))
	} else if !validFrequency[s.Frequency] {
		errs = append(errs, fmt.Errorf("%s: unknown frequency %q (must be daily, weekly, or monthly)", loc, s.Frequency))
	}

	if s.Frequency == "weekly" {
		if s.On == "" {
			errs = append(errs, fmt.Errorf("%s: weekly schedule requires an \"on\" field (e.g. \"monday\" or \"monday,wednesday\")", loc))
		} else {
			for _, day := range strings.Split(s.On, ",") {
				d := strings.TrimSpace(strings.ToLower(day))
				if d != "" && !validWeekday[d] {
					errs = append(errs, fmt.Errorf("%s: unknown weekday %q in \"on\" field", loc, strings.TrimSpace(day)))
				}
			}
		}
	}

	if s.Period != "" && !validPeriod[s.Period] {
		errs = append(errs, fmt.Errorf("%s: unknown period %q (must be morning, afternoon, or evening)", loc, s.Period))
	}

	if s.Cooldown != "" {
		if err := parseCooldown(s.Cooldown); err != nil {
			errs = append(errs, fmt.Errorf("%s: invalid cooldown %q (%w)", loc, s.Cooldown, err))
		}
	}

	return errs
}

// parseCooldown validates strings like "7d" (days) or standard Go durations ("2h30m").
func parseCooldown(s string) error {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		_, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return fmt.Errorf("expected a number before \"d\" (e.g. \"7d\")")
		}
		return nil
	}
	_, err := time.ParseDuration(s)
	return err
}
