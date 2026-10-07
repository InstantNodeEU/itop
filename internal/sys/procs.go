package sys

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

type Proc struct {
	PID   int
	PPID  int
	State string
	Name  string
	Cmd   string
	User  string
	Ticks uint64 // utime + stime
	RSS   uint64 // bytes
	Nice  int
}

var pageSize = uint64(os.Getpagesize())

// Procs lists all processes visible to the current user.
func Procs() ([]Proc, error) {
	d, err := os.Open(procPath())
	if err != nil {
		return nil, err
	}
	names, err := d.Readdirnames(-1)
	d.Close()
	if err != nil {
		return nil, err
	}
	out := make([]Proc, 0, len(names))
	for _, n := range names {
		pid, err := strconv.Atoi(n)
		if err != nil {
			continue
		}
		p, err := readProc(pid)
		if err != nil {
			continue // process exited while we were looking
		}
		out = append(out, p)
	}
	return out, nil
}

func readProc(pid int) (Proc, error) {
	id := strconv.Itoa(pid)
	stat, err := readFile(id, "stat")
	if err != nil {
		return Proc{}, err
	}
	p, err := parseStat(stat)
	if err != nil {
		return p, err
	}
	if b, err := os.ReadFile(procPath(id, "cmdline")); err == nil {
		p.Cmd = strings.TrimSpace(strings.ReplaceAll(string(b), "\x00", " "))
	}
	if p.Cmd == "" {
		p.Cmd = "[" + p.Name + "]"
	}
	if fi, err := os.Stat(procPath(id)); err == nil {
		if st, ok := fi.Sys().(*syscall.Stat_t); ok {
			p.User = Username(st.Uid)
		}
	}
	return p, nil
}

// parseStat parses /proc/<pid>/stat. The comm field can contain spaces
// and parens, so split on the last ')'.
func parseStat(s string) (Proc, error) {
	var p Proc
	open := strings.IndexByte(s, '(')
	end := strings.LastIndexByte(s, ')')
	if open < 0 || end < open {
		return p, errBadStat
	}
	p.PID, _ = strconv.Atoi(strings.TrimSpace(s[:open]))
	p.Name = s[open+1 : end]
	f := strings.Fields(s[end+1:])
	// f[0] is field 3 (state) in proc(5) numbering
	if len(f) < 22 {
		return p, errBadStat
	}
	p.State = f[0]
	p.PPID, _ = strconv.Atoi(f[1])
	p.Ticks = atou(f[11]) + atou(f[12])
	p.Nice, _ = strconv.Atoi(f[16])
	p.RSS = atou(f[21]) * pageSize
	return p, nil
}

type statError string

func (e statError) Error() string { return string(e) }

const errBadStat = statError("malformed stat line")

var (
	usersMu sync.Mutex
	users   map[uint32]string
)

// PasswdPath is read once to map uids to names.
var PasswdPath = "/etc/passwd"

func Username(uid uint32) string {
	usersMu.Lock()
	defer usersMu.Unlock()
	if users == nil {
		users = loadPasswd(PasswdPath)
	}
	if u, ok := users[uid]; ok {
		return u
	}
	return strconv.Itoa(int(uid))
}

func loadPasswd(path string) map[uint32]string {
	m := map[uint32]string{}
	f, err := os.Open(path)
	if err != nil {
		return m
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Split(sc.Text(), ":")
		if len(parts) < 3 {
			continue
		}
		if uid, err := strconv.ParseUint(parts[2], 10, 32); err == nil {
			m[uint32(uid)] = parts[0]
		}
	}
	return m
}
