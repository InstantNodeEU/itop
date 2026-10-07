package main

import (
	"context"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/instantnodeeu/itop/internal/systemd"
)

type servicesPanel struct {
	*list
	app     *app
	all     bool
	units   []systemd.Unit
	enabled bool
}

func newServicesPanel(a *app) *servicesPanel {
	p := &servicesPanel{
		list:    newList(4, "UNIT", "LOAD", "ACTIVE", "SUB", "DESCRIPTION"),
		app:     a,
		enabled: systemd.Available(),
	}
	return p
}

func (p *servicesPanel) title() string { return "Services" }

func (p *servicesPanel) hints() string {
	return "s start  x stop  r restart  l/enter logs  a show inactive"
}

func (p *servicesPanel) update() func() {
	if !p.enabled {
		return func() { p.setEmpty("systemd not found on this system") }
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	units, err := systemd.Services(ctx)
	if err != nil {
		msg := "systemctl: " + err.Error()
		return func() { p.setEmpty(msg) }
	}
	return func() {
		p.units = units
		p.show()
	}
}

func (p *servicesPanel) show() {
	var rows, failed []row
	for _, u := range p.units {
		if !p.all && u.Active == "inactive" {
			continue
		}
		var color tcell.Color
		switch u.Active {
		case "failed":
			color = tcell.ColorRed
		case "activating", "deactivating", "reloading":
			color = tcell.ColorYellow
		case "inactive":
			color = tcell.ColorGray
		}
		r := row{
			key:   u.Name,
			color: color,
			cells: []string{u.Name, u.Load, u.Active, u.Sub, u.Description},
		}
		if u.Active == "failed" {
			failed = append(failed, r)
		} else {
			rows = append(rows, r)
		}
	}
	p.setRows(append(failed, rows...))
}

func (p *servicesPanel) key(ev *tcell.EventKey) *tcell.EventKey {
	if ev.Rune() == 'a' {
		p.all = !p.all
		p.show()
		return nil
	}
	unit := p.selected()
	if unit == "" || !p.enabled {
		return ev
	}
	if ev.Key() == tcell.KeyEnter {
		p.app.logs.followUnit(unit)
		return nil
	}
	switch ev.Rune() {
	case 'l':
		p.app.logs.followUnit(unit)
	case 's':
		p.run(unit, "start")
	case 'r':
		p.run(unit, "restart")
	case 'x':
		p.app.confirm("Stop "+unit+"?", []string{"Stop", "Cancel"}, func(b string) {
			if b == "Stop" {
				p.run(unit, "stop")
			}
		})
	default:
		return ev
	}
	return nil
}

func (p *servicesPanel) run(unit, action string) {
	p.app.status("%s %s...", action, unit)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if err := systemd.Action(ctx, unit, action); err != nil {
			p.app.statusAsync("%s %s: %v", action, unit, err)
			return
		}
		p.app.statusAsync("%s %s: done", action, unit)
		p.app.kick()
	}()
}
