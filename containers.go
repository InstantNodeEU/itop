package main

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/instantnode/itop/internal/docker"
)

type containersPanel struct {
	*list
	app *app
	cli *docker.Client

	mu       sync.Mutex
	stats    map[string]docker.Stats
	inflight map[string]bool
	names    map[string]string
}

func newContainersPanel(a *app) *containersPanel {
	p := &containersPanel{
		list:     newList(6, "NAME", "IMAGE", "STATE", "CPU%", "MEM", "STATUS", "PORTS"),
		app:      a,
		cli:      docker.New(docker.SocketPath()),
		stats:    map[string]docker.Stats{},
		inflight: map[string]bool{},
		names:    map[string]string{},
	}
	p.alignRight(3, 4)
	return p
}

func (p *containersPanel) title() string { return "Docker" }

func (p *containersPanel) hints() string {
	return "s start  x stop  r restart  l/enter logs"
}

func (p *containersPanel) update() func() {
	ctx, cancel := context.WithTimeout(context.Background(), docker.Timeout)
	defer cancel()
	cs, err := p.cli.Containers(ctx)
	if err != nil {
		msg := "docker not available: " + err.Error()
		return func() { p.setEmpty(msg) }
	}
	sort.Slice(cs, func(i, j int) bool {
		if (cs[i].State == "running") != (cs[j].State == "running") {
			return cs[i].State == "running"
		}
		return cs[i].Name() < cs[j].Name()
	})

	p.mu.Lock()
	defer p.mu.Unlock()
	rows := make([]row, len(cs))
	for i, c := range cs {
		p.names[c.ID] = c.Name()
		cpu, mem := "", ""
		if c.State == "running" {
			if st, ok := p.stats[c.ID]; ok {
				cpu = fmt.Sprintf("%.1f", st.CPU)
				mem = human(st.Mem)
			}
			p.fetchStats(c.ID)
		} else {
			delete(p.stats, c.ID)
		}
		rows[i] = row{
			key:   c.ID,
			color: stateColor(c.State),
			cells: []string{c.Name(), c.Image, c.State, cpu, mem, c.Status, c.PortList()},
		}
	}
	if len(rows) == 0 {
		return func() { p.setEmpty("no containers") }
	}
	return func() { p.setRows(rows) }
}

// fetchStats samples in the background since each call blocks for
// about a second on the daemon side. Caller holds p.mu.
func (p *containersPanel) fetchStats(id string) {
	if p.inflight[id] {
		return
	}
	p.inflight[id] = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), docker.Timeout)
		st, err := p.cli.Stats(ctx, id)
		cancel()
		p.mu.Lock()
		if err == nil {
			p.stats[id] = st
		}
		delete(p.inflight, id)
		p.mu.Unlock()
	}()
}

func stateColor(s string) tcell.Color {
	switch s {
	case "running":
		return tcell.ColorLightGreen
	case "restarting", "paused":
		return tcell.ColorYellow
	case "dead":
		return tcell.ColorRed
	}
	return tcell.ColorGray
}

func (p *containersPanel) key(ev *tcell.EventKey) *tcell.EventKey {
	id := p.selected()
	if id == "" {
		return ev
	}
	p.mu.Lock()
	name := p.names[id]
	p.mu.Unlock()

	if ev.Key() == tcell.KeyEnter {
		p.app.logs.followContainer(p.cli, id, name)
		return nil
	}
	switch ev.Rune() {
	case 'l':
		p.app.logs.followContainer(p.cli, id, name)
	case 's':
		p.run(id, name, "start")
	case 'r':
		p.run(id, name, "restart")
	case 'x':
		p.app.confirm("Stop container "+name+"?", []string{"Stop", "Cancel"}, func(b string) {
			if b == "Stop" {
				p.run(id, name, "stop")
			}
		})
	default:
		return ev
	}
	return nil
}

func (p *containersPanel) run(id, name, action string) {
	p.app.status("%s %s...", action, name)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := p.cli.Action(ctx, id, action); err != nil {
			p.app.statusAsync("%s %s: %v", action, name, err)
			return
		}
		p.app.statusAsync("%s %s: done", action, name)
		p.app.kick()
	}()
}
