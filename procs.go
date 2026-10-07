package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/gdamore/tcell/v2"
	"github.com/instantnode/itop/internal/sys"
)

type procRow struct {
	sys.Proc
	cpu float64
	mem float64
}

type procsPanel struct {
	*list
	app *app

	prevTicks map[int]uint64
	prevTotal uint64

	procs   []procRow
	sortBy  byte
	reverse bool
}

func newProcsPanel(a *app) *procsPanel {
	p := &procsPanel{
		list:   newList(7, "PID", "USER", "S", "CPU%", "MEM%", "RES", "TIME+", "COMMAND"),
		app:    a,
		sortBy: 'c',
	}
	p.alignRight(0, 3, 4, 5, 6)
	return p
}

func (p *procsPanel) title() string { return "Processes" }

func (p *procsPanel) hints() string {
	return "c/m/p/n sort  k kill  / filter"
}

func (p *procsPanel) update() func() {
	all, cores, _ := sys.ReadCPU()
	mem, _ := sys.ReadMem()
	procs, err := sys.Procs()
	if err != nil {
		return func() { p.setEmpty(err.Error()) }
	}

	// percentage of one core, same as top and htop
	elapsed := float64(all.Total-p.prevTotal) / float64(max(len(cores), 1))
	ticks := make(map[int]uint64, len(procs))
	rows := make([]procRow, len(procs))
	for i, pr := range procs {
		rows[i].Proc = pr
		ticks[pr.PID] = pr.Ticks
		if prev, ok := p.prevTicks[pr.PID]; ok && p.prevTotal > 0 && elapsed > 0 && pr.Ticks >= prev {
			rows[i].cpu = float64(pr.Ticks-prev) / elapsed * 100
		}
		if mem.Total > 0 {
			rows[i].mem = float64(pr.RSS) / float64(mem.Total) * 100
		}
	}
	p.prevTicks, p.prevTotal = ticks, all.Total

	return func() {
		p.procs = rows
		p.show()
	}
}

func (p *procsPanel) less(a, b procRow) bool {
	switch p.sortBy {
	case 'm':
		return a.RSS > b.RSS
	case 'p':
		return a.PID < b.PID
	case 'n':
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	}
	if a.cpu == b.cpu {
		return a.PID < b.PID
	}
	return a.cpu > b.cpu
}

func (p *procsPanel) show() {
	rs := p.procs
	sort.SliceStable(rs, func(i, j int) bool {
		if p.reverse {
			return p.less(rs[j], rs[i])
		}
		return p.less(rs[i], rs[j])
	})

	out := make([]row, len(rs))
	for i, r := range rs {
		var color tcell.Color
		switch {
		case strings.HasPrefix(r.Cmd, "["):
			color = tcell.ColorGray
		case r.State == "R":
			color = tcell.ColorLightGreen
		}
		out[i] = row{
			key:   strconv.Itoa(r.PID),
			color: color,
			cells: []string{
				strconv.Itoa(r.PID),
				r.User,
				r.State,
				fmt.Sprintf("%.1f", r.cpu),
				fmt.Sprintf("%.1f", r.mem),
				human(r.RSS),
				cputime(r.Ticks),
				r.Cmd,
			},
		}
	}
	p.setRows(out)
}

func (p *procsPanel) key(ev *tcell.EventKey) *tcell.EventKey {
	switch r := ev.Rune(); r {
	case 'c', 'm', 'p', 'n':
		if p.sortBy == byte(r) {
			p.reverse = !p.reverse
		} else {
			p.sortBy, p.reverse = byte(r), false
		}
		p.show()
		return nil
	case 'k':
		p.kill()
		return nil
	}
	if ev.Key() == tcell.KeyF9 {
		p.kill()
		return nil
	}
	return ev
}

func (p *procsPanel) kill() {
	pid, err := strconv.Atoi(p.selected())
	if err != nil {
		return
	}
	var name string
	for _, r := range p.procs {
		if r.PID == pid {
			name = r.Name
		}
	}
	p.app.confirm(fmt.Sprintf("Send signal to %d (%s)?", pid, name), []string{"TERM", "KILL", "Cancel"}, func(b string) {
		sig := syscall.SIGTERM
		switch b {
		case "KILL":
			sig = syscall.SIGKILL
		case "TERM":
		default:
			return
		}
		if err := syscall.Kill(pid, sig); err != nil {
			p.app.status("kill %d: %v", pid, err)
			return
		}
		p.app.status("sent %s to %d", b, pid)
	})
}
