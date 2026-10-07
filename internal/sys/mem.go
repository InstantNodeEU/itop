package sys

import (
	"bufio"
	"strings"
)

// Mem values are in bytes.
type Mem struct {
	Total, Available, SwapTotal, SwapFree uint64
}

func (m Mem) Used() uint64     { return m.Total - m.Available }
func (m Mem) SwapUsed() uint64 { return m.SwapTotal - m.SwapFree }

func ReadMem() (Mem, error) {
	var m Mem
	s, err := readFile("meminfo")
	if err != nil {
		return m, err
	}
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 2 {
			continue
		}
		v := atou(f[1]) * 1024
		switch f[0] {
		case "MemTotal:":
			m.Total = v
		case "MemAvailable:":
			m.Available = v
		case "SwapTotal:":
			m.SwapTotal = v
		case "SwapFree:":
			m.SwapFree = v
		}
	}
	return m, sc.Err()
}
