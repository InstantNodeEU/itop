// Package sys reads system and process information straight from /proc.
package sys

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ProcRoot is where procfs is mounted. Tests point it at a fixture tree.
var ProcRoot = "/proc"

func procPath(parts ...string) string {
	return filepath.Join(append([]string{ProcRoot}, parts...)...)
}

func readFile(parts ...string) (string, error) {
	b, err := os.ReadFile(procPath(parts...))
	return string(b), err
}

func atou(s string) uint64 {
	n, _ := strconv.ParseUint(s, 10, 64)
	return n
}

// Uptime returns system uptime in seconds.
func Uptime() (float64, error) {
	s, err := readFile("uptime")
	if err != nil {
		return 0, err
	}
	f := strings.Fields(s)
	if len(f) == 0 {
		return 0, nil
	}
	return strconv.ParseFloat(f[0], 64)
}

// LoadAvg returns the 1, 5 and 15 minute load averages.
func LoadAvg() ([3]float64, error) {
	var l [3]float64
	s, err := readFile("loadavg")
	if err != nil {
		return l, err
	}
	f := strings.Fields(s)
	for i := 0; i < 3 && i < len(f); i++ {
		l[i], _ = strconv.ParseFloat(f[i], 64)
	}
	return l, nil
}
