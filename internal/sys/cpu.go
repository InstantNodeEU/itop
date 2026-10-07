package sys

import (
	"bufio"
	"strings"
)

// CPUTimes holds cumulative jiffies for one CPU line in /proc/stat.
type CPUTimes struct {
	Busy  uint64
	Total uint64
}

// Usage returns the busy percentage between two samples.
func (c CPUTimes) Usage(prev CPUTimes) float64 {
	if c.Total <= prev.Total {
		return 0
	}
	dt := c.Total - prev.Total
	db := c.Busy - prev.Busy
	if c.Busy < prev.Busy {
		db = 0
	}
	return float64(db) / float64(dt) * 100
}

// ReadCPU returns the aggregate line and one entry per core.
func ReadCPU() (all CPUTimes, cores []CPUTimes, err error) {
	s, err := readFile("stat")
	if err != nil {
		return all, nil, err
	}
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 5 || !strings.HasPrefix(f[0], "cpu") {
			continue
		}
		var t CPUTimes
		// user nice system idle iowait irq softirq steal; guest is already in user
		for i := 1; i < len(f) && i <= 8; i++ {
			t.Total += atou(f[i])
		}
		idle := atou(f[4])
		if len(f) > 5 {
			idle += atou(f[5])
		}
		t.Busy = t.Total - idle
		if f[0] == "cpu" {
			all = t
		} else {
			cores = append(cores, t)
		}
	}
	return all, cores, sc.Err()
}
