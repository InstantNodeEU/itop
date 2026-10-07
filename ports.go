package main

import (
	"net"
	"sort"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/instantnodeeu/itop/internal/sys"
)

type portsPanel struct {
	*list
	app *app
}

func newPortsPanel(a *app) *portsPanel {
	p := &portsPanel{
		list: newList(5, "PROTO", "ADDRESS", "PORT", "PID", "USER", "PROCESS"),
		app:  a,
	}
	p.alignRight(2, 3)
	return p
}

func (p *portsPanel) title() string { return "Ports" }

func (p *portsPanel) hints() string { return "enter show process" }

func (p *portsPanel) update() func() {
	ports, err := sys.ListenPorts()
	if err != nil && len(ports) == 0 {
		msg := err.Error()
		return func() { p.setEmpty(msg) }
	}
	sort.Slice(ports, func(i, j int) bool {
		if ports[i].Port != ports[j].Port {
			return ports[i].Port < ports[j].Port
		}
		return ports[i].Proto < ports[j].Proto
	})
	rows := make([]row, len(ports))
	for i, pt := range ports {
		pid, name := "-", pt.Name
		if pt.PID > 0 {
			pid = strconv.Itoa(pt.PID)
		}
		if name == "" {
			name = "-"
		}
		var color tcell.Color
		if ip := net.ParseIP(pt.Addr); ip != nil && !ip.IsLoopback() {
			color = tcell.ColorYellow // reachable from outside
		}
		rows[i] = row{
			key:   pt.Proto + " " + pt.Addr + " " + strconv.Itoa(pt.Port),
			color: color,
			cells: []string{pt.Proto, pt.Addr, strconv.Itoa(pt.Port), pid, sys.Username(pt.UID), name},
		}
	}
	return func() { p.setRows(rows) }
}

func (p *portsPanel) key(ev *tcell.EventKey) *tcell.EventKey {
	if ev.Key() != tcell.KeyEnter {
		return ev
	}
	row, _ := p.GetSelection()
	pid, err := strconv.Atoi(p.GetCell(row, 3).Text)
	if err != nil {
		p.app.status("owner not visible, try running as root")
		return nil
	}
	p.app.showProcess(pid)
	return nil
}
