package check

import (
	"time"

	"github.com/rejisterjack/tock/internal/model"
)

const (
	minPeriodSeconds = 60
	maxPeriodSeconds = 365 * 24 * 60 * 60
	maxGraceSeconds  = 365 * 24 * 60 * 60
	maxNameLen       = 80
)

// ValidationError is rejected operator input.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func invalid(msg string) error { return &ValidationError{Message: msg} }

// Validate checks a name, period, and grace period.
func Validate(name string, periodSeconds, graceSeconds int) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	if periodSeconds < minPeriodSeconds || periodSeconds > maxPeriodSeconds {
		return invalid("period must be between 1 minute and 365 days")
	}
	if graceSeconds < 0 || graceSeconds > maxGraceSeconds {
		return invalid("grace period must be between 0 and 365 days")
	}
	return nil
}

// ValidateName rejects empty names and control characters.
func ValidateName(name string) error {
	if name == "" {
		return invalid("name is required")
	}
	if len(name) > maxNameLen {
		return invalid("name must be 80 characters or fewer")
	}
	for _, r := range name {
		if r < 32 || r == 127 {
			return invalid("name cannot contain control characters")
		}
	}
	return nil
}

// Anchor is the timestamp the schedule is measured from.
func Anchor(c model.Check) time.Time {
	if c.LastPingAt != nil {
		return c.LastPingAt.UTC()
	}
	return c.CreatedAt.UTC()
}

// PeriodEnd is when the expected interval elapses, before grace.
func PeriodEnd(c model.Check) time.Time {
	return Anchor(c).Add(time.Duration(c.PeriodSeconds) * time.Second)
}

// DueAt is when the check becomes overdue: anchor + period + grace.
func DueAt(c model.Check) time.Time {
	return Anchor(c).Add(time.Duration(c.PeriodSeconds+c.GraceSeconds) * time.Second)
}

// IsOverdue reports whether now is past the due time.
func IsOverdue(c model.Check, now time.Time) bool {
	return now.UTC().After(DueAt(c))
}

// DisplayStatus is the operator-facing state. Stored status stays "up" or
// "new" through the grace window; "grace" and "paused" are derived.
func DisplayStatus(c model.Check, now time.Time) string {
	if c.Paused {
		return model.DisplayPaused
	}
	if c.Status == model.StatusDown {
		return model.StatusDown
	}
	now = now.UTC()
	if (c.Status == model.StatusUp || c.Status == model.StatusNew) && now.After(PeriodEnd(c)) && !now.After(DueAt(c)) {
		return model.DisplayGrace
	}
	return c.Status
}

// NextAfterPing is the stored status after a success ping, and whether a
// recovery alert should be sent. Paused checks keep their status.
func NextAfterPing(c model.Check) (status string, recovery bool) {
	if c.Paused {
		return c.Status, false
	}
	if c.Status == model.StatusDown {
		return model.StatusUp, true
	}
	return model.StatusUp, false
}

// NextAfterResume is the stored status after pause is cleared, and whether
// open down alerts should be resolved without notifying. Overdue checks keep
// their status so the scheduler can alert an "up" check that lapsed while paused,
// without re-alerting one that is already down.
func NextAfterResume(c model.Check, now time.Time) (status string, resolve bool) {
	if IsOverdue(c, now) {
		return c.Status, false
	}
	if c.Status == model.StatusDown && c.LastPingAt != nil {
		return model.StatusUp, true
	}
	if c.Status == model.StatusDown && c.LastPingAt == nil {
		return model.StatusNew, true
	}
	return c.Status, false
}

// FormatInterval renders a second count as a short label.
func FormatInterval(seconds int) string {
	if seconds == 0 {
		return "0s"
	}
	switch {
	case seconds%86400 == 0:
		return itoa(seconds/86400) + "d"
	case seconds%3600 == 0:
		return itoa(seconds/3600) + "h"
	case seconds%60 == 0:
		return itoa(seconds/60) + "m"
	default:
		return itoa(seconds) + "s"
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// SplitInterval picks the largest unit that divides seconds evenly, for forms.
func SplitInterval(seconds int) (int, string) {
	if seconds == 0 {
		return 0, "minutes"
	}
	switch {
	case seconds%86400 == 0:
		return seconds / 86400, "days"
	case seconds%3600 == 0:
		return seconds / 3600, "hours"
	case seconds%60 == 0:
		return seconds / 60, "minutes"
	default:
		return seconds, "seconds"
	}
}

// ParseInterval converts a form value and unit into seconds.
func ParseInterval(value int, unit string) (int, error) {
	if value < 0 {
		return 0, invalid("intervals cannot be negative")
	}
	switch unit {
	case "seconds":
		return value, nil
	case "minutes":
		return value * 60, nil
	case "hours":
		return value * 3600, nil
	case "days":
		return value * 86400, nil
	default:
		return 0, invalid("unknown interval unit")
	}
}
