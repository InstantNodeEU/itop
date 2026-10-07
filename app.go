package main

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type panel interface {
	tview.Primitive
	title() string
	hints() string
	// update collects data off the UI goroutine and returns a function
	// that applies it on the UI goroutine (or nil).
	update() func()
	key(*tcell.EventKey) *tcell.EventKey
	setFilter(string)
}

type app struct {
	*tview.Application

	header *header
	tabs   *tview.TextView
	body   *tview.Pages
	footer *tview.Pages
	msg    *tview.TextView
	input  *tview.InputField
	root   *tview.Pages
	layout *tview.Flex

	panels  []panel
	procs   *procsPanel
	logs    *logsPanel
	active  atomic.Int32
	filters []string
	modal   bool

	interval time.Duration
	wake     chan struct{}
	msgUntil time.Time
}

func newApp(interval time.Duration) *app {
	a := &app{
		Application: tview.NewApplication(),
		header:      newHeader(),
		tabs:        tview.NewTextView().SetDynamicColors(true).SetWrap(false),
		body:        tview.NewPages(),
		footer:      tview.NewPages(),
		msg:         tview.NewTextView().SetDynamicColors(true).SetWrap(false),
		input:       tview.NewInputField().SetLabel("/"),
		root:        tview.NewPages(),
		interval:    interval,
		wake:        make(chan struct{}, 1),
	}
	a.procs = newProcsPanel(a)
	a.logs = newLogsPanel(a)
	a.panels = []panel{a.procs, newContainersPanel(a), newServicesPanel(a), newPortsPanel(a), a.logs}
	a.filters = make([]string, len(a.panels))
	for i, p := range a.panels {
		a.body.AddPage(p.title(), p, true, i == 0)
	}

	a.input.SetFieldBackgroundColor(tcell.ColorDefault)
	a.input.SetChangedFunc(func(text string) {
		i := a.active.Load()
		a.filters[i] = text
		a.panels[i].setFilter(text)
	})
	a.input.SetDoneFunc(func(k tcell.Key) {
		if k == tcell.KeyEscape {
			a.input.SetText("")
		}
		a.footer.SwitchToPage("msg")
		a.SetFocus(a.current())
		a.drawTabs()
	})
	a.footer.AddPage("msg", a.msg, true, true)
	a.footer.AddPage("input", a.input, true, false)

	a.layout = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.header, 4, 0, false).
		AddItem(a.tabs, 1, 0, false).
		AddItem(a.body, 0, 1, true).
		AddItem(a.footer, 1, 0, false)
	a.root.AddPage("main", a.layout, true, true)

	a.SetRoot(a.root, true)
	a.SetInputCapture(a.globalKeys)
	a.drawTabs()
	return a
}

func (a *app) current() panel { return a.panels[a.active.Load()] }

func (a *app) switchTo(p panel) {
	for i, q := range a.panels {
		if q == p {
			a.selectTab(i)
			return
		}
	}
}

func (a *app) selectTab(i int) {
	if i < 0 || i >= len(a.panels) {
		return
	}
	a.active.Store(int32(i))
	p := a.panels[i]
	a.body.SwitchToPage(p.title())
	a.SetFocus(p)
	if p == a.logs && a.logs.source == "" {
		a.logs.followSystem()
	}
	a.drawTabs()
	a.kick()
}

func (a *app) drawTabs() {
	var b strings.Builder
	cur := int(a.active.Load())
	for i, p := range a.panels {
		if i == cur {
			fmt.Fprintf(&b, "[black:aqua] %d %s [-:-]", i+1, p.title())
		} else {
			fmt.Fprintf(&b, " [aqua]%d[-] %s ", i+1, p.title())
		}
	}
	if f := a.filters[cur]; f != "" {
		fmt.Fprintf(&b, "  [yellow]filter: %s[-]", tview.Escape(f))
	}
	a.tabs.SetText(b.String())
	if time.Now().After(a.msgUntil) {
		a.msg.SetText("[gray]" + tview.Escape(a.current().hints()) + "  ? help  q quit[-]")
	}
}

func (a *app) status(format string, args ...any) {
	a.msg.SetText(tview.Escape(fmt.Sprintf(format, args...)))
	a.msgUntil = time.Now().Add(4 * time.Second)
}

func (a *app) statusAsync(format string, args ...any) {
	a.QueueUpdateDraw(func() { a.status(format, args...) })
}

// kick triggers a refresh right away instead of waiting for the next tick.
func (a *app) kick() {
	select {
	case a.wake <- struct{}{}:
	default:
	}
}

func (a *app) showProcess(pid int) {
	a.filters[0] = ""
	a.procs.setFilter("")
	a.procs.selectKey(fmt.Sprint(pid))
	a.selectTab(0)
}

func (a *app) globalKeys(ev *tcell.EventKey) *tcell.EventKey {
	if a.modal || a.input.HasFocus() {
		return ev
	}
	switch ev.Key() {
	case tcell.KeyCtrlC:
		a.Stop()
		return nil
	case tcell.KeyTab:
		a.selectTab((int(a.active.Load()) + 1) % len(a.panels))
		return nil
	case tcell.KeyBacktab:
		a.selectTab((int(a.active.Load()) + len(a.panels) - 1) % len(a.panels))
		return nil
	case tcell.KeyEscape:
		i := a.active.Load()
		if a.filters[i] != "" {
			a.filters[i] = ""
			a.panels[i].setFilter("")
			a.drawTabs()
		}
		return nil
	case tcell.KeyF10:
		a.Stop()
		return nil
	}
	switch r := ev.Rune(); {
	case r == 'q':
		a.Stop()
		return nil
	case r >= '1' && r <= '9':
		a.selectTab(int(r - '1'))
		return nil
	case r == '?':
		a.help()
		return nil
	case r == '/' && a.current() != a.logs:
		a.input.SetText(a.filters[a.active.Load()])
		a.footer.SwitchToPage("input")
		a.SetFocus(a.input)
		return nil
	}
	return a.current().key(ev)
}

func (a *app) confirm(text string, buttons []string, done func(string)) {
	m := tview.NewModal().SetText(text).AddButtons(buttons).
		SetDoneFunc(func(_ int, label string) {
			a.root.RemovePage("modal")
			a.modal = false
			a.SetFocus(a.current())
			done(label)
		})
	a.modal = true
	a.root.AddPage("modal", m, true, true)
	a.SetFocus(m)
}

const helpText = `Tabs
  1-5, Tab        switch tab
  /               filter current tab, Esc clears it
  q, F10, Ctrl-C  quit

Processes
  c m p n         sort by cpu, memory, pid, name (again to reverse)
  k, F9           send TERM or KILL

Docker and Services
  s x r           start, stop, restart
  l, Enter        follow logs
  a               show inactive services

Ports
  Enter           jump to the owning process

Logs
  s               back to the system log
  g G             top, end (end keeps following)`

func (a *app) help() {
	tv := tview.NewTextView().SetText(helpText)
	tv.SetBorder(true).SetTitle(" itop " + version + " ")
	tv.SetDoneFunc(func(tcell.Key) { a.closeHelp() })
	tv.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Rune() == 'q' || ev.Rune() == '?' {
			a.closeHelp()
			return nil
		}
		return ev
	})
	grid := tview.NewGrid().SetColumns(0, 60, 0).SetRows(0, 24, 0).AddItem(tv, 1, 1, 1, 1, 0, 0, true)
	a.modal = true
	a.root.AddPage("help", grid, true, true)
	a.SetFocus(tv)
}

func (a *app) closeHelp() {
	a.root.RemovePage("help")
	a.modal = false
	a.SetFocus(a.current())
}

func (a *app) loop() {
	t := time.NewTicker(a.interval)
	defer t.Stop()
	for {
		hd := a.header.update()
		apply := a.current().update()
		a.QueueUpdateDraw(func() {
			lines := a.header.render(hd)
			a.layout.ResizeItem(a.header, lines, 0)
			if apply != nil {
				apply()
			}
			a.drawTabs()
		})
		select {
		case <-t.C:
		case <-a.wake:
		}
	}
}
