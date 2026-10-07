package main

import (
	"fmt"
	"strings"
)

func human(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	f := float64(b)
	for _, s := range []string{"K", "M", "G", "T", "P"} {
		f /= unit
		if f < unit {
			if f >= 100 {
				return fmt.Sprintf("%.0f%s", f, s)
			}
			return fmt.Sprintf("%.1f%s", f, s)
		}
	}
	return fmt.Sprintf("%.0fE", f/unit)
}

func uptime(sec float64) string {
	s := int64(sec)
	d, h, m := s/86400, s/3600%24, s/60%60
	if d > 0 {
		return fmt.Sprintf("%dd %02d:%02d", d, h, m)
	}
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s%60)
}

// cputime formats clock ticks like htop's TIME+ column.
func cputime(ticks uint64) string {
	cs := ticks * 100 / clkTck
	m := cs / 6000
	if m >= 600 {
		return fmt.Sprintf("%dh%02d", m/60, m%60)
	}
	return fmt.Sprintf("%d:%02d.%02d", m, cs/100%60, cs%100)
}

// Practically every Linux build uses USER_HZ=100 and we don't want cgo
// just to call sysconf.
const clkTck = 100

// bar draws a usage meter with tview color tags, e.g. [||||||    42.0%].
func bar(pct float64, width int, label string) string {
	if width < 1 {
		width = 1
	}
	pct = min(max(pct, 0), 100)
	if label == "" {
		label = fmt.Sprintf("%.1f%%", pct)
	}
	inner := width - len(label)
	if inner < 0 {
		inner = 0
	}
	n := int(pct / 100 * float64(width))
	color := "green"
	switch {
	case pct >= 90:
		color = "red"
	case pct >= 60:
		color = "yellow"
	}
	fill := min(n, inner)
	return "[" + "[" + color + "]" + strings.Repeat("|", fill) + "[-]" +
		strings.Repeat(" ", inner-fill) + "[gray]" + label + "[-]]"
}
