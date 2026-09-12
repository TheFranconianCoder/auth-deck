package tui

import (
	"fmt"
	"time"
)

// formatRemaining renders a duration as up to two human-readable units,
// e.g. "1h 2m", "59m", "45s".
func formatRemaining(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)

	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case h > 0:
		return fmt.Sprintf("%dh", h)
	case m > 0 && s > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	case m > 0:
		return fmt.Sprintf("%dm", m)
	default:
		return fmt.Sprintf("%ds", s)
	}
}
