package sys

import (
	"bufio"
	"strings"
	"syscall"
)

// NetBytes returns total rx/tx bytes over all interfaces except loopback.
func NetBytes() (rx, tx uint64, err error) {
	s, err := readFile("net", "dev")
	if err != nil {
		return 0, 0, err
	}
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		name, rest, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		f := strings.Fields(rest)
		if name == "lo" || len(f) < 9 {
			continue
		}
		rx += atou(f[0])
		tx += atou(f[8])
	}
	return rx, tx, sc.Err()
}

// DiskUsage returns total and used bytes for the filesystem at path.
func DiskUsage(path string) (total, used uint64, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, err
	}
	bs := uint64(st.Bsize)
	total = st.Blocks * bs
	used = total - st.Bfree*bs
	return total, used, nil
}
